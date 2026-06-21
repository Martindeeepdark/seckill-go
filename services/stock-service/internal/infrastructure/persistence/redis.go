package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	commonredis "github.com/Martindeeepdark/go-common/cache/redis"
	goredis "github.com/redis/go-redis/v9"

	"seckill-common/rediskeys"
)

// RedisStore Redis库存存储实现，内嵌内存存储作为后备
type RedisStore struct {
	*MemoryStore
	cache *commonredis.Client
}

// NewRedisStore 创建Redis库存存储
func NewRedisStore(ctx context.Context, addr, password string, db int, memory *MemoryStore) (*RedisStore, error) {
	cache := commonredis.New(addr, commonredis.WithPassword(password), commonredis.WithDB(db))
	if err := cache.Redis().Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisStore{MemoryStore: memory, cache: cache}, nil
}

// PeekStock 查询库存（先查Redis，回退到内存）
func (s *RedisStore) PeekStock(ctx context.Context, activityNo, skuNo string) (int64, error) {
	value, err := s.stockValue(ctx, activityNo, skuNo)
	if errors.Is(err, goredis.Nil) {
		return s.MemoryStore.PeekStock(ctx, activityNo, skuNo)
	}
	if err != nil {
		return 0, fmt.Errorf("redis get stock: %w", err)
	}
	return parseInt64(value)
}

// DeductStockWithLimit 扣减库存（支持购买限制）
func (s *RedisStore) DeductStockWithLimit(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, purchaseLimit int64, orderNo string) (bool, error) {
	if err := s.ensureStockKey(ctx, activityNo, skuNo); err != nil {
		return false, err
	}
	stockKey := redisStockKey(activityNo, skuNo)
	reservedKey := redisReservedKey(activityNo, skuNo)

	if purchaseLimit > 0 {
		purchaseKey := redisUserPurchaseKey(userID, activityNo, skuNo)
		// 检查购买限制
		if _, err := s.cache.CheckLimit(ctx, purchaseKey, quantity, purchaseLimit); err != nil {
			if isLimitExceeded(err) {
				return false, nil
			}
			return false, fmt.Errorf("check purchase limit: %w", err)
		}
		// 扣减库存
		if _, err := s.cache.DeductStockOrder(ctx, stockKey, reservedKey, orderNo, quantity); err != nil {
			// 失败回滚限制检查
			if rollbackErr := s.cache.RollbackLimit(ctx, purchaseKey, quantity); rollbackErr != nil {
				return false, fmt.Errorf("deduct stock: %w; rollback limit: %v", err, rollbackErr)
			}
			if isStockRejected(err) {
				return false, nil
			}
			return false, fmt.Errorf("deduct stock: %w", err)
		}
		// 设置过期时间
		if err := s.cache.Redis().Expire(ctx, purchaseKey, 24*time.Hour).Err(); err != nil {
			return false, fmt.Errorf("redis expire: %w", err)
		}
		return true, nil
	}
	// 无购买限制，直接扣减
	if _, err := s.cache.DeductStockOrder(ctx, stockKey, reservedKey, orderNo, quantity); err != nil {
		if isStockRejected(err) {
			return false, nil
		}
		return false, fmt.Errorf("deduct stock: %w", err)
	}
	return true, nil
}

// ReleaseStock 释放库存
func (s *RedisStore) ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error {
	if err := s.ensureStockKey(ctx, activityNo, skuNo); err != nil {
		return err
	}
	stockKey := redisStockKey(activityNo, skuNo)
	reservedKey := redisReservedKey(activityNo, skuNo)
	if _, err := s.cache.ReleaseStockOrder(ctx, stockKey, reservedKey, orderNo, quantity); err != nil {
		return fmt.Errorf("release stock order: %w", err)
	}
	// 减少用户购买数量
	if orderNo != "" {
		s.cache.Redis().IncrBy(ctx, redisUserPurchaseKey(userID, activityNo, skuNo), -quantity)
	}
	return nil
}

