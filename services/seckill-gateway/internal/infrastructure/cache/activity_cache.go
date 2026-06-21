// Package cache 基于 twocache (BigCache) 为 gateway 数据提供本地缓存。
// ActivityCache 包装 ActivityGateway，提供 BigCache 本地缓存、
// singleflight 去重和后台刷新。
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Martindeeepdark/go-common/cache/twocache"
	"golang.org/x/sync/singleflight"

	"seckill-gateway-service/internal/application"
)

const (
	gwListKey      = "gateway:activity:list"
	gwDetailPrefix = "gateway:activity:detail:"
)

// 编译期检查 ActivityCache 是否实现了 ActivityGateway。
var _ application.ActivityGateway = (*ActivityCache)(nil)

// ActivityCache 使用 BigCache（经由 twocache）缓存活动数据，
// 并提供 singleflight 保护和后台刷新。
type ActivityCache struct {
	source      application.ActivityGateway
	sfGroup     singleflight.Group
	detailCache *twocache.TwoLevelCache[string, *application.ActivityDetail]
	listCache   *twocache.TwoLevelCache[string, *application.ActivityList]

	refreshInterval time.Duration
	stopCh          chan struct{}
	logger          *slog.Logger
}

// EntryConfig 描述单层 (detail 或 list) 缓存的 BigCache 参数。
// 定义在 cache 包内，避免 cache 包反向依赖 config 包。
type EntryConfig struct {
	LocalTTL   time.Duration
	Shards     int
	MaxEntries int
}

// Config 描述 ActivityCache 的构造参数。
type Config struct {
	RefreshInterval time.Duration
	Detail          EntryConfig
	List            EntryConfig
}

// DefaultConfig 返回一组保守的默认缓存参数，用于测试或未配置场景。
func DefaultConfig() Config {
	return Config{
		RefreshInterval: 30 * time.Second,
		Detail: EntryConfig{
			LocalTTL:   5 * time.Minute,
			Shards:     16,
			MaxEntries: 256,
		},
		List: EntryConfig{
			LocalTTL:   5 * time.Minute,
			Shards:     4,
			MaxEntries: 4,
		},
	}
}

// NewActivityCache 创建一个包装指定数据源的 ActivityCache。
// 调用 Start 开始后台刷新。
func NewActivityCache(source application.ActivityGateway, cfg Config) *ActivityCache {
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = DefaultConfig().RefreshInterval
	}
	if cfg.Detail.LocalTTL <= 0 {
		cfg.Detail.LocalTTL = DefaultConfig().Detail.LocalTTL
	}
	if cfg.Detail.Shards <= 0 {
		cfg.Detail.Shards = DefaultConfig().Detail.Shards
	}
	if cfg.Detail.MaxEntries <= 0 {
		cfg.Detail.MaxEntries = DefaultConfig().Detail.MaxEntries
	}
	if cfg.List.LocalTTL <= 0 {
		cfg.List.LocalTTL = DefaultConfig().List.LocalTTL
	}
	if cfg.List.Shards <= 0 {
		cfg.List.Shards = DefaultConfig().List.Shards
	}
	if cfg.List.MaxEntries <= 0 {
		cfg.List.MaxEntries = DefaultConfig().List.MaxEntries
	}

	detailCache, err := twocache.New[string, *application.ActivityDetail](
		twocache.WithKeyEncoder[string, *application.ActivityDetail](func(key string) string { return gwDetailPrefix + key }),
		twocache.WithLocalTTL[string, *application.ActivityDetail](cfg.Detail.LocalTTL),
		twocache.WithShards[string, *application.ActivityDetail](cfg.Detail.Shards),
		twocache.WithMaxEntries[string, *application.ActivityDetail](cfg.Detail.MaxEntries),
	)
	if err != nil {
		slog.Default().Error("failed to create detail twocache", "error", err)
	}

	listCache, err := twocache.New[string, *application.ActivityList](
		twocache.WithKeyEncoder[string, *application.ActivityList](func(key string) string { return gwListKey }),
		twocache.WithLocalTTL[string, *application.ActivityList](cfg.List.LocalTTL),
		twocache.WithShards[string, *application.ActivityList](cfg.List.Shards),
		twocache.WithMaxEntries[string, *application.ActivityList](cfg.List.MaxEntries),
	)
	if err != nil {
		slog.Default().Error("failed to create list twocache", "error", err)
	}

	return &ActivityCache{
		source:          source,
		detailCache:     detailCache,
		listCache:       listCache,
		refreshInterval: cfg.RefreshInterval,
		stopCh:          make(chan struct{}),
		logger:          slog.Default(),
	}
}

