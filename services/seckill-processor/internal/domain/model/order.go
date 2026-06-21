package model

import "time"

// OrderRequest 订单创建请求
type OrderRequest struct {
	OrderNo        string    // 订单号
	UserID         int64     // 用户 ID
	ActivityNo     string    // 活动编号
	SKUNo          string    // SKU 编号
	Quantity       int64     // 购买数量
	PayAmount      int64     // 支付金额
	Status         string    // 订单状态
	TraceID        string    // 追踪 ID
	RequestTraceID string    // 请求追踪 ID
	CreatedAt      time.Time // 创建时间
}

// OrderInfo 订单信息
type OrderInfo struct {
	OrderNo       string     // 订单号
	UserID        int64      // 用户 ID
	ActivityNo    string     // 活动编号
	SKUNo         string     // SKU 编号
	Quantity      int64      // 购买数量
	PayAmount     int64      // 支付金额
	Status        string     // 订单状态
	TransactionNo string     // 交易流水号
	PaidAt        *time.Time // 支付时间
	CreatedAt     time.Time  // 创建时间
}

// PayQueryResult 支付查询结果
type PayQueryResult struct {
	OrderNo       string     // 订单号
	PayStatus     int        // 支付状态
	TransactionNo string     // 交易流水号
	PaidAt        *time.Time // 支付时间
}
