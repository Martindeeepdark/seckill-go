// Package cache 用 local+redis 缓存包装 processor 的 gateway 客户端。
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Martindeeepdark/go-common/cache/twocache"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"seckill-processor-service/internal/domain/model"
	"seckill-processor-service/internal/domain/service"
)

const (
	detailKeyPrefix = "processor:activity:detail:"
	skuKeyPrefix    = "processor:activity:sku:"
)

var _ service.ActivityQuery = (*ActivityCache)(nil)

// ActivityCache 使用 local+redis 两级缓存包装活动查询，通过 singleflight 抑制并发回源。
type ActivityCache struct {
	source      service.ActivityQuery
	detailCache *twocache.TwoLevelCache[string, *model.ActivityInfo]
	skuCache    *twocache.TwoLevelCache[string, *model.SKUInfo]
	sfGroup     singleflight.Group
	logger      *slog.Logger
}

// EntryConfig 描述单层 (detail 或 sku) 缓存的 BigCache 参数。
type EntryConfig struct {
	LocalTTL   time.Duration
	Shards     int
	MaxEntries int
}

// Config 描述 ActivityCache 的构造参数。
type Config struct {
	Detail EntryConfig
	SKU    EntryConfig
}

// DefaultConfig 返回一组保守的默认缓存参数，用于测试或未配置场景。
func DefaultConfig() Config {
	return Config{
		Detail: EntryConfig{
			LocalTTL:   2 * time.Minute,
			Shards:     16,
			MaxEntries: 1024,
		},
		SKU: EntryConfig{
			LocalTTL:   2 * time.Minute,
			Shards:     32,
			MaxEntries: 4096,
		},
	}
}

func withDefaults(cfg Config) Config {
	d := DefaultConfig()
	if cfg.Detail.LocalTTL <= 0 {
		cfg.Detail.LocalTTL = d.Detail.LocalTTL
	}
	if cfg.Detail.Shards <= 0 {
		cfg.Detail.Shards = d.Detail.Shards
	}
	if cfg.Detail.MaxEntries <= 0 {
		cfg.Detail.MaxEntries = d.Detail.MaxEntries
	}
	if cfg.SKU.LocalTTL <= 0 {
		cfg.SKU.LocalTTL = d.SKU.LocalTTL
	}
	if cfg.SKU.Shards <= 0 {
		cfg.SKU.Shards = d.SKU.Shards
	}
	if cfg.SKU.MaxEntries <= 0 {
		cfg.SKU.MaxEntries = d.SKU.MaxEntries
	}
	return cfg
}

// NewActivityCache 创建活动两级缓存实例，source 为缓存未命中时的回源查询。
func NewActivityCache(source service.ActivityQuery, rdb *goredis.Client, cfg Config, logger *slog.Logger) (*ActivityCache, error) {
	if source == nil {
		return nil, fmt.Errorf("activity cache: source is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	cfg = withDefaults(cfg)

	detailOpts := []twocache.Option[string, *model.ActivityInfo]{
		twocache.WithKeyEncoder[string, *model.ActivityInfo](func(key string) string { return detailKeyPrefix + key }),
		twocache.WithLocalTTL[string, *model.ActivityInfo](cfg.Detail.LocalTTL),
		twocache.WithShards[string, *model.ActivityInfo](cfg.Detail.Shards),
		twocache.WithMaxEntries[string, *model.ActivityInfo](cfg.Detail.MaxEntries),
	}
	if rdb != nil {
		detailOpts = append(detailOpts, twocache.WithRedisClient[string, *model.ActivityInfo](rdb))
	}
	detailCache, err := twocache.New(detailOpts...)
	if err != nil {
		return nil, fmt.Errorf("create activity detail cache: %w", err)
	}

	skuOpts := []twocache.Option[string, *model.SKUInfo]{
		twocache.WithKeyEncoder[string, *model.SKUInfo](func(key string) string { return skuKeyPrefix + key }),
		twocache.WithLocalTTL[string, *model.SKUInfo](cfg.SKU.LocalTTL),
		twocache.WithShards[string, *model.SKUInfo](cfg.SKU.Shards),
		twocache.WithMaxEntries[string, *model.SKUInfo](cfg.SKU.MaxEntries),
	}
	if rdb != nil {
		skuOpts = append(skuOpts, twocache.WithRedisClient[string, *model.SKUInfo](rdb))
	}
	skuCache, err := twocache.New(skuOpts...)
	if err != nil {
		_ = detailCache.Close()
		return nil, fmt.Errorf("create activity sku cache: %w", err)
	}

	return &ActivityCache{
		source:      source,
		detailCache: detailCache,
		skuCache:    skuCache,
		logger:      logger,
	}, nil
}

// GetActivity 按活动编号查询活动信息，命中缓存则直接返回，否则回源并写入两级缓存。
func (c *ActivityCache) GetActivity(ctx context.Context, activityNo string) (model.ActivityInfo, error) {
	v, err, _ := c.sfGroup.Do("activity:"+activityNo, func() (any, error) {
		ptr, err := c.detailCache.GetOrLoad(ctx, activityNo, func(ctx context.Context, key string) (*model.ActivityInfo, error) {
			info, err := c.source.GetActivity(ctx, activityNo)
			if err != nil {
				return nil, err
			}
			return &info, nil
		})
		if err != nil {
			return nil, err
		}
		return ptr, nil
	})
	if err != nil {
		return model.ActivityInfo{}, err
	}
	if v == nil {
		return model.ActivityInfo{}, nil
	}
	ptr, ok := v.(*model.ActivityInfo)
	if !ok || ptr == nil {
		return model.ActivityInfo{}, nil
	}
	return *ptr, nil
}

// GetSKU 按活动和 SKU 编号查询 SKU 信息，命中缓存则直接返回，否则回源并写入两级缓存。
func (c *ActivityCache) GetSKU(ctx context.Context, activityNo, skuNo string) (model.SKUInfo, error) {
	key := activityNo + ":" + skuNo
	v, err, _ := c.sfGroup.Do("sku:"+key, func() (any, error) {
		ptr, err := c.skuCache.GetOrLoad(ctx, key, func(ctx context.Context, cacheKey string) (*model.SKUInfo, error) {
			info, err := c.source.GetSKU(ctx, activityNo, skuNo)
			if err != nil {
				return nil, err
			}
			return &info, nil
		})
		if err != nil {
			return nil, err
		}
		return ptr, nil
	})
	if err != nil {
		return model.SKUInfo{}, err
	}
	if v == nil {
		return model.SKUInfo{}, nil
	}
	ptr, ok := v.(*model.SKUInfo)
	if !ok || ptr == nil {
		return model.SKUInfo{}, nil
	}
	return *ptr, nil
}

// Close 关闭并释放底层两级缓存资源。
func (c *ActivityCache) Close() error {
	var firstErr error
	if err := c.detailCache.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := c.skuCache.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
