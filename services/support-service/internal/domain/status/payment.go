// Package status 提供状态机转换逻辑
package status

import (
	"seckill-support-service/internal/domain/entity"
	"slices"
	"time"
)

const (
	PayStatusPending = 0 // 待支付
	PayStatusPaid    = 1 // 已支付
	PayStatusClosed  = 2 // 已关闭
)

// PaymentAllowedTransitions 支付状态允许的转换映射
var PaymentAllowedTransitions = map[int64][]int64{
	PayStatusPending: {PayStatusPaid, PayStatusClosed}, // 待支付 → 已支付/已关闭
	PayStatusPaid:    {},                               // 已支付 → 终态
	PayStatusClosed:  {},                               // 已关闭 → 终态
}

// CanPaymentTransitionTo 检查支付是否可以从from状态转换到to状态
func CanPaymentTransitionTo(from int64, to int64) bool {
	targets, ok := PaymentAllowedTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(targets, to)
}

// TransitPaymentPaid 将支付状态转换为已支付
func TransitPaymentPaid(p entity.Payment, txn string, paidAt time.Time) (entity.Payment, bool) {
	if p.Status == PayStatusPaid {
		return p, true
	}
	if !CanPaymentTransitionTo(p.Status, PayStatusPaid) {
		return p, false
	}
	p.Status = PayStatusPaid
	p.TransactionNo = txn
	p.PaidAt = &paidAt
	return p, true
}

// TransitPaymentClosed 将支付状态转换为已关闭
func TransitPaymentClosed(p entity.Payment) (entity.Payment, bool) {
	if p.Status == PayStatusClosed {
		return p, true
	}
	if !CanPaymentTransitionTo(p.Status, PayStatusClosed) {
		return p, false
	}
	p.Status = PayStatusClosed
	return p, true
}
