// Package infrastructure 提供运行时存储工厂
package infrastructure

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"seckill-common/config"
	"seckill-common/database"
	"seckill-order-service/internal/domain/repository"
	"seckill-order-service/internal/infrastructure/persistence"
)

// NewStore 根据配置创建合适的存储实现
// 优先级: PostgreSQL > Redis > 内存存储
// ctx: 上下文
// cfg: 配置对象
// logger: 日志记录器
// 返回订单仓储实例
func NewStore(ctx context.Context, cfg config.Config, logger *slog.Logger) repository.OrderStore {
	// 尝试使用 PostgreSQL 存储
	if cfg.PGHost != "" {
		pgPool, err := pgxpool.New(ctx, database.DSN(cfg))
		if err == nil {
			if err := pgPool.Ping(ctx); err == nil {
				logger.Info("postgres store enabled", "host", cfg.PGHost, "database", cfg.PGDatabase)
				return persistence.NewPostgresStore(pgPool)
			}
		}
		logger.Warn("postgres unavailable, trying redis", "host", cfg.PGHost, "error", err)
	}

	// 尝试使用 Redis 存储
	if cfg.RedisAddr != "" {
		stdDB, err := sql.Open("pgx", database.DSN(cfg))
		if err == nil {
			defer func() {
				if closeErr := stdDB.Close(); closeErr != nil {
					logger.Warn("close db connection", "error", closeErr)
				}
			}()
		}
		memory := persistence.NewMemoryStore()
		redisStore, err := persistence.NewRedisStore(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, memory)
		if err != nil {
			logger.Warn("redis unavailable, using memory store", "addr", cfg.RedisAddr, "error", err)
			return memory
		}
		logger.Info("redis store enabled", "addr", cfg.RedisAddr)
		return redisStore
	}

	// 降级到内存存储
	return persistence.NewMemoryStore()
}
