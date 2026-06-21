// Package rpc 定义订单网关接口和数据传输对象
package rpc

import "context"

// OrderGateway 订单网关接口，定义订单相关的 RPC 操作
type OrderGateway interface {
	// CreateOrder 创建订单
	CreateOrder(ctx context.Context, orderNo string, userID int64, activityNo, skuNo string, quantity int, payAmount int64, status, traceID, requestTraceID string) error
	// GetOrder 获取订单
	GetOrder(ctx context.Context, orderNo string) (OrderDTO, error)
	// ListOrdersByActivity 根据活动编号列出订单
	ListOrdersByActivity(ctx context.Context, activityNo string) ([]OrderDTO, error)
	// ListOrdersByActivities 根据多个活动编号批量列出订单
	ListOrdersByActivities(ctx context.Context, activityNos []string) (map[string][]OrderDTO, error)
	// ListOrdersByUser 根据用户ID列出订单
	ListOrdersByUser(ctx context.Context, userID int64) ([]OrderDTO, error)
	// MarkOrderPaid 标记订单为已支付
	MarkOrderPaid(ctx context.Context, orderNo, transactionNo string, paidAt interface{}) error
	// CloseOrder 关闭订单
	CloseOrder(ctx context.Context, orderNo string) error
}

// OrderDTO 订单数据传输对象
type OrderDTO struct {
	OrderNo        string      // 订单号
	UserID         int64       // 用户ID
	ActivityNo     string      // 活动编号
	SKUNo          string      // SKU编号
	Quantity       int         // 购买数量
	PayAmount      int64       // 支付金额（分为单位）
	Status         string      // 订单状态
	TraceID        string      // 追踪ID
	RequestTraceID string      // 请求追踪ID
	TransactionNo  string      // 交易流水号
	PaidAt         interface{} // 支付时间
	ClosedAt       interface{} // 关闭时间
	CreatedAt      interface{} // 创建时间
}
