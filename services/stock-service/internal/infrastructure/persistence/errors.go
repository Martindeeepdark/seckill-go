// Package persistence 提供持久化层实现
package persistence

import "seckill-stock-service/internal/domain/repository"

var (
	ErrNotFound      = repository.ErrNotFound      // 资源未找到错误
	ErrStockNotReady = repository.ErrStockNotReady // 库存未就绪错误
)
