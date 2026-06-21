package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	commonlogs "github.com/Martindeeepdark/go-common/logs"

	"seckill-processor-service/internal/domain/model"
	"seckill-processor-service/internal/domain/service"
	"seckill-processor-service/internal/domain/status"

	commonerrors "seckill-common/errors"
)

// ErrInvalidState 状态无效错误
var ErrInvalidState = errors.New("invalid state")

// PaymentOrderGateway 支付超时处理所需的订单网关接口
type PaymentOrderGateway interface {
	service.OrderCreator
	GetOrder(ctx context.Context, orderNo string) (model.OrderInfo, error)
	MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error
	CloseOrder(ctx context.Context, orderNo string) error
}

// PaymentStockGateway 支付超时处理所需的库存网关接口
type PaymentStockGateway = service.StockGateway

// PaymentGateway 支付网关接口
type PaymentGateway interface {
	QueryPayment(ctx context.Context, orderNo string) (model.PayQueryResult, error)
	ClosePayment(ctx context.Context, orderNo string) error
}

// HandlePaymentTimeout 支付超时处理 Use Case
// 从 PaymentTimeoutProcessor.Handle() 提取的完整业务流程：
// 1. 查询订单状态
// 2. 如果订单不是待支付状态，直接返回
// 3. 查询支付状态，如果已支付则标记订单为已支付
// 4. 如果未支付，关闭订单、释放库存、关闭支付
type HandlePaymentTimeout struct {
	orders   PaymentOrderGateway
	stock    PaymentStockGateway
	payments PaymentGateway
}

// NewHandlePaymentTimeout 创建支付超时处理 Use Case 实例
func NewHandlePaymentTimeout(
	orders PaymentOrderGateway,
	stock PaymentStockGateway,
	payments PaymentGateway,
	_ any, // logger 参数保留用于签名兼容
) *HandlePaymentTimeout {
	return &HandlePaymentTimeout{
		orders:   orders,
		stock:    stock,
		payments: payments,
	}
}

// Execute 执行支付超时处理
func (uc *HandlePaymentTimeout) Execute(ctx context.Context, task model.PaymentTimeoutTask) error {
	orderNo := strings.TrimSpace(task.OrderNo)
	if orderNo == "" {
		return nil
	}

	// 1. 查询订单
	order, err := uc.orders.GetOrder(ctx, orderNo)
	if isNotFoundError(err) {
		commonlogs.CtxWarnf(ctx, "payment timeout order not found orderNo=%s", orderNo)
		return nil
	}
	if err != nil {
		if commonerrors.IsTemporaryRPCError(err) {
			return fmt.Errorf("get order %s: temporary rpc error, will retry: %w", orderNo, err)
		}
		return fmt.Errorf("terminal: %w", commonerrors.WrapTerminal(fmt.Errorf("get order %s: %w", orderNo, err)))
	}
	if order.Status != status.OrderPending {
		return nil
	}

	// 2. 检查是否已支付（可能在超时前已支付）
	query, err := uc.payments.QueryPayment(ctx, orderNo)
	if err == nil && query.PayStatus == status.PayStatusPaid {
		paidAt := query.PaidAt
		if paidAt == nil {
			now := time.Now()
			paidAt = &now
		}
		if err := uc.orders.MarkOrderPaid(ctx, orderNo, query.TransactionNo, *paidAt); err != nil {
			return fmt.Errorf("mark order paid %s: %w", orderNo, err)
		}
		return nil
	}
	if err != nil {
		// 临时错误需要重试
		if commonerrors.IsTemporaryRPCError(err) {
			return fmt.Errorf("query payment %s: temporary rpc error, will retry: %w", orderNo, err)
		}
		// 忽略未找到错误（订单无支付记录，直接关闭）
		if errors.Is(err, service.ErrNotFound) || commonerrors.IsRPCNotFoundError(err) {
			// 继续往下关闭订单
		} else {
			// 永久性未知错误，标记为终端错误避免无限重试
			return fmt.Errorf("terminal: %w", commonerrors.WrapTerminal(fmt.Errorf("query payment %s: %w", orderNo, err)))
		}
	}

	// 3. 关闭订单
	if err := uc.orders.CloseOrder(ctx, orderNo); err != nil {
		if isInvalidStateError(err) {
			return nil
		}
		if commonerrors.IsTemporaryRPCError(err) {
			return fmt.Errorf("close order %s: temporary rpc error, will retry: %w", orderNo, err)
		}
		return fmt.Errorf("terminal: %w", commonerrors.WrapTerminal(fmt.Errorf("close order %s: %w", orderNo, err)))
	}

	// 4. 验证订单已关闭
	closed, err := uc.orders.GetOrder(ctx, orderNo)
	if err != nil {
		if commonerrors.IsTemporaryRPCError(err) {
			return fmt.Errorf("get closed order %s: temporary rpc error, will retry: %w", orderNo, err)
		}
		return fmt.Errorf("mark timeout terminal: %w", commonerrors.WrapTerminal(fmt.Errorf("get closed order %s: %w", orderNo, err)))
	}
	if closed.Status != status.OrderClosed {
		return nil
	}

	// 5. 释放库存
	if err := uc.stock.ReleaseStock(ctx, order.ActivityNo, order.SKUNo, order.UserID, order.Quantity, orderNo); err != nil {
		if commonerrors.IsTemporaryRPCError(err) {
			return fmt.Errorf("release stock for order %s: temporary rpc error, will retry: %w", orderNo, err)
		}
		return fmt.Errorf("mark timeout terminal: %w", commonerrors.WrapTerminal(fmt.Errorf("release stock for order %s: %w", orderNo, err)))
	}
	if err := uc.payments.ClosePayment(ctx, orderNo); err != nil && !isNotFoundError(err) {
		if commonerrors.IsTemporaryRPCError(err) {
			return fmt.Errorf("close payment %s: temporary rpc error, will retry: %w", orderNo, err)
		}
		return fmt.Errorf("mark timeout terminal: %w", commonerrors.WrapTerminal(fmt.Errorf("close payment %s: %w", orderNo, err)))
	}

	commonlogs.CtxInfof(ctx, "payment timeout handled orderNo=%s userId=%d activityNo=%s createdAt=%s ageMinutes=%d source=delay_queue",
		orderNo, order.UserID, order.ActivityNo, order.CreatedAt.Format(time.RFC3339), int(time.Since(order.CreatedAt).Minutes()))
	return nil
}

// isNotFoundError 判断是否为未找到错误
func isNotFoundError(err error) bool {
	return errors.Is(err, service.ErrNotFound) || commonerrors.IsRPCNotFoundError(err)
}

// isInvalidStateError 判断是否为状态无效错误
func isInvalidStateError(err error) bool {
	return errors.Is(err, ErrInvalidState) || commonerrors.IsRPCFailedPreconditionError(err)
}
