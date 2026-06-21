// Package traceresult 提供链路追踪结果存储
// 用于记录秒杀请求的执行结果
package traceresult

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	commonerrors "seckill-common/errors"
)

const (
	// Processing 处理中状态
	Processing = commonerrors.TraceProcessing
	keyPrefix  = "seckill:order:result:"
)

// RedisStore Redis 追踪结果存储
type RedisStore struct {
	client *goredis.Client
}

// NewRedisStore 创建 Redis 追踪结果存储实例
func NewRedisStore(client *goredis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// TryStart 尝试标记追踪为处理中状态
// 使用 SetNX 实现幂等性，返回是否成功设置
func (s *RedisStore) TryStart(ctx context.Context, traceID string, ttl time.Duration) (bool, error) {
	ok, err := s.client.SetNX(ctx, Key(traceID), Processing, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("set trace processing %s: %w", traceID, err)
	}
	return ok, nil
}

// MarkSuccess 标记追踪为成功状态
// 将订单号写入 Redis
func (s *RedisStore) MarkSuccess(ctx context.Context, traceID, orderNo string, ttl time.Duration) error {
	if err := s.client.Set(ctx, Key(traceID), orderNo, ttl).Err(); err != nil {
		return fmt.Errorf("set trace success %s: %w", traceID, err)
	}
	return nil
}

// MarkFail 标记追踪为失败状态
// 将失败原因写入 Redis
func (s *RedisStore) MarkFail(ctx context.Context, traceID, reason string, ttl time.Duration) error {
	if err := s.client.Set(ctx, Key(traceID), reason, ttl).Err(); err != nil {
		return fmt.Errorf("set trace fail %s: %w", traceID, err)
	}
	return nil
}

// Delete 删除追踪结果
func (s *RedisStore) Delete(ctx context.Context, traceID string) error {
	if traceID == "" {
		return nil
	}
	script := `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end return 0`
	if err := s.client.Eval(ctx, script, []string{Key(traceID)}, Processing).Err(); err != nil {
		return fmt.Errorf("delete trace result %s: %w", traceID, err)
	}
	return nil
}

// Get 获取追踪结果
// 返回订单号、失败原因或空字符串
func (s *RedisStore) Get(ctx context.Context, traceID string) (string, error) {
	value, err := s.client.Get(ctx, Key(traceID)).Result()
	if err == nil {
		return value, nil
	}
	if err == goredis.Nil {
		return "", nil
	}
	return "", fmt.Errorf("get trace result %s: %w", traceID, err)
}

// Key 生成追踪结果的 Redis key
func Key(traceID string) string {
	return keyPrefix + traceID
}
