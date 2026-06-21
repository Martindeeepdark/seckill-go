// Package stockmap 提供库存幂等订单号映射存储。
package stockmap

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const keyPrefix = "seckill:stock:order:map:"

// RedisStore 使用 Redis 保存真实订单号到库存幂等订单号的映射。
type RedisStore struct {
	client *goredis.Client
}

// NewRedisStore 创建 Redis 映射存储。
func NewRedisStore(client *goredis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// Save 保存订单号映射。
func (s *RedisStore) Save(ctx context.Context, orderNo, stockOrderNo string, ttl time.Duration) error {
	if orderNo == "" || stockOrderNo == "" {
		return nil
	}
	if err := s.client.Set(ctx, Key(orderNo), stockOrderNo, ttl).Err(); err != nil {
		return fmt.Errorf("redis set stock order mapping %s: %w", orderNo, err)
	}
	return nil
}

// Resolve 查询库存幂等订单号，映射不存在时返回空字符串。
func (s *RedisStore) Resolve(ctx context.Context, orderNo string) (string, error) {
	if orderNo == "" {
		return "", nil
	}
	value, err := s.client.Get(ctx, Key(orderNo)).Result()
	if err == goredis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("redis get stock order mapping %s: %w", orderNo, err)
	}
	return value, nil
}

// Key 生成映射 Redis key。
func Key(orderNo string) string {
	return keyPrefix + orderNo
}
