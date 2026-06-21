// Package persistence 提供基于 Redis 的仓储实现，用于高并发库存操作。
package persistence

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	commonredis "github.com/Martindeeepdark/go-common/cache/redis"
	goredis "github.com/redis/go-redis/v9"

	"seckill-common/rediskeys"
	"seckill-common/traceresult"

	domain "seckill-activity-service/internal/domain/entity"
)

// RedisStore 使用 Redis 管理库存和链路追踪，其他操作回退到内存存储。
type RedisStore struct {
	*MemoryStore
	cache *commonredis.Client
}

// NewRedisStore 创建 Redis 仓储实例。
func NewRedisStore(ctx context.Context, addr, password string, db int, memory *MemoryStore) (*RedisStore, error) {
	cache := commonredis.New(addr, commonredis.WithPassword(password), commonredis.WithDB(db))
	if err := cache.Redis().Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis get activity: %w", err)
	}
	return &RedisStore{MemoryStore: memory, cache: cache}, nil
}

// AddActivity 写入活动元数据到内存仓储，并为每个 SKU 通过 SetNX 初始化 Redis 库存键。
func (s *RedisStore) AddActivity(ctx context.Context, activity domain.Activity) error {
	if err := s.MemoryStore.AddActivity(ctx, activity); err != nil {
		return err
	}
	for _, sku := range activity.SKUs {
		key := redisStockKey(activity.ActivityNo, sku.SKUNo)
		if err := s.cache.Redis().SetNX(ctx, key, sku.TotalStock, 0).Err(); err != nil {
			return fmt.Errorf("redis delete activity: %w", err)
		}
	}
	return nil
}

// UpdateActivity 委托内存仓储更新活动基本信息。
func (s *RedisStore) UpdateActivity(ctx context.Context, activity domain.Activity) error {
	return s.MemoryStore.UpdateActivity(ctx, activity)
}

// UpdateActivityStatus 委托内存仓储更新活动状态。
func (s *RedisStore) UpdateActivityStatus(ctx context.Context, activityNo string, status int64) error {
	return s.MemoryStore.UpdateActivityStatus(ctx, activityNo, status)
}

// AddActivitySKU 向内存仓储追加 SKU，并通过 SetNX 初始化 Redis 库存键。
func (s *RedisStore) AddActivitySKU(ctx context.Context, activityNo string, sku domain.SKU) error {
	if err := s.MemoryStore.AddActivitySKU(ctx, activityNo, sku); err != nil {
		return err
	}
	if err := s.cache.Redis().SetNX(ctx, redisStockKey(activityNo, sku.SKUNo), sku.TotalStock, 0).Err(); err != nil {
		return fmt.Errorf("redis setnx stock: %w", err)
	}
	return nil
}

// RemoveActivitySKU 从内存仓储移除 SKU，并删除主/遗留 Redis 库存键。
func (s *RedisStore) RemoveActivitySKU(ctx context.Context, activityNo, skuNo string) error {
	if err := s.MemoryStore.RemoveActivitySKU(ctx, activityNo, skuNo); err != nil {
		return err
	}
	return s.cache.Redis().Del(ctx, redisStockKey(activityNo, skuNo), legacyRedisStockKey(activityNo, skuNo)).Err()
}

// PeekStock 读取 SKU 当前可用库存，Redis 未命中时回退到内存仓储。
func (s *RedisStore) PeekStock(ctx context.Context, activityNo, skuNo string) (int64, error) {
	value, err := s.stockValue(ctx, activityNo, skuNo)
	if errors.Is(err, goredis.Nil) {
		return s.MemoryStore.PeekStock(ctx, activityNo, skuNo)
	}
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse stock value: %w", err)
	}
	return v, nil
}

