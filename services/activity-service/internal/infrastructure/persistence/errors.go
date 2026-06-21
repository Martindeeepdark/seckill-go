// Package persistence 提供仓储错误的公共导出。
package persistence

import "seckill-activity-service/internal/domain/repository"

var (
	ErrNotFound      = repository.ErrNotFound
	ErrDuplicate     = repository.ErrDuplicate
	ErrStockNotReady = repository.ErrStockNotReady
	ErrInvalidState  = repository.ErrInvalidState
)

// TraceProcessing 表示 trace 链路处于处理中状态的常量值。
const TraceProcessing = repository.TraceProcessing
