// Package persistence 提供风险控制的持久化层实现
package persistence

import (
	"seckill-risk-service/internal/domain/repository"
)

// RiskRepository 是风险仓储的类型别名，便于外部引用
type RiskRepository = repository.RiskRepository