// DeductStockWithLimit 原子扣减 Redis 库存，purchaseLimit>0 时同时校验并占用用户限购额度。
func (s *RedisStore) DeductStockWithLimit(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, purchaseLimit int64) (bool, error) {
	if err := s.ensureStockKey(ctx, activityNo, skuNo); err != nil {
		return false, err
	}
	if purchaseLimit > 0 {
		purchaseKey := redisUserPurchaseKey(userID, activityNo, skuNo)
		if _, err := s.cache.CheckLimit(ctx, purchaseKey, int64(quantity), int64(purchaseLimit)); err != nil {
			return false, fmt.Errorf("check purchase limit: %w", err)
		}
		if _, err := s.cache.DeductStock(ctx, redisStockKey(activityNo, skuNo), int64(quantity)); err != nil {
			if rollbackErr := s.cache.RollbackLimit(ctx, purchaseKey, int64(quantity)); rollbackErr != nil {
				return false, fmt.Errorf("deduct stock: %w; rollback limit: %v", err, rollbackErr)
			}
			return false, fmt.Errorf("deduct stock: %w", err)
		}
		if err := s.cache.Redis().Expire(ctx, purchaseKey, 24*time.Hour).Err(); err != nil {
			return false, err
		}
		return true, nil
	}
	if _, err := s.cache.DeductStock(ctx, redisStockKey(activityNo, skuNo), int64(quantity)); err != nil {
		return false, fmt.Errorf("deduct stock: %w", err)
	}
	return true, nil
}

// ReleaseStock 通过 Redis 事务回滚库存，并同步归还用户限购额度。
func (s *RedisStore) ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64) error {
	if err := s.ensureStockKey(ctx, activityNo, skuNo); err != nil {
		return err
	}
	pipe := s.cache.Redis().TxPipeline()
	pipe.IncrBy(ctx, redisStockKey(activityNo, skuNo), int64(quantity))
	pipe.IncrBy(ctx, redisUserPurchaseKey(userID, activityNo, skuNo), int64(-quantity))
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis pipeline set stocks: %w", err)
	}
	return nil
}

// CleanupActivityStock 删除活动的库存键，skuNos 为空时按 pattern 扫描清理。
func (s *RedisStore) CleanupActivityStock(ctx context.Context, activityNo string, skuNos []string) (int64, error) {
	_, _ = s.MemoryStore.CleanupActivityStock(ctx, activityNo, skuNos) //nolint:errcheck // best-effort cleanup
	deleted := int64(0)
	if len(skuNos) > 0 {
		keys := make([]string, 0, len(skuNos)*2)
		for _, skuNo := range skuNos {
			keys = append(keys, redisStockKey(activityNo, skuNo))
			keys = append(keys, legacyRedisStockKey(activityNo, skuNo))
		}
		count, err := s.cache.Redis().Del(ctx, keys...).Result()
		if err != nil {
			return deleted, fmt.Errorf("redis delete stocks: %w", err)
		}
		deleted += count
	} else {
		for _, pattern := range []string{redisStockPattern(activityNo), legacyRedisStockPattern(activityNo)} {
			count, err := s.deleteKeysByPattern(ctx, pattern)
			if err != nil {
				return deleted, fmt.Errorf("redis delete stocks: %w", err)
			}
			deleted += count
		}
	}
	return deleted, nil
}

// CleanupActivityPurchases 按 pattern 扫描并删除活动的用户购买记录键。
func (s *RedisStore) CleanupActivityPurchases(ctx context.Context, activityNo string) (int64, error) {
	_, _ = s.MemoryStore.CleanupActivityPurchases(ctx, activityNo) //nolint:errcheck // best-effort cleanup
	deleted := int64(0)
	for _, pattern := range []string{redisUserPurchasePattern(activityNo), legacyRedisUserPurchasePattern(activityNo)} {
		count, err := s.deleteKeysByPattern(ctx, pattern)
		if err != nil {
			return deleted, fmt.Errorf("redis delete trace results: %w", err)
		}
		deleted += count
	}
	return deleted, nil
}

// TryStartTrace 用 SetNX 占用 trace 链路，返回是否首次抢占成功。
func (s *RedisStore) TryStartTrace(ctx context.Context, traceID string, ttl time.Duration) (bool, error) {
	result, err := s.cache.Redis().SetNX(ctx, redisTraceResultKey(traceID), TraceProcessing, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis setnx trace result: %w", err)
	}
	return result, nil
}

// MarkTraceSuccess 将 trace 结果写为订单号，表示链路处理成功。
func (s *RedisStore) MarkTraceSuccess(ctx context.Context, traceID, orderNo string, ttl time.Duration) error {
	return s.cache.Redis().Set(ctx, redisTraceResultKey(traceID), orderNo, ttl).Err()
}

