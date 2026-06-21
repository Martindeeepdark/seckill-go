// Package infrastructure 提供基础设施层的本地订单网关实现
package infrastructure

import (
	"context"
	"fmt"
	"time"

	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/domain/repository"
)

// LocalOrderGateway 本地订单网关，委托给订单仓储实现具体操作
type LocalOrderGateway struct {
	Store repository.OrderStore
}

// CreateOrder 创建订单
// ctx: 上下文
// order: 订单实体
// 返回错误表示创建失败
func (g LocalOrderGateway) CreateOrder(ctx context.Context, order entity.Order) error {
	if err := g.Store.CreateOrder(ctx, order); err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

// GetOrder 获取订单
// ctx: 上下文
// orderNo: 订单号
// 返回订单实体和错误
func (g LocalOrderGateway) GetOrder(ctx context.Context, orderNo string) (entity.Order, error) {
	order, err := g.Store.GetOrder(ctx, orderNo)
	if err != nil {
		return entity.Order{}, fmt.Errorf("get order: %w", err)
	}
	return order, nil
}

// ListOrdersByActivity 根据活动编号列出订单
// ctx: 上下文
// activityNo: 活动编号
// 返回订单列表和错误
func (g LocalOrderGateway) ListOrdersByActivity(ctx context.Context, activityNo string) ([]entity.Order, error) {
	orders, err := g.Store.ListOrdersByActivity(ctx, activityNo)
	if err != nil {
		return nil, fmt.Errorf("list orders by activity: %w", err)
	}
	return orders, nil
}

// ListOrdersByActivities 根据多个活动编号批量列出订单
// ctx: 上下文
// activityNos: 活动编号列表
// 返回按活动编号分组的订单映射和错误
func (g LocalOrderGateway) ListOrdersByActivities(ctx context.Context, activityNos []string) (map[string][]entity.Order, error) {
	orders, err := g.Store.ListOrdersByActivities(ctx, activityNos)
	if err != nil {
		return nil, fmt.Errorf("list orders by activities: %w", err)
	}
	return orders, nil
}

// ListOrdersByUser 根据用户ID列出订单
// ctx: 上下文
// userID: 用户ID
// 返回订单列表和错误
func (g LocalOrderGateway) ListOrdersByUser(ctx context.Context, userID int64) ([]entity.Order, error) {
	orders, err := g.Store.ListOrdersByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders by user: %w", err)
	}
	return orders, nil
}

// MarkOrderPaid 标记订单为已支付
// ctx: 上下文
// orderNo: 订单号
// transactionNo: 交易流水号
// paidAt: 支付时间
// 返回错误表示标记失败
func (g LocalOrderGateway) MarkOrderPaid(ctx context.Context, orderNo, transactionNo string, paidAt time.Time) error {
	if err := g.Store.MarkOrderPaid(ctx, orderNo, transactionNo, paidAt); err != nil {
		return fmt.Errorf("mark order paid: %w", err)
	}
	return nil
}

// CloseOrder 关闭订单
// ctx: 上下文
// orderNo: 订单号
// 返回错误表示关闭失败
func (g LocalOrderGateway) CloseOrder(ctx context.Context, orderNo string) error {
	if err := g.Store.CloseOrder(ctx, orderNo); err != nil {
		return fmt.Errorf("close order: %w", err)
	}
	return nil
}
