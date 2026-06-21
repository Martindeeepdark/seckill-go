package cache

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Martindeeepdark/go-common/cache/twocache"

	"seckill-risk-service/internal/domain/entity"
	"seckill-risk-service/internal/domain/repository"
)

const (
	riskUserKeyPrefix = "risk:blacklist:"
)

var _ repository.RiskRepository = (*RiskCache)(nil)

type riskFlag struct {
	Value bool
}

// EntryConfig 描述单层缓存的 BigCache 参数。
// 定义在 cache 包内，避免 cache 包反向依赖 config 包。
type EntryConfig struct {
	LocalTTL   time.Duration
	Shards     int
	MaxEntries int
}

// Config 描述 RiskCache 的构造参数。
type Config struct {
	Risk EntryConfig
}

// DefaultConfig 返回一组保守的默认缓存参数，用于测试或未配置场景。
func DefaultConfig() Config {
	return Config{
		Risk: EntryConfig{
			LocalTTL:   60 * time.Second,
			Shards:     16,
			MaxEntries: 2048,
		},
	}
}

func withDefaults(cfg Config) Config {
	d := DefaultConfig()
	if cfg.Risk.LocalTTL <= 0 {
		cfg.Risk.LocalTTL = d.Risk.LocalTTL
	}
	if cfg.Risk.Shards <= 0 {
		cfg.Risk.Shards = d.Risk.Shards
	}
	if cfg.Risk.MaxEntries <= 0 {
		cfg.Risk.MaxEntries = d.Risk.MaxEntries
	}
	return cfg
}

// RiskCache 基于 Redis + 本地 BigCache 的两级风控缓存，实现 RiskRepository 接口。
type RiskCache struct {
	source repository.RiskRepository
	cache  *twocache.TwoLevelCache[string, *riskFlag]
	logger *slog.Logger
}

// NewRiskCache 创建风控缓存，source 为回源仓储，cfg 控制本地缓存参数。
func NewRiskCache(source repository.RiskRepository, cfg Config, logger *slog.Logger) (*RiskCache, error) {
	if source == nil {
		return nil, fmt.Errorf("risk cache: source is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	cfg = withDefaults(cfg)
	c, err := twocache.New[string, *riskFlag](
		twocache.WithKeyEncoder[string, *riskFlag](func(key string) string {
			return riskUserKeyPrefix + key
		}),
		twocache.WithLocalTTL[string, *riskFlag](cfg.Risk.LocalTTL),
		twocache.WithShards[string, *riskFlag](cfg.Risk.Shards),
		twocache.WithMaxEntries[string, *riskFlag](cfg.Risk.MaxEntries),
	)
	if err != nil {
		return nil, fmt.Errorf("create risk user cache: %w", err)
	}
	return &RiskCache{source: source, cache: c, logger: logger}, nil
}

// IsRiskUser 判断用户是否为风控用户，命中缓存则直接返回，否则回源查询。
func (c *RiskCache) IsRiskUser(ctx context.Context, userID int64) (bool, error) {
	key := strconv.FormatInt(userID, 10)
	flag, err := c.cache.GetOrLoad(ctx, key, func(ctx context.Context, _ string) (*riskFlag, error) {
		v, err := c.source.IsRiskUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		return &riskFlag{Value: v}, nil
	})
	if err != nil {
		return false, err
	}
	if flag == nil {
		return false, nil
	}
	return flag.Value, nil
}

// MarkRiskUser 标记用户为风控用户并按 ttl 设置有效期，写入后失效本地缓存。
func (c *RiskCache) MarkRiskUser(ctx context.Context, userID int64, ttl time.Duration) error {
	if err := c.source.MarkRiskUser(ctx, userID, ttl); err != nil {
		return err
	}
	c.invalidate(ctx, userID)
	return nil
}

// RecordRiskAction 记录一条风控行为到回源仓储。
func (c *RiskCache) RecordRiskAction(ctx context.Context, record entity.RiskRecord) error {
	return c.source.RecordRiskAction(ctx, record)
}

// CountRecentRiskActions 统计用户自 since 以来指定 actionType 的风控行为次数。
func (c *RiskCache) CountRecentRiskActions(ctx context.Context, userID int64, actionType string, since time.Time) (int, error) {
	return c.source.CountRecentRiskActions(ctx, userID, actionType, since)
}

// HasHighRiskRecord 判断用户自 since 以来是否存在高危风控记录。
func (c *RiskCache) HasHighRiskRecord(ctx context.Context, userID int64, since time.Time) (bool, error) {
	return c.source.HasHighRiskRecord(ctx, userID, since)
}

// ListRiskRecords 查询用户的风控记录列表。
func (c *RiskCache) ListRiskRecords(ctx context.Context, userID int64) ([]entity.RiskRecord, error) {
	return c.source.ListRiskRecords(ctx, userID)
}

// CleanupExpiredRiskUsers 清理过期的风控用户，返回清理数量。
func (c *RiskCache) CleanupExpiredRiskUsers(ctx context.Context) (int, error) {
	return c.source.CleanupExpiredRiskUsers(ctx)
}

// Close 释放底层两级缓存资源。
func (c *RiskCache) Close() error {
	if c.cache == nil {
		return nil
	}
	return c.cache.Close()
}

func (c *RiskCache) invalidate(ctx context.Context, userID int64) {
	key := strconv.FormatInt(userID, 10)
	if err := c.cache.Delete(ctx, key); err != nil {
		c.logger.Warn("risk cache invalidate failed", "userID", userID, "error", err)
	}
}
