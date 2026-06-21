package model

// SubmitCommand 秒杀提交命令
type SubmitCommand struct {
	TraceID        string // 追踪 ID
	RequestTraceID string // 请求追踪 ID
	ActivityNo     string // 活动编号
	SKUNo          string // SKU 编号
	UserID         int64  // 用户 ID
	Quantity       int64  // 购买数量
	TotalFee       int64  // 总费用（分）
	RequestIP      string // 请求 IP
	RunID          string // smoke 压测 run-id
}
