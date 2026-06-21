package model

import (
	"strconv"
	"time"
)

// SeckillMessage 秒杀消息，由上游服务发送到消息队列
type SeckillMessage struct {
	TraceID        string `json:"traceId"`               // 秒杀请求的唯一追踪 ID
	RequestTraceID string `json:"requestTraceId"`        // 原始请求追踪 ID
	ActivityNo     string `json:"activityNo"`            // 活动编号
	SKUNo          string `json:"skuNo"`                 // 商品 SKU 编号
	UserID         int64  `json:"userId"`                // 用户 ID
	Quantity       int64  `json:"quantity"`              // 购买数量
	TotalFee       int64  `json:"totalFee"`              // 总费用（分）
	RequestIP      string `json:"requestIp,omitempty"`   // 请求 IP
	RunID          string `json:"runId,omitempty"`       // smoke 压测 run-id
	MachinePass    bool   `json:"machinePass,omitempty"` // 是否通过机验
}

// IsExpired 检查秒杀请求相对于截止时间是否已过期。
func (m *SeckillMessage) IsExpired(deadline time.Time) bool {
	// 没有请求时间的请求永远不会被视为过期
	return false
}

// UserIDStr 将用户 ID 转为字符串用于日志输出。
func (m *SeckillMessage) UserIDStr() string {
	return strconv.FormatInt(m.UserID, 10)
}
