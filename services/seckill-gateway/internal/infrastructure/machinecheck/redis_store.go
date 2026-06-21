// Package machinecheck 提供机审令牌存储实现。
package machinecheck

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore 使用 Redis 保存一次性机审校验值。
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore 创建 Redis 机审存储。
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// Set 写入带 TTL 的校验值。
func (s *RedisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if err := s.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %s: %w", key, err)
	}
	return nil
}

// Get 读取校验值，key 不存在时返回空字符串。
func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("redis get %s: %w", key, err)
	}
	return value, nil
}

// Delete 删除校验值。
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis del %s: %w", key, err)
	}
	return nil
}
