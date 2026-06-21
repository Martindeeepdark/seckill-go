// Package cache 提供活动查询的二级缓存实现（BigCache 本地 + Redis）。
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	commonredis "github.com/Martindeeepdark/go-common/cache/redis"
	"github.com/Martindeeepdark/go-common/cache/twocache"
	goredis "github.com/redis/go-redis/v9"

	"seckill-activity-service/internal/config"
	domain "seckill-activity-service/internal/domain/entity"
	"seckill-activity-service/internal/domain/repository"
)

const (
	activeActivityListKey    = "seckill:activity:active:all"
	activityInfoKeyPrefix    = "seckill:activity:info:"
	activityProductKeyPrefix = "seckill:activity:product:list:"
)

// ActivityGateway 定义活动缓存的数据源接口。
type ActivityGateway interface {
	ListActivities(ctx context.Context) ([]domain.Activity, error)
	GetActivity(ctx context.Context, activityNo string) (domain.Activity, error)
	GetSKU(ctx context.Context, activityNo, skuNo string) (domain.SKU, error)
}

// CachedActivityGateway 使用 twocache（BigCache L1 + Redis L2）实现活动缓存。
type CachedActivityGateway struct {
	source ActivityGateway
	cache  *twocache.TwoLevelCache[string, json.RawMessage]
	client *commonredis.Client
	logger *slog.Logger
}

// NewCachedActivityGateway 创建基于 twocache 的活动网关。
func NewCachedActivityGateway(ctx context.Context, cfg config.Config, source ActivityGateway, logger *slog.Logger) (ActivityGateway, func() error) {
	if source == nil || !cfg.Cache.Activity.Enabled {
		return source, func() error { return nil }
	}
	if logger == nil {
		logger = slog.Default()
	}

	activityCfg := normalizeActivityCacheConfig(cfg.Cache.Activity)

	var redisClient *goredis.Client
	var client *commonredis.Client
	if cfg.RedisAddr != "" {
		client = commonredis.New(cfg.RedisAddr, commonredis.WithPassword(cfg.RedisPassword), commonredis.WithDB(cfg.RedisDB))
		if err := client.Redis().Ping(ctx).Err(); err != nil {
			logger.Warn("activity redis cache unavailable, using local cache only", "addr", cfg.RedisAddr, "error", err)
			if closeErr := client.Close(); closeErr != nil {
				logger.Warn("activity redis cache close failed", "error", closeErr)
			}
			client = nil
		} else {
			redisClient = client.Redis()
		}
	}

	cacheInstance, err := twocache.New[string, json.RawMessage](
		twocache.WithKeyEncoder[string, json.RawMessage](identityEncoder),
		twocache.WithRedisClient[string, json.RawMessage](redisClient),
		twocache.WithRedisTTL[string, json.RawMessage](activityCfg.RedisTTL),
		twocache.WithLocalTTL[string, json.RawMessage](activityCfg.LocalTTL),
		twocache.WithNullTTL[string, json.RawMessage](activityCfg.NullTTL),
		twocache.WithShards[string, json.RawMessage](32),
		twocache.WithMaxEntries[string, json.RawMessage](activityCfg.MaxSize),
	)
	if err != nil {
		logger.Error("failed to create activity twocache, falling back to source", "error", err)
		return source, func() error { return nil }
	}

	gateway := &CachedActivityGateway{
		source: source,
		cache:  cacheInstance,
		client: client,
		logger: logger,
	}
	return gateway, gateway.Close
}

func identityEncoder(key string) string { return key }

// Close 释放缓存资源。
func (g *CachedActivityGateway) Close() error {
	if g.cache != nil {
		_ = g.cache.Close()
	}
	if g.client == nil {
		return nil
	}
	if err := g.client.Close(); err != nil {
		return fmt.Errorf("close cache client: %w", err)
	}
	return nil
}

// EvictActivity 失效活动相关缓存。
func (g *CachedActivityGateway) EvictActivity(ctx context.Context, activityNo string) error {
	keys := []string{
		activeActivityListKey,
		activityInfoKeyPrefix + activityNo,
		activityProductKeyPrefix + activityNo,
	}
	for _, key := range keys {
		if err := g.cache.Delete(ctx, key); err != nil {
			g.logger.Warn("activity cache evict failed", "key", key, "error", err)
		}
	}
	return nil
}

