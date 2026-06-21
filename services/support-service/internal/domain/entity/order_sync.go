// Package entity 定义支持服务的领域实体
package entity

import "time"

// SyncOrderRequest 订单同步请求
type SyncOrderRequest struct {
	OrderNo        string    // 订单号
	UserID         int64     // 用户ID
	OrderSource    string    // 订单来源
	TotalAmount    int64     // 订单总金额（单位：分）
	DiscountAmount int64     // 折扣金额（单位：分）
	PayAmount      int64     // 实付金额（单位：分）
	PaidAt         time.Time // 支付时间
	TransactionNo  string    // 交易流水号
}

// SyncedOrder 已同步的订单
type SyncedOrder struct {
	OrderNo        string     // 订单号
	UserID         int64      // 用户ID
	OrderSource    string     // 订单来源
	TotalAmount    int64      // 订单总金额（单位：分）
	DiscountAmount int64      // 折扣金额（单位：分）
	PayAmount      int64      // 实付金额（单位：分）
	OrderStatus    int64      // 订单状态
	PaidAt         time.Time  // 支付时间
	CompletedAt    *time.Time // 完成时间
	TransactionNo  string     // 交易流水号
	CreatedAt      time.Time  // 创建时间
}
