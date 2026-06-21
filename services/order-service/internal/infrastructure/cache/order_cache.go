// Package cache wraps OrderStore with a local BigCache layer for hot order reads.
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Martindeeepdark/go-common/cache/twocache"

	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/domain/repository"
)

const (
	orderDetailKeyPrefix = "order:detail:"
)

var _ repository.OrderStore = (*OrderCache)(nil)

// OrderCache decorates an OrderStore with a local-only BigCache layer.
// Only GetOrder is cached; writes invalidate the cached entry.
// Local-only (no Redis L2): the underlying RedisStore already caches in Redis,
// so adding a Redis L2 here would create redundant double-writes.
type OrderCache struct {
	source repository.OrderStore
	cache  *twocache.TwoLevelCache[string, *entity.Order]
	logger *slog.Logger
}

// EntryConfig 描述单层缓存的 BigCache 参数。
// 定义在 cache 包内，避免 cache 包反向依赖 config 包。
type EntryConfig struct {
	LocalTTL   time.Duration
	Shards     int
	MaxEntries int
}

// Config 描述 OrderCache 的构造参数。
type Config struct {
	Order EntryConfig
}

// DefaultConfig 返回一组保守的默认缓存参数，用于测试或未配置场景。
func DefaultConfig() Config {
	return Config{
		Order: EntryConfig{
			LocalTTL:   3 * time.Minute,
			Shards:     16,
			MaxEntries: 1024,
		},
	}
}

func withDefaults(cfg Config) Config {
	d := DefaultConfig()
	if cfg.Order.LocalTTL <= 0 {
		cfg.Order.LocalTTL = d.Order.LocalTTL
	}
	if cfg.Order.Shards <= 0 {
		cfg.Order.Shards = d.Order.Shards
	}
	if cfg.Order.MaxEntries <= 0 {
		cfg.Order.MaxEntries = d.Order.MaxEntries
	}
	return cfg
}

// NewOrderCache builds an OrderCache with the given BigCache params.
func NewOrderCache(source repository.OrderStore, cfg Config, logger *slog.Logger) (*OrderCache, error) {
	if source == nil {
		return nil, fmt.Errorf("order cache: source is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	cfg = withDefaults(cfg)
	c, err := twocache.New[string, *entity.Order](
		twocache.WithKeyEncoder[string, *entity.Order](func(key string) string {
			return orderDetailKeyPrefix + key
		}),
		twocache.WithLocalTTL[string, *entity.Order](cfg.Order.LocalTTL),
		twocache.WithShards[string, *entity.Order](cfg.Order.Shards),
		twocache.WithMaxEntries[string, *entity.Order](cfg.Order.MaxEntries),
	)
	if err != nil {
		return nil, fmt.Errorf("create order detail cache: %w", err)
	}
	return &OrderCache{source: source, cache: c, logger: logger}, nil
}

// GetOrder returns the order for orderNo, serving from local cache on hit.
func (c *OrderCache) GetOrder(ctx context.Context, orderNo string) (entity.Order, error) {
	ptr, err := c.cache.GetOrLoad(ctx, orderNo, func(ctx context.Context, key string) (*entity.Order, error) {
		order, err := c.source.GetOrder(ctx, orderNo)
		if err != nil {
			return nil, err
		}
		return &order, nil
	})
	if err != nil {
		return entity.Order{}, err
	}
	if ptr == nil {
		return entity.Order{}, nil
	}
	return *ptr, nil
}

// CreateOrder delegates to the source and invalidates any stale cached entry.
func (c *OrderCache) CreateOrder(ctx context.Context, order entity.Order) error {
	if err := c.source.CreateOrder(ctx, order); err != nil {
		return err
	}
	c.invalidate(ctx, order.OrderNo)
	return nil
}

// MarkOrderPaid delegates to the source and invalidates the cached entry.
func (c *OrderCache) MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error {
	if err := c.source.MarkOrderPaid(ctx, orderNo, transactionNo, paidAt); err != nil {
		return err
	}
	c.invalidate(ctx, orderNo)
	return nil
}

// CloseOrder delegates to the source and invalidates the cached entry.
func (c *OrderCache) CloseOrder(ctx context.Context, orderNo string) error {
	if err := c.source.CloseOrder(ctx, orderNo); err != nil {
		return err
	}
	c.invalidate(ctx, orderNo)
	return nil
}

// ListOrdersByActivity is pass-through (list queries are not cached).
func (c *OrderCache) ListOrdersByActivity(ctx context.Context, activityNo string) ([]entity.Order, error) {
	return c.source.ListOrdersByActivity(ctx, activityNo)
}

// ListOrdersByActivities is pass-through (list queries are not cached).
func (c *OrderCache) ListOrdersByActivities(ctx context.Context, activityNos []string) (map[string][]entity.Order, error) {
	return c.source.ListOrdersByActivities(ctx, activityNos)
}

// ListOrdersByUser is pass-through (list queries are not cached).
func (c *OrderCache) ListOrdersByUser(ctx context.Context, userID int64) ([]entity.Order, error) {
	return c.source.ListOrdersByUser(ctx, userID)
}

// GetByUserAndTrace is pass-through (DuplicateKey fallback lookup, rare; not cached).
func (c *OrderCache) GetByUserAndTrace(ctx context.Context, userID int64, traceID string) (entity.Order, error) {
	return c.source.GetByUserAndTrace(ctx, userID, traceID)
}

// Close releases the underlying BigCache resources.
func (c *OrderCache) Close() error {
	if c.cache == nil {
		return nil
	}
	return c.cache.Close()
}

func (c *OrderCache) invalidate(ctx context.Context, orderNo string) {
	if err := c.cache.Delete(ctx, orderNo); err != nil {
		c.logger.Warn("order cache invalidate failed", "orderNo", orderNo, "error", err)
	}
}
