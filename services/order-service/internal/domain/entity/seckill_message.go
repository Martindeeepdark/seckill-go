// Package entity 提供订单领域的实体定义
package entity

import "time"

// SeckillMessage 秒杀消息实体
type SeckillMessage struct {
	UserID         int64     // 用户ID
	ActivityNo     string    // 活动编号
	SKUNo          string    // SKU编号
	Quantity       int64     // 购买数量
	TotalFee       int64     // 总费用（分为单位）
	Token          string    // 秒杀令牌
	TraceID        string    // 追踪ID
	RequestTraceID string    // 请求追踪ID（可选）
	RequestTime    time.Time // 请求时间
}