// Start 启动后台刷新循环。
func (c *ActivityCache) Start(ctx context.Context) {
	c.refresh(ctx)

	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.refresh(ctx)
		}
	}
}

// Stop 通知后台刷新 goroutine 停止。
func (c *ActivityCache) Stop() {
	select {
	case c.stopCh <- struct{}{}:
	default:
	}
	if c.detailCache != nil {
		_ = c.detailCache.Close()
	}
	if c.listCache != nil {
		_ = c.listCache.Close()
	}
}

// refresh 执行全量刷新：先调用 ListActivities，再对每条记录调用
// GetActivity，以用完整详情填充缓存。
func (c *ActivityCache) refresh(ctx context.Context) {
	list, err := c.source.ListActivities(ctx)
	if err != nil {
		c.logger.Warn("activity cache refresh: ListActivities failed", "error", err)
		return
	}

	if c.listCache != nil {
		_ = c.listCache.Set(ctx, gwListKey, &list)
	}

	for _, item := range list.Activities {
		detail, err := c.source.GetActivity(ctx, item.ActivityNo)
		if err != nil {
			c.logger.Warn("activity cache refresh: GetActivity failed",
				"activityNo", item.ActivityNo, "error", err)
			continue
		}
		if detail != nil && c.detailCache != nil {
			_ = c.detailCache.Set(ctx, item.ActivityNo, detail)
		}
	}
}

// ListActivities 返回缓存的活动列表。缓存未命中时通过 singleflight 去重从数据源拉取。
func (c *ActivityCache) ListActivities(ctx context.Context) (application.ActivityList, error) {
	v, err, _ := c.sfGroup.Do("ListActivities", func() (any, error) {
		if c.listCache != nil {
			cached, err := c.listCache.GetOrLoad(ctx, gwListKey, func(ctx context.Context, key string) (*application.ActivityList, error) {
				list, err := c.source.ListActivities(ctx)
				if err != nil {
					return nil, err
				}
				return &list, nil
			})
			if err != nil {
				return application.ActivityList{}, err
			}
			if cached != nil {
				return *cached, nil
			}
		}
		list, err := c.source.ListActivities(ctx)
		if err != nil {
			return application.ActivityList{}, err
		}
		return list, nil
	})
	if err != nil {
		return application.ActivityList{}, err
	}
	return v.(application.ActivityList), nil
}

// GetActivity 返回缓存的活动详情。缓存未命中时通过 singleflight 去重从数据源拉取。
func (c *ActivityCache) GetActivity(ctx context.Context, activityNo string) (*application.ActivityDetail, error) {
	v, err, _ := c.sfGroup.Do("GetActivity:"+activityNo, func() (any, error) {
		if c.detailCache != nil {
			detail, err := c.detailCache.GetOrLoad(ctx, activityNo, func(ctx context.Context, key string) (*application.ActivityDetail, error) {
				d, err := c.source.GetActivity(ctx, activityNo)
				if err != nil {
					return nil, fmt.Errorf("get activity %s: %w", activityNo, err)
				}
				return d, nil
			})
			if err != nil {
				return nil, err
			}
			return detail, nil
		}
		d, err := c.source.GetActivity(ctx, activityNo)
		return d, err
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.(*application.ActivityDetail), nil
}

// CreateActivity 透传给数据源。
func (c *ActivityCache) CreateActivity(ctx context.Context, req application.CreateActivityRequest) (*application.ActivityDetail, error) {
	return c.source.CreateActivity(ctx, req)
}

// UpdateActivity 透传给数据源。
func (c *ActivityCache) UpdateActivity(ctx context.Context, req application.UpdateActivityRequest) error {
	return c.source.UpdateActivity(ctx, req)
}

// EndActivity 透传给数据源。
func (c *ActivityCache) EndActivity(ctx context.Context, activityNo string) error {
	return c.source.EndActivity(ctx, activityNo)
}

// AddProduct 透传给数据源。
func (c *ActivityCache) AddProduct(ctx context.Context, req application.AddProductRequest) error {
	return c.source.AddProduct(ctx, req)
}

// RemoveProduct 透传给数据源。
func (c *ActivityCache) RemoveProduct(ctx context.Context, activityNo, skuNo string) error {
	return c.source.RemoveProduct(ctx, activityNo, skuNo)
}
