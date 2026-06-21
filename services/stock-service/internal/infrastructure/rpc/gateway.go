package rpc

import "context"

// StockGateway 库存网关接口
type StockGateway interface {
	// PeekStock 查询库存
	PeekStock(ctx context.Context, activityNo, skuNo string) (int64, error)
	// DeductStockWithLimit 扣减库存（支持购买限制）
	DeductStockWithLimit(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, purchaseLimit int64, orderNo string) (bool, error)
	// ReleaseStock 释放库存
	ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error
	// CleanupActivityStock 清理活动库存数据
	CleanupActivityStock(ctx context.Context, activityNo string, skuNos []string) (int64, error)
	// CleanupActivityPurchases 清理活动购买记录
	CleanupActivityPurchases(ctx context.Context, activityNo string) (int64, error)
}
