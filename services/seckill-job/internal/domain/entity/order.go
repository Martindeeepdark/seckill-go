// Package entity 提供定时任务领域实体模型
package entity

import "time"

// Order 订单实体
type Order struct {
	OrderNo       string     // 订单编号
	UserID        int64      // 用户ID
	ActivityNo    string     // 活动编号
	SKUNo         string     // SKU编号
	Quantity      int        // 购买数量
	PayAmount     int64      // 支付金额（分）
	Status        string     // 订单状态
	TransactionNo string     // 交易流水号
	PaidAt        *time.Time // 支付时间
	CreatedAt     time.Time  // 创建时间
}

// PayQueryResult 支付查询结果
type PayQueryResult struct {
	OrderNo       string     // 订单编号
	PayStatus     int        // 支付状态（0待支付/1已支付）
	TransactionNo string     // 交易流水号
	PaidAt        *time.Time // 支付时间
}

// Order status constants
const (
	OrderPending = "PENDING_PAY" // 订单待支付
	OrderPaid    = "PAID"        // 订单已支付
	OrderClosed  = "CLOSED"      // 订单已关闭
)

// Payment status constants
const (
	PayStatusPending = 0 // 支付待处理
	PayStatusPaid    = 1 // 支付成功
)

// IsPending checks if the order is awaiting payment.
func (o *Order) IsPending() bool {
	return o.Status == OrderPending
}

// IsPaid checks if the order has been paid.
func (o *Order) IsPaid() bool {
	return o.Status == OrderPaid
}

// IsPendingOrPaid checks if the order is in an active state.
func (o *Order) IsPendingOrPaid() bool {
	return o.Status == OrderPending || o.Status == OrderPaid
}
