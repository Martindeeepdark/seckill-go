// Package infrastructure 提供基础设施层的实现
package infrastructure

import (
	"context"
	"fmt"

	"seckill-stock-service/internal/domain/repository"
)

// LocalStockGateway 本地库存网关，封装库存仓储操作
type LocalStockGateway struct {
	Store repository.StockRepository // 库存仓储实例
}

// PeekStock 查询库存
func (g LocalStockGateway) PeekStock(ctx context.Context, activityNo, skuNo string) (int64, error) {
	stock, err := g.Store.PeekStock(ctx, activityNo, skuNo)
	if err != nil {
		return 0, fmt.Errorf("peek stock: %w", err)
	}
	return stock, nil
}

// DeductStockWithLimit 扣减库存（支持购买限制）
func (g LocalStockGateway) DeductStockWithLimit(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, purchaseLimit int64, orderNo string) (bool, error) {
	deducted, err := g.Store.DeductStockWithLimit(ctx, activityNo, skuNo, userID, quantity, purchaseLimit, orderNo)
	if err != nil {
		return false, fmt.Errorf("deduct stock with limit: %w", err)
	}
	return deducted, nil
}

// ReleaseStock 释放库存
func (g LocalStockGateway) ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error {
	if err := g.Store.ReleaseStock(ctx, activityNo, skuNo, userID, quantity, orderNo); err != nil {
		return fmt.Errorf("release stock: %w", err)
	}
	return nil
}

// CleanupActivityStock 清理活动库存数据
func (g LocalStockGateway) CleanupActivityStock(ctx context.Context, activityNo string, skuNos []string) (int64, error) {
	count, err := g.Store.CleanupActivityStock(ctx, activityNo, skuNos)
	if err != nil {
		return 0, fmt.Errorf("cleanup activity stock: %w", err)
	}
	return count, nil
}

// CleanupActivityPurchases 清理活动购买记录
func (g LocalStockGateway) CleanupActivityPurchases(ctx context.Context, activityNo string) (int64, error) {
	count, err := g.Store.CleanupActivityPurchases(ctx, activityNo)
	if err != nil {
		return 0, fmt.Errorf("cleanup activity purchases: %w", err)
	}
	return count, nil
}
