// Package persistence 提供数据持久化层实现
package persistence

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"seckill-common/config"
	"seckill-common/database"
	"seckill-support-service/internal/infrastructure/ledger"
)

// NewStore 创建持久化层，支持降级：
// 1. 优先尝试PostgreSQL（如果配置）
// 2. 如果PostgreSQL不可用，降级到内存账本
func NewStore(ctx context.Context, cfg config.Config, logger *slog.Logger) *ledger.SupportLedger {
	if cfg.PGHost != "" {
		pgPool, err := pgxpool.New(ctx, database.DSN(cfg))
		if err == nil {
			if err := pgPool.Ping(ctx); err == nil {
				defer pgPool.Close()
				conn, err := pgPool.Acquire(ctx)
				if err == nil {
					logger.Info("postgres store enabled", "host", cfg.PGHost, "database", cfg.PGDatabase)
					// 目前仍使用内存账本，因为PostgreSQL实现还不完整
					// 等所有方法实现后，可以返回 NewPostgresStore(conn.Conn())
					_ = conn.Conn()
					// return NewPostgresStore(conn.Conn())
				}
			}
		}
		logger.Warn("postgres unavailable, using memory store", "host", cfg.PGHost, "error", err)
	}

	logger.Info("using in-memory ledger store")
	return ledger.NewSupportLedger()
}

// TryNewPostgresStore 尝试创建PostgreSQL存储，如果不可用返回nil
func TryNewPostgresStore(ctx context.Context, cfg config.Config, logger *slog.Logger) *PostgresStore {
	if cfg.PGHost == "" {
		logger.Info("postgres not configured")
		return nil
	}

	pgPool, err := pgxpool.New(ctx, database.DSN(cfg))
	if err != nil {
		logger.Warn("failed to create postgres pool", "error", err)
		return nil
	}
	defer func() {
		if err != nil {
			pgPool.Close()
		}
	}()

	if err := pgPool.Ping(ctx); err != nil {
		logger.Warn("postgres ping failed", "error", err)
		return nil
	}

	conn, err := pgPool.Acquire(ctx)
	if err != nil {
		logger.Warn("failed to acquire postgres connection", "error", err)
		return nil
	}

	logger.Info("postgres store enabled", "host", cfg.PGHost, "database", cfg.PGDatabase)
	return NewPostgresStore(conn.Conn())
}