// ListActivities 返回活动列表。
func (g *CachedActivityGateway) ListActivities(ctx context.Context) ([]domain.Activity, error) {
	raw, err := g.cache.GetOrLoad(ctx, activeActivityListKey, func(ctx context.Context, key string) (json.RawMessage, error) {
		activities, err := g.source.ListActivities(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(activities)
	})
	if err != nil {
		return nil, err
	}
	if isNull(raw) {
		return nil, repository.ErrNotFound
	}
	var activities []domain.Activity
	if err := json.Unmarshal(raw, &activities); err != nil {
		return nil, fmt.Errorf("unmarshal activity list: %w", err)
	}
	return activities, nil
}

// GetActivity 获取活动详情，并回填商品列表缓存。
func (g *CachedActivityGateway) GetActivity(ctx context.Context, activityNo string) (domain.Activity, error) {
	infoKey := activityInfoKeyPrefix + activityNo
	raw, err := g.cache.GetOrLoad(ctx, infoKey, func(ctx context.Context, key string) (json.RawMessage, error) {
		activity, err := g.source.GetActivity(ctx, activityNo)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("get activity from gateway: %w", err)
		}
		if len(activity.SKUs) > 0 {
			g.setProductList(ctx, activityNo, activity.SKUs)
		}
		return json.Marshal(activity)
	})
	if err != nil {
		return domain.Activity{}, err
	}
	if isNull(raw) {
		return domain.Activity{}, repository.ErrNotFound
	}
	var activity domain.Activity
	if err := json.Unmarshal(raw, &activity); err != nil {
		return domain.Activity{}, fmt.Errorf("unmarshal activity: %w", err)
	}
	return activity, nil
}

// GetSKU 从商品列表缓存中获取指定 SKU。
func (g *CachedActivityGateway) GetSKU(ctx context.Context, activityNo, skuNo string) (domain.SKU, error) {
	productKey := activityProductKeyPrefix + activityNo
	raw, err := g.cache.GetOrLoad(ctx, productKey, func(ctx context.Context, key string) (json.RawMessage, error) {
		activity, err := g.source.GetActivity(ctx, activityNo)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("get activity from gateway: %w", err)
		}
		if len(activity.SKUs) == 0 {
			return nil, nil
		}
		return json.Marshal(activity.SKUs)
	})
	if err != nil {
		return domain.SKU{}, err
	}
	if isNull(raw) {
		return domain.SKU{}, repository.ErrNotFound
	}
	var skus []domain.SKU
	if err := json.Unmarshal(raw, &skus); err != nil {
		return domain.SKU{}, fmt.Errorf("unmarshal sku list: %w", err)
	}
	for _, sku := range skus {
		if sku.SKUNo == skuNo {
			return sku, nil
		}
	}
	return domain.SKU{}, repository.ErrNotFound
}

func (g *CachedActivityGateway) setProductList(ctx context.Context, activityNo string, skus []domain.SKU) {
	if len(skus) == 0 {
		return
	}
	productKey := activityProductKeyPrefix + activityNo
	if err := g.cache.Set(ctx, productKey, mustMarshal(skus)); err != nil {
		g.logger.Warn("activity product cache set failed", "activityNo", activityNo, "error", err)
	}
}

func isNull(raw json.RawMessage) bool {
	return len(raw) == 0
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

func normalizeActivityCacheConfig(cfg config.ActivityCacheConfig) config.ActivityCacheConfig {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 512
	}
	if cfg.LocalTTL <= 0 {
		cfg.LocalTTL = 30 * time.Minute
	}
	if cfg.RedisTTL <= 0 {
		cfg.RedisTTL = 30 * time.Minute
	}
	if cfg.NullTTL <= 0 {
		cfg.NullTTL = time.Minute
	}
	if cfg.WarmupAhead <= 0 {
		cfg.WarmupAhead = 10 * time.Minute
	}
	if cfg.RefreshInitial < 0 {
		cfg.RefreshInitial = 0
	}
	if cfg.RefreshTick <= 0 {
		cfg.RefreshTick = time.Minute
	}
	return cfg
}
