// Package event 定义秒杀领域的领域事件
package event

const (
	TopicOrderCreated    = "order.created"    // 订单创建事件主题
	TopicSeckillRejected = "seckill.rejected" // 秒杀拒绝事件主题
	ReasonActivityClosed = "ACTIVITY_CLOSED"  // 拒绝原因：活动已关闭
	ReasonStockEmpty     = "STOCK_EMPTY"      // 拒绝原因：库存不足
	ReasonRiskUser       = "RISK_USER"        // 拒绝原因：风控拦截
	ReasonOrderFail      = "ORDER_FAIL"       // 拒绝原因：订单创建失败
)

// OrderCreated 订单创建事件
// 在处理器创建待支付订单后发布
type OrderCreated struct {
	OrderNo        string // 订单号
	UserID         int64  // 用户 ID
	ActivityNo     string // 活动编号
	SKUNo          string // 商品 SKU 编号
	Quantity       int64  // 购买数量
	PayAmount      int64  // 支付金额
	TraceID        string // 追踪 ID
	RequestTraceID string // 请求追踪 ID
}

// SeckillRejected 秒杀拒绝事件
// 当秒杀消息被业务规则拒绝时发布
type SeckillRejected struct {
	TraceID        string // 追踪 ID
	Reason         string // 拒绝原因
	RequestTraceID string // 请求追踪 ID
}
