package infrastructure

import (
	"context"
	"log/slog"

	"seckill-common/config"
	"seckill-stock-service/internal/domain/repository"
	"seckill-stock-service/internal/infrastructure/persistence"
)

// NewStore 创建库存存储实例（优先使用Redis，回退到内存）
func NewStore(ctx context.Context, cfg config.Config, logger *slog.Logger) repository.StockRepository {
	memory := persistence.NewMemoryStore()
	var repo repository.StockRepository = memory
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
