// Package factory 提供订单工厂方法
package factory

import (
	"time"

	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/domain/vo"
)

// OrderFactory 订单工厂
type OrderFactory struct{}

// PendingOrderParams 创建待支付订单的参数
type PendingOrderParams struct {
	OrderNo        vo.OrderNo        // 订单号
	UserID         int64             // 用户ID
	ActivityNo     vo.ActivityNo     // 活动编号
	SKUNo          vo.SKUNo          // SKU编号
	Quantity       vo.Quantity       // 购买数量
	PayAmount      vo.Money          // 支付金额
	TraceID        vo.TraceID        // 追踪ID
	RequestTraceID vo.RequestTraceID // 请求追踪ID
	CreatedAt      time.Time         // 创建时间
}

// NewPending 创建一个新的待支付订单
// params: 订单创建参数
// 返回订单实体和是否创建成功
func (OrderFactory) NewPending(params PendingOrderParams) (entity.Order, bool) {
	// 验证用户ID和创建时间
	if params.UserID <= 0 || params.CreatedAt.IsZero() {
		return entity.Order{}, false
	}
	// 验证必填字段
	if params.OrderNo.String() == "" || params.ActivityNo.String() == "" || params.SKUNo.String() == "" || params.TraceID.String() == "" || params.RequestTraceID.String() == "" {
		return entity.Order{}, false
	}
	// 验证数量和金额
	if params.Quantity.Int() <= 0 || params.PayAmount.Cents() < 0 {
		return entity.Order{}, false
	}
	// 创建订单实体
	return entity.Order{
		OrderNo:        params.OrderNo.String(),
		UserID:         params.UserID,
		ActivityNo:     params.ActivityNo.String(),
		SKUNo:          params.SKUNo.String(),
		Quantity:       params.Quantity.Int(),
		PayAmount:      params.PayAmount.Cents(),
		Status:         entity.OrderPending,
		TraceID:        params.TraceID.String(),
		RequestTraceID: params.RequestTraceID.String(),
		CreatedAt:      params.CreatedAt,
	}, true
}
