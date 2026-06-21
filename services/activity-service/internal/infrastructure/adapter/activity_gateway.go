// Package adapter 提供仓储接口到领域网关的适配器实现。
package adapter

import (
	"context"
	"fmt"

	domain "seckill-activity-service/internal/domain/entity"
	"seckill-activity-service/internal/domain/repository"
)

// LocalActivityGateway 将本地仓储接口适配为应用层所需的活动网关。
type LocalActivityGateway struct {
	Store repository.Store
}

// ListActivities 列出所有活动。
func (g LocalActivityGateway) ListActivities(ctx context.Context) ([]domain.Activity, error) {
	activities, err := g.Store.ListActivities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	return activities, nil
}

// GetActivity 获取单个活动。
func (g LocalActivityGateway) GetActivity(ctx context.Context, activityNo string) (domain.Activity, error) {
	activity, err := g.Store.GetActivity(ctx, activityNo)
	if err != nil {
		return domain.Activity{}, fmt.Errorf("get activity: %w", err)
	}
	return activity, nil
}

// GetSKU 获取活动商品。
func (g LocalActivityGateway) GetSKU(ctx context.Context, activityNo, skuNo string) (domain.SKU, error) {
	sku, err := g.Store.GetSKU(ctx, activityNo, skuNo)
	if err != nil {
		return domain.SKU{}, fmt.Errorf("get sku: %w", err)
	}
	return sku, nil
}

// CreateActivity 创建活动并返回创建结果。
func (g LocalActivityGateway) CreateActivity(ctx context.Context, activity domain.Activity) (domain.Activity, error) {
	if err := g.Store.AddActivity(ctx, activity); err != nil {
		return domain.Activity{}, fmt.Errorf("add activity: %w", err)
	}
	activity, err := g.Store.GetActivity(ctx, activity.ActivityNo)
	if err != nil {
		return domain.Activity{}, fmt.Errorf("get activity from store: %w", err)
	}
	return activity, nil
}

// UpdateActivity 更新活动。
func (g LocalActivityGateway) UpdateActivity(ctx context.Context, activity domain.Activity) error {
	if err := g.Store.UpdateActivity(ctx, activity); err != nil {
		return fmt.Errorf("update activity: %w", err)
	}
	return nil
}

// UpdateActivityStatus 更新活动状态。
func (g LocalActivityGateway) UpdateActivityStatus(ctx context.Context, activityNo string, status int64) error {
	if err := g.Store.UpdateActivityStatus(ctx, activityNo, status); err != nil {
		return fmt.Errorf("update activity status: %w", err)
	}
	return nil
}

// AddActivitySKU 向活动添加商品。
func (g LocalActivityGateway) AddActivitySKU(ctx context.Context, activityNo string, sku domain.SKU) error {
	if err := g.Store.AddActivitySKU(ctx, activityNo, sku); err != nil {
		return fmt.Errorf("add activity sku: %w", err)
	}
	return nil
}

// RemoveActivitySKU 从活动移除商品。
func (g LocalActivityGateway) RemoveActivitySKU(ctx context.Context, activityNo, skuNo string) error {
	if err := g.Store.RemoveActivitySKU(ctx, activityNo, skuNo); err != nil {
		return fmt.Errorf("remove activity sku: %w", err)
	}
	return nil
}
