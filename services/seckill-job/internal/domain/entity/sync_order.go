// Package entity 提供定时任务领域实体模型
package entity

import "time"

// SyncOrderRequest 订单同步请求
type SyncOrderRequest struct {
	OrderNo        string    // 订单编号
	UserID         int64     // 用户ID
	OrderSource    string    // 订单来源
	TotalAmount    int64     // 总金额（分）
	DiscountAmount int64     // 优惠金额（分）
	PayAmount      int64     // 实付金额（分）
	PaidAt         time.Time // 支付时间
	TransactionNo  string    // 交易流水号
}

// SyncedOrder 已同步订单
type SyncedOrder struct {
	OrderNo string // 订单编号
}
