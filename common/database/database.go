// Package database 提供 PostgreSQL 数据库连接池管理
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"seckill-common/config"
)

// NewPool 创建 PostgreSQL 数据库连接池
// 配置连接池参数并验证连接
func NewPool(ctx context.Context, cfg config.Config, logger *slog.Logger) (*sql.DB, error) {
	dsn := DSN(cfg)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	// 配置连接池参数
	db.SetMaxOpenConns(25)                 // 最大打开连接数
	db.SetMaxIdleConns(5)                  // 最大空闲连接数
	db.SetConnMaxLifetime(5 * time.Minute) // 连接最大生命周期
	if err := db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			logger.Error("close postgres on ping failure", "error", closeErr)
		}
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	logger.Info("postgres connected", "host", cfg.PGHost, "database", cfg.PGDatabase)
	return db, nil
}

// DSN 生成 PostgreSQL 数据源名称（连接字符串）
// 格式：postgres://user:password@host:port/database?sslmode=disable
func DSN(cfg config.Config) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.PGUser, cfg.PGPassword, cfg.PGHost, cfg.PGPort, cfg.PGDatabase,
	)
}
