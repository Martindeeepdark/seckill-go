// Package repository 定义活动领域的数据仓储接口。
package repository

import (
	"context"
	"errors"
	"time"

	domain "seckill-activity-service/internal/domain/entity"
)

var (
	// 错误定义已迁移到 activity.go，这里保留引用以兼容旧代码
	ErrStockNotReady = errors.New("stock not ready")
	ErrInvalidState  = errors.New("invalid state")
)

// TraceProcessing 表示 trace 链路处于处理中状态的标记值。
const TraceProcessing = "PROCESSING"

// Store 定义活动仓储接口，包含活动管理和库存操作的完整方法集。
type Store interface {
	// 活动管理
	AddActivity(ctx context.Context, activity domain.Activity) error
	UpdateActivity(ctx context.Context, activity domain.Activity) error
	UpdateActivityStatus(ctx context.Context, activityNo string, status int64) error
	AddActivitySKU(ctx context.Context, activityNo string, sku domain.SKU) error
	RemoveActivitySKU(ctx context.Context, activityNo, skuNo string) error
	ListActivities(ctx context.Context) ([]domain.Activity, error)
	GetActivity(ctx context.Context, activityNo string) (domain.Activity, error)
	GetSKU(ctx context.Context, activityNo, skuNo string) (domain.SKU, error)

	// 库存操作
	PeekStock(ctx context.Context, activityNo, skuNo string) (int64, error)
	DeductStockWithLimit(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, purchaseLimit int64) (bool, error)
	ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64) error
	CleanupActivityStock(ctx context.Context, activityNo string, skuNos []string) (int64, error)
	CleanupActivityPurchases(ctx context.Context, activityNo string) (int64, error)

	// 异步链路追踪
	TryStartTrace(ctx context.Context, traceID string, ttl time.Duration) (bool, error)
	MarkTraceSuccess(ctx context.Context, traceID, orderNo string, ttl time.Duration) error
	MarkTraceFail(ctx context.Context, traceID, reason string, ttl time.Duration) error
	GetTraceResult(ctx context.Context, traceID string) (string, error)
	DeleteTrace(ctx context.Context, traceID string) error
}
