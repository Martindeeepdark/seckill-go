// Package entity 提供订单领域的实体定义
package entity

import (
	"errors"
	"fmt"
	"time"

	"seckill-common/domain"
	"seckill-order-service/internal/domain/event"
)

// Order status constants.
const (
	OrderPending = "PENDING_PAY" // 待支付
	OrderPaid    = "PAID"        // 已支付
	OrderClosed  = "CLOSED"      // 已关闭
	OrderRefund  = "REFUNDED"    // 已退款
)

var (
	// ErrInvalidOrderNumber 订单号不能为空
	ErrInvalidOrderNumber = errors.New("order number cannot be empty")
	// ErrInvalidUserID 用户ID不能为零
	ErrInvalidUserID = errors.New("user ID cannot be zero")
	// ErrInvalidQuantity 数量必须为正数
	ErrInvalidQuantity = errors.New("quantity must be positive")
	// ErrInvalidPayAmount 支付金额必须为正数
	ErrInvalidPayAmount = errors.New("pay amount must be positive")
	// ErrInvalidPaymentAmount 支付金额不匹配
	ErrInvalidPaymentAmount = errors.New("payment amount does not match order amount")
	// ErrAlreadyPaid 订单已支付
	ErrAlreadyPaid = errors.New("order is already paid")
	// ErrAlreadyClosed 订单已关闭
	ErrAlreadyClosed = errors.New("order is already closed")
	// ErrPaidOrderCannotBeClosed 已支付订单不能关闭
	ErrPaidOrderCannotBeClosed = errors.New("paid order cannot be closed")
	// ErrRefundedOrderCannotBeClosed 已退款订单不能关闭
	ErrRefundedOrderCannotBeClosed = errors.New("refunded order cannot be closed")
)

// Order 订单聚合根
type Order struct {
	domain.AggregateRoot
	OrderNo        string     // 订单号
	UserID         int64      // 用户ID
	ActivityNo     string     // 活动编号
	SKUNo          string     // SKU编号
	Quantity       int64      // 购买数量
	PayAmount      int64      // 支付金额（分为单位）
	Status         string     // 订单状态
	TraceID        string     // 追踪ID
	RequestTraceID string     // 请求追踪ID（可选）
	TransactionNo  string     // 交易流水号（可选）
	PaidAt         *time.Time // 支付时间（可选）
	ClosedAt       *time.Time // 关闭时间（可选）
	CreatedAt      time.Time  // 创建时间
}

// CreateOrder 创建订单工厂方法
func CreateOrder(orderNo string, userID int64, activityNo, skuNo string, quantity, payAmount int64, traceID string) (*Order, error) {
	if orderNo == "" {
		return nil, ErrInvalidOrderNumber
	}
	if userID == 0 {
		return nil, ErrInvalidUserID
	}
	if quantity <= 0 {
		return nil, ErrInvalidQuantity
	}
	if payAmount <= 0 {
		return nil, ErrInvalidPayAmount
	}

	order := &Order{
		OrderNo:    orderNo,
		UserID:     userID,
		ActivityNo: activityNo,
		SKUNo:      skuNo,
		Quantity:   quantity,
		PayAmount:  payAmount,
		Status:     OrderPending,
		TraceID:    traceID,
		CreatedAt:  time.Now(),
	}

	// 记录订单创建事件
	order.RecordEvent(event.NewOrderCreatedEvent(
		orderNo,
		userID,
		activityNo,
		skuNo,
		quantity,
		payAmount,
	))

	return order, nil
}

// MarkAsPaid 标记订单为已支付
func (o *Order) MarkAsPaid(transactionNo string, amount int64, paidAt time.Time) error {
	if o.Status != OrderPending {
		if o.Status == OrderPaid {
			return fmt.Errorf("order %s: %w", o.OrderNo, ErrAlreadyPaid)
		}
		if o.Status == OrderClosed {
			return fmt.Errorf("order %s: %w", o.OrderNo, ErrAlreadyClosed)
		}
		return fmt.Errorf("order %s cannot be marked as paid in status %s", o.OrderNo, o.Status)
	}

	if amount != o.PayAmount {
		return fmt.Errorf("payment amount %d does not match order amount %d: %w", amount, o.PayAmount, ErrInvalidPaymentAmount)
	}

	o.Status = OrderPaid
	o.TransactionNo = transactionNo
	o.PaidAt = &paidAt

	// 记录订单支付事件
	o.RecordEvent(event.NewOrderPaidEvent(
		o.OrderNo,
		o.UserID,
		transactionNo,
		amount,
		paidAt,
	))

	return nil
}

// Close 关闭订单
func (o *Order) Close(closedAt time.Time) error {
	if o.Status == OrderClosed {
		return fmt.Errorf("order %s: %w", o.OrderNo, ErrAlreadyClosed)
	}

	// 已支付或已退款的订单不能关闭
	if o.Status == OrderPaid {
		return fmt.Errorf("paid order %s: %w", o.OrderNo, ErrPaidOrderCannotBeClosed)
	}
	if o.Status == OrderRefund {
		return fmt.Errorf("refunded order %s: %w", o.OrderNo, ErrRefundedOrderCannotBeClosed)
	}

	o.Status = OrderClosed
	o.ClosedAt = &closedAt

	// 记录订单关闭事件
	o.RecordEvent(event.NewOrderClosedEvent(
		o.OrderNo,
		o.UserID,
		"user_closed",
		closedAt,
	))

	return nil
}

// IsPending 检查订单是否待支付
func (o *Order) IsPending() bool {
	return o.Status == OrderPending
}

// IsPaid 检查订单是否已支付
func (o *Order) IsPaid() bool {
	return o.Status == OrderPaid
}

// CanBeReconciled 检查订单是否可以对账
func (o *Order) CanBeReconciled() bool {
	return o.Status == OrderPending || o.Status == OrderPaid
}
