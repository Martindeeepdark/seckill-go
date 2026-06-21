package entity

import "time"

// Order 订单实体（用于 application 层的订单网关返回值）
type Order struct {
	OrderNo       string     // 订单号
	UserID        int64      // 用户ID
	PayAmount     int64      // 支付金额（单位：分）
	Status        string     // 订单状态
	TransactionNo string     // 交易流水号
	PaidAt        *time.Time // 支付时间
}
