// Package application 提供 gateway 的应用层服务
// 包含秒杀、支付、管理等业务逻辑处理
package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	commonlogs "github.com/Martindeeepdark/go-common/logs"
)

// PaymentApp 处理支付相关操作
type PaymentApp struct {
	payment PaymentGateway
}

// NewPaymentApp 创建新的支付应用服务
func NewPaymentApp(payment PaymentGateway, _ any) *PaymentApp {
	return &PaymentApp{payment: payment}
}

// Prepay 发起订单支付
func (p *PaymentApp) Prepay(ctx context.Context, userID int64, orderNo, payChannel string) (*PayResult, error) {
	orderNo = strings.TrimSpace(orderNo)
	payChannel = strings.TrimSpace(payChannel)
	if orderNo == "" {
		return nil, fmt.Errorf("missing orderNo")
	}
	result, err := p.payment.Prepay(ctx, PrepayPaymentRequest{
		OrderNo:    orderNo,
		UserID:     userID,
		PayChannel: payChannel,
	})
	if err != nil {
		return nil, fmt.Errorf("prepay failed: %w", err)
	}
	commonlogs.CtxInfof(ctx, "prepay created orderNo=%s userId=%d payChannel=%s",
		orderNo, userID, payChannel)
	return result, nil
}

// Notify 处理支付回调通知
func (p *PaymentApp) Notify(ctx context.Context, orderNo, transactionNo, channel string) error {
	orderNo = strings.TrimSpace(orderNo)
	transactionNo = strings.TrimSpace(transactionNo)
	if orderNo == "" || transactionNo == "" {
		return fmt.Errorf("missing orderNo or transactionNo")
	}
	channel = strings.TrimSpace(channel)
	if err := p.payment.Notify(ctx, PaymentNotifyRequest{
		Channel:       channel,
		OrderNo:       orderNo,
		TransactionNo: transactionNo,
	}); err != nil {
		return fmt.Errorf("notify payment failed: %w", err)
	}
	commonlogs.CtxInfof(ctx, "payment notify received orderNo=%s transactionNo=%s channel=%s",
		orderNo, transactionNo, channel)
	return nil
}

// CloseExpiredPayment 关闭超过支付窗口的支付
func (p *PaymentApp) CloseExpiredPayment(ctx context.Context, orderNo string) error {
	if err := p.payment.ClosePayment(ctx, orderNo); err != nil {
		return fmt.Errorf("close payment failed: %w", err)
	}
	commonlogs.CtxInfof(ctx, "expired payment closed orderNo=%s", orderNo)
	return nil
}

// PaymentExpiry 返回支付窗口时长
func PaymentExpiry() time.Duration {
	return 310 * time.Second
}