// CleanupActivityStock 清理活动库存
func (s *RedisStore) CleanupActivityStock(ctx context.Context, activityNo string, skuNos []string) (int64, error) {
	// 先清理内存存储
	_, _ = s.MemoryStore.CleanupActivityStock(ctx, activityNo, skuNos) //nolint:errcheck // best-effort cleanup
	deleted := int64(0)
	if len(skuNos) > 0 {
		// 按指定SKU清理
		keys := make([]string, 0, len(skuNos)*2)
		for _, skuNo := range skuNos {
			keys = append(keys, redisStockKey(activityNo, skuNo))
			keys = append(keys, legacyRedisStockKey(activityNo, skuNo))
		}
		count, err := s.cache.Redis().Del(ctx, keys...).Result()
		if err != nil {
			return deleted, fmt.Errorf("delete stock keys: %w", err)
		}
		deleted += count
	} else {
		// 按模式清理
		for _, pattern := range []string{redisStockPattern(activityNo), legacyRedisStockPattern(activityNo)} {
			count, err := s.deleteKeysByPattern(ctx, pattern)
			if err != nil {
				return deleted, fmt.Errorf("delete stock keys: %w", err)
			}
			deleted += count
		}
	}
	return deleted, nil
}

// CleanupActivityPurchases 清理活动购买记录
func (s *RedisStore) CleanupActivityPurchases(ctx context.Context, activityNo string) (int64, error) {
	_, _ = s.MemoryStore.CleanupActivityPurchases(ctx, activityNo) //nolint:errcheck // best-effort cleanup
	deleted := int64(0)
	for _, pattern := range []string{redisUserPurchasePattern(activityNo), legacyRedisUserPurchasePattern(activityNo)} {
		count, err := s.deleteKeysByPattern(ctx, pattern)
		if err != nil {
			return deleted, fmt.Errorf("delete purchase keys: %w", err)
		}
		deleted += count
	}
	return deleted, nil
}

// deleteKeysByPattern 按模式删除Redis键
func (s *RedisStore) deleteKeysByPattern(ctx context.Context, pattern string) (int64, error) {
	var cursor uint64
	deleted := int64(0)
	for {
		keys, next, err := s.cache.Redis().Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return deleted, fmt.Errorf("scan stock keys: %w", err)
		}
		if len(keys) > 0 {
			count, err := s.cache.Redis().Del(ctx, keys...).Result()
			if err != nil {
				return deleted, fmt.Errorf("del stock keys: %w", err)
			}
			deleted += count
		}
		cursor = next
		if cursor == 0 {
			return deleted, nil
		}
	}
}

// redisStockKey 生成Redis库存键
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

// redisUserPurchaseKey 生成Redis用户购买记录键
func redisUserPurchaseKey(userID int64, activityNo, skuNo string) string {
	return rediskeys.UserPurchaseLimit(userID, activityNo, skuNo)
}

func redisUserPurchasePattern(activityNo string) string {
	return rediskeys.UserPurchaseLimitPattern(activityNo)
}

func legacyRedisUserPurchasePattern(activityNo string) string {
	return rediskeys.LegacyUserPurchaseLimitPattern(activityNo)
}

// redisReservedKey 生成Redis预留键
func redisReservedKey(activityNo, skuNo string) string {
	return fmt.Sprintf("seckill:reserved:%s:%s", activityNo, skuNo)
}

// parseInt64 解析字符串为int64
func parseInt64(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

func isLimitExceeded(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "check_limit") && strings.Contains(msg, "would exceed limit")
}

func isStockRejected(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "deduct_stock_order") &&
		(strings.Contains(msg, "insufficient stock") || strings.Contains(msg, "key not found"))
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
	if err == nil {
		return value, nil
	}
	if !errors.Is(err, goredis.Nil) {
		return "", fmt.Errorf("redis get stock: %w", err)
	}

	legacy := legacyRedisStockKey(activityNo, skuNo)
	value, err = s.cache.Redis().Get(ctx, legacy).Result()
	if err != nil {
		return "", fmt.Errorf("redis get stock: %w", err)
	}
	return value, s.copyLegacyStock(ctx, primary, legacy, value)
}

func (s *RedisStore) copyLegacyStock(ctx context.Context, primary, legacy, value string) error {
	ttl, err := s.cache.Redis().TTL(ctx, legacy).Result()
	if err != nil || ttl < 0 {
		ttl = 0
	}
	return fmt.Errorf("redis setnx stock key: %w", s.cache.Redis().SetNX(ctx, primary, value, ttl).Err())
}
