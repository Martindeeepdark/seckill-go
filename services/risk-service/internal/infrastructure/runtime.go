// Package infrastructure 提供风险控制服务的基础设施层实现
package infrastructure

import (
	"context"
	"log/slog"

	"seckill-common/config"
	"seckill-risk-service/internal/domain/repository"
	"seckill-risk-service/internal/infrastructure/persistence"
)

// NewStore 根据配置创建持久化存储实例
// 优先使用 Redis 存储（如果配置了 Redis 地址），否则使用内存存储
// 参数：ctx-上下文，cfg-应用配置，logger-日志记录器
// 返回：风险仓储接口实现
func NewStore(ctx context.Context, cfg config.Config, logger *slog.Logger) repository.RiskRepository {
	memory := persistence.NewMemoryStore()
	var repo repository.RiskRepository = memory
	if cfg.RedisAddr != "" {
		redisStore, err := persistence.NewRedisStore(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, memory)
		if err != nil {
			logger.Warn("redis unavailable, using memory store", "addr", cfg.RedisAddr, "error", err)
		} else {
			repo = redisStore
			logger.Info("redis store enabled", "addr", cfg.RedisAddr)
		}
	}
	return repo
}
