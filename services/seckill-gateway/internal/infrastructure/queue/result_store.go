// Package queue 提供消息队列发布器实现
// 支持 NATS 和 Redis 作为消息队列后端
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"seckill-common/traceresult"

	"seckill-gateway-service/internal/application"
)

// ErrResultNotFound 表示 Redis 中未找到对应秒杀结果。
var ErrResultNotFound = errors.New("result not found")

const (
	legacyResultKeyPrefix = "seckill:trace:"
	resultTTL             = 5 * time.Minute
)

// 失败原因映射
var failReasons = map[string]bool{
	"ACTIVITY_CLOSED": true,
	"RISK_USER":       true,
	"STOCK_EMPTY":     true,
	"ORDER_FAIL":      true,
	"PROCESS_ERROR":   true,
	"PUBLISH_FAILED":  true,
}

// RedisResultStore 从 Redis 读取异步秒杀结果
// 桥接处理器的纯字符串协议和 gateway 的 SeckillResult 类型
type RedisResultStore struct {
	client *redis.Client
}

// NewRedisResultStore 创建由 Redis 支持的结果存储
func NewRedisResultStore(client *redis.Client) *RedisResultStore {
	return &RedisResultStore{client: client}
}

// SetProcessing 设置处理中状态
func (s *RedisResultStore) SetProcessing(ctx context.Context, requestID string) error {
	if err := s.client.Set(ctx, resultKey(requestID), "PROCESSING", resultTTL).Err(); err != nil {
		return fmt.Errorf("set processing result: %w", err)
	}
	return nil
}

// SetSuccess 设置成功状态
func (s *RedisResultStore) SetSuccess(ctx context.Context, requestID, orderNo string) error {
	if err := s.client.Set(ctx, resultKey(requestID), orderNo, resultTTL).Err(); err != nil {
		return fmt.Errorf("set result order: %w", err)
	}
	return nil
}

// SetFailed 设置失败状态
func (s *RedisResultStore) SetFailed(ctx context.Context, requestID, errMsg string) error {
	if err := s.client.Set(ctx, resultKey(requestID), errMsg, resultTTL).Err(); err != nil {
		return fmt.Errorf("set result error: %w", err)
	}
	return nil
}

// Get 获取秒杀结果
func (s *RedisResultStore) Get(ctx context.Context, requestID string) (*application.SeckillResult, error) {
	val, err := s.client.Get(ctx, resultKey(requestID)).Result()
	if err == redis.Nil {
		val, err = s.client.Get(ctx, legacyResultKey(requestID)).Result()
		if err == redis.Nil {
			return nil, ErrResultNotFound
		}
	}
	if err != nil {
		return nil, fmt.Errorf("redis get result: %w", err)
	}
	// 先尝试 JSON 格式
	if strings.HasPrefix(val, "{") {
		var result application.SeckillResult
		if err := json.Unmarshal([]byte(val), &result); err == nil {
			return &result, nil
		}
	}
	// 处理器的纯字符串协议
	switch val {
	case "PROCESSING":
		return &application.SeckillResult{Status: "processing"}, nil
	case "":
		return &application.SeckillResult{Status: "processing"}, nil
	}
	if failReasons[val] {
		return &application.SeckillResult{Status: "failed", Error: val}, nil
	}
	// 否则是订单号 = 成功
	return &application.SeckillResult{Status: "success", OrderNo: val}, nil
}

func resultKey(requestID string) string {
	return traceresult.Key(requestID)
}

func legacyResultKey(requestID string) string {
	return legacyResultKeyPrefix + requestID
}
