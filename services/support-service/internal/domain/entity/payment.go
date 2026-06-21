// Package entity 定义支持服务的领域实体
package entity

import "time"

// CreatePayRequest 创建支付请求
type CreatePayRequest struct {
	OrderNo    string    // 订单号
	UserID     int64     // 用户ID
	PayAmount  int64     // 支付金额（单位：分）
	PayChannel string    // 支付渠道
	Subject    string    // 支付主题
	ExpireAt   time.Time // 过期时间
}

// PayResult 支付结果
type PayResult struct {
	OrderNo    string // 订单号
	PayChannel string // 支付渠道
	PrepayID   string // 预支付ID
	NonceStr   string // 随机字符串
	TimeStamp  string // 时间戳
	Sign       string // 签名
}

// PayQueryResult 支付查询结果
type PayQueryResult struct {
	OrderNo       string     // 订单号
	PayStatus     int64      // 支付状态
	TransactionNo string     // 交易流水号
	PaidAt        *time.Time // 支付时间
}

// Payment 支付实体
type Payment struct {
	Request       CreatePayRequest // 支付请求
	Result        PayResult        // 支付结果
	Status        int64            // 支付状态
	TransactionNo string           // 交易流水号
	PaidAt        *time.Time       // 支付时间
	CreatedAt     time.Time        // 创建时间
}
