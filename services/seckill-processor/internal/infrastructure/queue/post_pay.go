// Package queue 提供消息队列的多种实现
// 支持 NATS JetStream、Redis Stream 和内存队列
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"

	"seckill-processor-service/internal/domain/model"
)

const postPayQueueKey = "seckill:queue:post_pay"

// ==================== Redis 支付后任务队列 ====================

// RedisPostPayQueue 使用 Redis Stream 实现的支付后任务队列
type RedisPostPayQueue struct {
	client *goredis.Client
	logger *slog.Logger
}

// NewRedisPostPayQueue 创建 Redis 支付后任务队列
func NewRedisPostPayQueue(client *goredis.Client, logger *slog.Logger) *RedisPostPayQueue {
	return &RedisPostPayQueue{client: client, logger: logger}
}

// ConsumePostPayTasks 消费支付后任务
func (q *RedisPostPayQueue) ConsumePostPayTasks(ctx context.Context, consumer string, handler func(context.Context, model.PostPayTask) error) error {
	group := "seckill-postpay"
	if err := q.client.XGroupCreateMkStream(ctx, postPayQueueKey, group, "0").Err(); err != nil && q.logger != nil {
		q.logger.Warn("create consumer group", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("post pay context cancelled: %w", ctx.Err())
		default:
		}

		streams, err := q.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{postPayQueueKey, ">"},
			Count:    1,
			Block:    0,
		}).Result()
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("post pay context cancelled: %w", ctx.Err())
			}
			if q.logger != nil {
				q.logger.Warn("redis xreadgroup post-pay error", "error", err)
			}
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				var task model.PostPayTask
				if data, ok := msg.Values["data"].(string); ok {
					if err := json.Unmarshal([]byte(data), &task); err != nil {
						if q.logger != nil {
							q.logger.Warn("unmarshal post-pay task failed", "error", err)
						}
						// 解析失败，确认消息移除
						_ = q.client.XAck(ctx, postPayQueueKey, group, msg.ID).Err() //nolint:errcheck // ack is best-effort
						continue
					}
				}
				if err := handler(ctx, task); err != nil {
					if q.logger != nil {
						q.logger.Warn("post-pay handler failed, skipping ack", "error", err, "msgID", msg.ID)
					}
					// 处理失败，消息保留在 pending 列表
					continue
				}
				// 处理成功，确认消息
				_ = q.client.XAck(ctx, postPayQueueKey, group, msg.ID).Err() //nolint:errcheck // ack is best-effort
			}
		}
	}
}

// PublishPostPayTask 发布支付后任务（供外部使用的辅助方法）
func (q *RedisPostPayQueue) PublishPostPayTask(ctx context.Context, task model.PostPayTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal post-pay task: %w", err)
	}
	if err := q.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: postPayQueueKey,
		Values: map[string]interface{}{"data": data},
	}).Err(); err != nil {
		return fmt.Errorf("publish post-pay task: %w", err)
	}
	return nil
}

// Close 关闭队列（无操作）
func (q *RedisPostPayQueue) Close() error {
	return nil
}
