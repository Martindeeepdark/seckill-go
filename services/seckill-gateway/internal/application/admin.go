package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ========== 活动 Admin ==========

// ActivityAdminApp 管理活动相关的 CRUD 操作
type ActivityAdminApp struct {
	activity ActivityGateway
}

// NewActivityAdminApp 创建活动管理应用服务
func NewActivityAdminApp(activity ActivityGateway, _ any) *ActivityAdminApp {
	return &ActivityAdminApp{activity: activity}
}

// ListActivities 查询全部活动列表。
func (a *ActivityAdminApp) ListActivities(ctx context.Context) (ActivityList, error) {
	res, err := a.activity.ListActivities(ctx)
	if err != nil {
		return ActivityList{}, fmt.Errorf("list activities: %w", err)
	}
	return res, nil
}

// GetActivity 根据活动编号查询活动详情。
func (a *ActivityAdminApp) GetActivity(ctx context.Context, activityNo string) (*ActivityDetail, error) {
	res, err := a.activity.GetActivity(ctx, activityNo)
	if err != nil {
		return nil, fmt.Errorf("get activity %s: %w", activityNo, err)
	}
	return res, nil
}

// CreateActivity 校验并创建活动，返回新建活动详情。
func (a *ActivityAdminApp) CreateActivity(ctx context.Context, req CreateActivityRequest) (*ActivityDetail, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	res, err := a.activity.CreateActivity(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create activity: %w", err)
	}
	return res, nil
}

// UpdateActivity 校验活动编号并更新活动信息。
func (a *ActivityAdminApp) UpdateActivity(ctx context.Context, req UpdateActivityRequest) error {
	if strings.TrimSpace(req.ActivityNo) == "" {
		return errors.New("activityNo is required")
	}
	if err := a.activity.UpdateActivity(ctx, req); err != nil {
		return fmt.Errorf("update activity: %w", err)
	}
	return nil
}

// EndActivity 校验活动编号并结束指定活动。
func (a *ActivityAdminApp) EndActivity(ctx context.Context, activityNo string) error {
	if strings.TrimSpace(activityNo) == "" {
		return errors.New("activityNo is required")
	}
	if err := a.activity.EndActivity(ctx, activityNo); err != nil {
		return fmt.Errorf("end activity %s: %w", activityNo, err)
	}
	return nil
}

// AddProduct 校验并为活动添加 SKU 商品。
func (a *ActivityAdminApp) AddProduct(ctx context.Context, req AddProductRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	if err := a.activity.AddProduct(ctx, req); err != nil {
		return fmt.Errorf("add product: %w", err)
	}
	return nil
}

// RemoveProduct 从指定活动中移除 SKU 商品。
func (a *ActivityAdminApp) RemoveProduct(ctx context.Context, activityNo, skuNo string) error {
	if strings.TrimSpace(activityNo) == "" || strings.TrimSpace(skuNo) == "" {
		return errors.New("activityNo and skuNo are required")
	}
	if err := a.activity.RemoveProduct(ctx, activityNo, skuNo); err != nil {
		return fmt.Errorf("remove product %s from %s: %w", skuNo, activityNo, err)
	}
	return nil
}

// ========== 订单 Admin ==========

// OrderAdminApp 管理订单相关的操作
type OrderAdminApp struct {
	order OrderGateway
}

// NewOrderAdminApp 创建订单管理应用服务
func NewOrderAdminApp(order OrderGateway, _ any) *OrderAdminApp {
	return &OrderAdminApp{order: order}
}

// GetOrder 根据订单号查询订单详情。
func (o *OrderAdminApp) GetOrder(ctx context.Context, orderNo string) (*OrderDetail, error) {
	res, err := o.order.GetOrder(ctx, orderNo)
	if err != nil {
		return nil, fmt.Errorf("get order %s: %w", orderNo, err)
	}
	return res, nil
}

// ListOrdersByActivity 查询指定活动下的全部订单。
func (o *OrderAdminApp) ListOrdersByActivity(ctx context.Context, activityNo string) ([]OrderDetail, error) {
	res, err := o.order.ListOrdersByActivity(ctx, activityNo)
	if err != nil {
		return nil, fmt.Errorf("list orders by activity %s: %w", activityNo, err)
	}
	return res, nil
}

// ListOrdersByUser 查询指定用户的全部订单。
func (o *OrderAdminApp) ListOrdersByUser(ctx context.Context, userID int64) ([]OrderDetail, error) {
	res, err := o.order.ListOrdersByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders by user %d: %w", userID, err)
	}
	return res, nil
}

// CloseOrder 校验订单号并关闭指定订单。
func (o *OrderAdminApp) CloseOrder(ctx context.Context, orderNo string) error {
	if strings.TrimSpace(orderNo) == "" {
		return errors.New("orderNo is required")
	}
	if err := o.order.CloseOrder(ctx, orderNo); err != nil {
		return fmt.Errorf("close order %s: %w", orderNo, err)
	}
	return nil
}

// ========== 库存 Admin ==========

// StockAdminApp 管理库存相关的操作
type StockAdminApp struct {
	stock StockGateway
}

// NewStockAdminApp 创建库存管理应用服务
func NewStockAdminApp(stock StockGateway, _ any) *StockAdminApp {
	return &StockAdminApp{stock: stock}
}

// PeekStock 查询指定活动和 SKU 的当前剩余库存。
func (s *StockAdminApp) PeekStock(ctx context.Context, activityNo, skuNo string) (int64, error) {
	res, err := s.stock.Peek(ctx, activityNo, skuNo)
	if err != nil {
		return 0, fmt.Errorf("peek stock for %s/%s: %w", activityNo, skuNo, err)
	}
	return res, nil
}
