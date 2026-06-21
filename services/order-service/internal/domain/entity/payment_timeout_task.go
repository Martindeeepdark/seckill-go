// Package entity 提供订单领域的实体定义
package entity

import "time"

// PaymentTimeoutTask 支付超时任务实体
type PaymentTimeoutTask struct {
	ID             string    // 任务ID
	OrderNo        string    // 订单号
	RequestTraceID string    // 请求追踪ID（可选）
	DueAt          time.Time // 到期时间
	Attempts       int64     // 重试次数
	LastError      string    // 最后一次错误信息（可选）
	CreatedAt      time.Time // 创建时间
	UpdatedAt      time.Time // 更新时间（可选）
}
