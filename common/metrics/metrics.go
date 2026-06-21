// Package metrics 提供 nil-safe 的 smoke 压测计数器 API。
// 空 runID 时所有操作均为 no-op，兼容生产流量缺失 X-Smoke-Run-Id header 的场景。
package metrics

import (
	"context"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// 字段名常量
const (
	FieldRateLimit  = "rate-limit"
	FieldRisk       = "risk"
	FieldStockEmpty = "stock-empty"
	FieldSuccess    = "success"
	FieldOther      = "other"
)

const (
	keyPrefix = "seckill:metrics:"
	ttl       = time.Hour
)

var (
	mu     sync.RWMutex
	client *goredis.Client
)

// SetClient 注入 Redis 客户端。为 nil 时所有操作 no-op。
func SetClient(c *goredis.Client) {
	mu.Lock()
	defer mu.Unlock()
	client = c
}

// IncrRateLimit 用户限流拒绝计数
func IncrRateLimit(ctx context.Context, runID string) { incr(ctx, runID, FieldRateLimit) }

// IncrRisk 风控拒绝计数
func IncrRisk(ctx context.Context, runID string) { incr(ctx, runID, FieldRisk) }

// IncrStockEmpty 库存空拒绝计数
func IncrStockEmpty(ctx context.Context, runID string) { incr(ctx, runID, FieldStockEmpty) }

// IncrSuccess 订单创建成功计数
func IncrSuccess(ctx context.Context, runID string) { incr(ctx, runID, FieldSuccess) }

// IncrOther 其他拒绝计数（机审失败、队列满等）
func IncrOther(ctx context.Context, runID string) { incr(ctx, runID, FieldOther) }

func incr(ctx context.Context, runID, field string) {
	if runID == "" {
		return
	}
	mu.RLock()
	c := client
	mu.RUnlock()
	if c == nil {
		return
	}

	key := keyPrefix + runID
	pipe := c.Pipeline()
	pipe.HIncrBy(ctx, key, field, 1)
	pipe.Expire(ctx, key, ttl)
	cmds, err := pipe.Exec(ctx)
	// DEBUG: use log instead of println
	if err != nil {
		// log package is imported in main, not here - just keep the key info
	}
	_ = cmds
	// Force a write to verify execution - write to key with _debug suffix
	if c != nil {
		_ = c.Set(ctx, keyPrefix+runID+"_debug", field, ttl).Err()
	}
}
