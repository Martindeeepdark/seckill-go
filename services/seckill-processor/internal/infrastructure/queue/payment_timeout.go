// Package queue 提供消息队列的多种实现
// 支持 NATS JetStream、Redis Stream 和内存队列
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"seckill-processor-service/internal/domain/model"
)

const paymentTimeoutQueueKey = "seckill:queue:payment_timeout"

// ==================== Redis 支付超时任务队列 ====================

// RedisPaymentTimeoutQueue 使用 Redis Sorted Set 实现的支付超时任务队列
// 使用时间戳作为分数，实现按到期时间排序
type RedisPaymentTimeoutQueue struct {
	client *goredis.Client
	logger *slog.Logger
}

// NewRedisPaymentTimeoutQueue 创建 Redis 支付超时任务队列
func NewRedisPaymentTimeoutQueue(client *goredis.Client, logger *slog.Logger) *RedisPaymentTimeoutQueue {
	return &RedisPaymentTimeoutQueue{client: client, logger: logger}
}

// PublishPaymentTimeout 发布支付超时任务
func (q *RedisPaymentTimeoutQueue) PublishPaymentTimeout(ctx context.Context, task model.PaymentTimeoutTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal payment timeout task: %w", err)
	}
	// 使用到期时间戳作为分数
	if err := q.client.ZAdd(ctx, paymentTimeoutQueueKey, goredis.Z{
		Score:  float64(task.DueAt.Unix()),
		Member: data,
	}).Err(); err != nil {
		return fmt.Errorf("publish payment timeout: %w", err)
	}
	return nil
}

// ConsumePaymentTimeouts 消费支付超时任务
// 轮询查询已到期的任务
func (q *RedisPaymentTimeoutQueue) ConsumePaymentTimeouts(ctx context.Context, consumer string, handler func(context.Context, model.PaymentTimeoutTask) error) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context done: %w", ctx.Err())
		default:
		}

		// 查询分数小于等于当前时间的任务（已到期）
		results, err := q.client.ZRangeArgs(ctx, goredis.ZRangeArgs{
			Key:   paymentTimeoutQueueKey,
			Start: "-inf",
			Stop:  fmt.Sprintf("%d", currentUnix()),
			Count: 10,
			ByScore: true,
		}).Result()
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("context done: %w", ctx.Err())
			}
			continue
		}

		for _, result := range results {
			var task model.PaymentTimeoutTask
			if err := json.Unmarshal([]byte(result), &task); err != nil {
				if q.logger != nil {
					q.logger.Warn("unmarshal payment timeout task failed", "error", err)
				}
				// 解析失败，删除任务
				if err := q.client.ZRem(ctx, paymentTimeoutQueueKey, result).Err(); err != nil && q.logger != nil {
					q.logger.Warn("zrem malformed task failed", "error", err)
				}
				continue
			}
			if err := handler(ctx, task); err != nil {
				if q.logger != nil {
					q.logger.Warn("payment timeout handler error", "orderNo", task.OrderNo, "error", err)
				}
				// 处理失败，任务保留稍后重试
				continue
			}
			// 处理成功，删除任务
			if err := q.client.ZRem(ctx, paymentTimeoutQueueKey, result).Err(); err != nil && q.logger != nil {
				q.logger.Warn("zrem completed task failed", "error", err)
			}
		}

		// 短暂等待后再次轮询
		select {
		case <-ctx.Done():
			return fmt.Errorf("context done: %w", ctx.Err())
		default:
		}
	}
}

// Close 关闭队列（无操作）
func (q *RedisPaymentTimeoutQueue) Close() error {
	return nil
}

// currentUnix 获取当前 Unix 时间戳
func currentUnix() int64 {
	return time.Now().Unix()
}
