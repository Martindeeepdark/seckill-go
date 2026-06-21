// Package repository 定义库存仓储接口
package repository

import (
	"context"
	"errors"
)

var (
	ErrNotFound      = errors.New("not found")       // 资源未找到错误
	ErrStockNotReady = errors.New("stock not ready") // 库存未就绪错误
)

// StockRepository 库存仓储接口，定义库存操作的核心方法
type StockRepository interface {
	// PeekStock 查询指定活动的SKU库存数量
	PeekStock(ctx context.Context, activityNo, skuNo string) (int64, error)
	// DeductStockWithLimit 扣减库存，支持购买限制检查
	// 返回是否扣减成功（可能因库存不足或超限而失败）
	DeductStockWithLimit(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, purchaseLimit int64, orderNo string) (bool, error)
	// ReleaseStock 释放库存（订单取消或超时）
	ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error
	// CleanupActivityStock 清理活动库存数据
	CleanupActivityStock(ctx context.Context, activityNo string, skuNos []string) (int64, error)
	// CleanupActivityPurchases 清理活动购买记录
	CleanupActivityPurchases(ctx context.Context, activityNo string) (int64, error)
}