// MarkTraceFail 将 trace 结果写为失败原因。
func (s *RedisStore) MarkTraceFail(ctx context.Context, traceID, reason string, ttl time.Duration) error {
	return s.cache.Redis().Set(ctx, redisTraceResultKey(traceID), reason, ttl).Err()
}

// GetTraceResult 读取 trace 结果，未命中返回 ErrNotFound。
func (s *RedisStore) GetTraceResult(ctx context.Context, traceID string) (string, error) {
	value, err := s.cache.Redis().Get(ctx, redisTraceResultKey(traceID)).Result()
	if errors.Is(err, goredis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("redis get trace result: %w", err)
	}
	return value, nil
}

// DeleteTrace 删除 trace 结果键。
func (s *RedisStore) DeleteTrace(ctx context.Context, traceID string) error {
	if err := s.cache.Redis().Del(ctx, redisTraceResultKey(traceID)).Err(); err != nil {
		return fmt.Errorf("redis delete trace result key: %w", err)
	}
	return nil
}

func (s *RedisStore) deleteKeysByPattern(ctx context.Context, pattern string) (int64, error) {
	var cursor uint64
	deleted := int64(0)
	for {
		keys, next, err := s.cache.Redis().Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return deleted, fmt.Errorf("redis delete trace results: %w", err)
		}
		if len(keys) > 0 {
			count, err := s.cache.Redis().Del(ctx, keys...).Result()
			if err != nil {
				return deleted, fmt.Errorf("redis delete trace result key: %w", err)
			}
			deleted += count
		}
		cursor = next
		if cursor == 0 {
			return deleted, nil
		}
	}
}

func redisStockKey(activityNo, skuNo string) string {
	return rediskeys.ProductSKUStock(activityNo, skuNo)
}

func legacyRedisStockKey(activityNo, skuNo string) string {
	return rediskeys.LegacyProductSKUStock(activityNo, skuNo)
}

func redisStockPattern(activityNo string) string {
	return rediskeys.ProductSKUStockPattern(activityNo)
}

func legacyRedisStockPattern(activityNo string) string {
	return rediskeys.LegacyProductSKUStockPattern(activityNo)
}

func redisUserPurchaseKey(userID int64, activityNo, skuNo string) string {
	return rediskeys.UserPurchaseLimit(userID, activityNo, skuNo)
}

func redisUserPurchasePattern(activityNo string) string {
	return rediskeys.UserPurchaseLimitPattern(activityNo)
}

func legacyRedisUserPurchasePattern(activityNo string) string {
	return rediskeys.LegacyUserPurchaseLimitPattern(activityNo)
}

func redisTraceResultKey(traceID string) string {
	return traceresult.Key(traceID)
}

func (s *RedisStore) ensureStockKey(ctx context.Context, activityNo, skuNo string) error {
	_, err := s.stockValue(ctx, activityNo, skuNo)
	if errors.Is(err, goredis.Nil) {
		return nil
	}
	return err
}

func (s *RedisStore) stockValue(ctx context.Context, activityNo, skuNo string) (string, error) {
	primary := redisStockKey(activityNo, skuNo)
	value, err := s.cache.Redis().Get(ctx, primary).Result()
	if !errors.Is(err, goredis.Nil) {
		if err != nil {
			return value, fmt.Errorf("redis get sku stock: %w", err)
		}
		return value, nil
	}

	legacy := legacyRedisStockKey(activityNo, skuNo)
	value, err = s.cache.Redis().Get(ctx, legacy).Result()
	if err != nil {
		return "", fmt.Errorf("redis get sku stock: %w", err)
	}
	return value, s.copyLegacyStock(ctx, primary, legacy, value)
}

func (s *RedisStore) copyLegacyStock(ctx context.Context, primary, legacy, value string) error {
	ttl, err := s.cache.Redis().TTL(ctx, legacy).Result()
	if err != nil || ttl < 0 {
		ttl = 0
	}
	return s.cache.Redis().SetNX(ctx, primary, value, ttl).Err()
}
