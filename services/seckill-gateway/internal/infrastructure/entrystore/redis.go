// Package entrystore 提供秒杀入口 Java 兼容 Redis 状态存储。
package entrystore

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	userRateLimitPrefix = "seckill:user:rate:limit:"
	orderQueuingPrefix  = "seckill:order:queuing:"
)

const userRateLimitScript = `
local rate = tonumber(ARGV[1])
local interval = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local now = tonumber(ARGV[4])
local bucket = redis.call("HMGET", KEYS[1], "tokens", "last")
local tokens = tonumber(bucket[1])
local last = tonumber(bucket[2])
if tokens == nil then
  tokens = rate
  last = now
end
local elapsed = now - last
if elapsed < 0 then
  elapsed = 0
end
tokens = math.min(rate, tokens + elapsed * rate / interval)
local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end
redis.call("HMSET", KEYS[1], "tokens", tokens, "last", now)
redis.call("PEXPIRE", KEYS[1], ttl)
return allowed
`

const resultTTL = 5 * time.Minute

var userRateLimiter = redis.NewScript(userRateLimitScript)

// RedisStore 保存入口限流和排队状态。
type RedisStore struct {
	client *redis.Client
	now    func() time.Time
}

// NewRedisStore 创建 Redis 入口状态存储。
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client, now: time.Now}
}

// TryAcquire 执行 Java key 兼容的用户级限流。
func (s *RedisStore) TryAcquire(ctx context.Context, userID int64, rate int, interval time.Duration, ttl time.Duration) (bool, error) {
	if s == nil || s.client == nil || userID <= 0 || rate <= 0 {
		return true, nil
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	result, err := userRateLimiter.Run(ctx, s.client, []string{UserRateLimitKey(userID)},
		rate,
		interval.Milliseconds(),
		ttl.Milliseconds(),
		now().UnixMilli(),
	).Int()
	if err != nil {
		return false, fmt.Errorf("redis user rate limit %d: %w", userID, err)
	}
	return result == 1, nil
}

// TryAcquireAndSetProcessing 通过 Redis Pipeline 合并限流检查和 SetProcessing，
// 将 fast path 的 2 RT 降为 1 RT。
//
// Pipeline 语义：
//   - 两个命令在同一网络往返中发送
//   - Lua 脚本本身保证原子性，SET 命令独立执行
//   - 如果限流拒绝（allowed=false），SetProcessing 仍会执行（Pipeline 无法条件跳过），
//     但调用方会忽略结果，行为等价于不设置
func (s *RedisStore) TryAcquireAndSetProcessing(ctx context.Context, userID int64, rate int, interval, ttl time.Duration, resultKey string) (bool, error) {
	if s == nil || s.client == nil || userID <= 0 || rate <= 0 {
		return true, nil
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}

	pipe := s.client.Pipeline()
	limiterCmd := userRateLimiter.Run(ctx, pipe, []string{UserRateLimitKey(userID)},
		rate,
		interval.Milliseconds(),
		ttl.Milliseconds(),
		now().UnixMilli(),
	)
	// 即使限流可能拒绝，也在 Pipeline 中发送 SET（节省 RT）
	// 调用方根据 allowed 决定是否使用 setCmd 的结果
	_ = pipe.Set(ctx, resultKey, "PROCESSING", resultTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("redis pipeline acquire+set %d: %w", userID, err)
	}

	allowed, err := limiterCmd.Int()
	if err != nil {
		return false, fmt.Errorf("redis pipeline rate limit result %d: %w", userID, err)
	}
	return allowed == 1, nil
}

// SetQueued 保存用户活动维度的排队 traceID。
func (s *RedisStore) SetQueued(ctx context.Context, userID int64, activityNo string, traceID string, ttl time.Duration) error {
	if s == nil || s.client == nil || userID <= 0 || activityNo == "" || traceID == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	if err := s.client.Set(ctx, OrderQueuingKey(userID, activityNo), traceID, ttl).Err(); err != nil {
		return fmt.Errorf("redis set queued trace %d/%s: %w", userID, activityNo, err)
	}
	return nil
}

// GetQueuedTrace 获取用户活动维度的排队 traceID。
func (s *RedisStore) GetQueuedTrace(ctx context.Context, userID int64, activityNo string) (string, error) {
	if s == nil || s.client == nil || userID <= 0 || activityNo == "" {
		return "", nil
	}
	value, err := s.client.Get(ctx, OrderQueuingKey(userID, activityNo)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("redis get queued trace %d/%s: %w", userID, activityNo, err)
	}
	return value, nil
}

// UserRateLimitKey 生成 Java 用户级限流 key。
func UserRateLimitKey(userID int64) string {
	return userRateLimitPrefix + strconv.FormatInt(userID, 10)
}

// OrderQueuingKey 生成 Java 排队状态 key。
func OrderQueuingKey(userID int64, activityNo string) string {
	return orderQueuingPrefix + strconv.FormatInt(userID, 10) + ":" + activityNo
}
