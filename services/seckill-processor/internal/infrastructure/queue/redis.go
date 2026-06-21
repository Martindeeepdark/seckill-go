// Package queue 提供消息队列的多种实现
// 支持 NATS JetStream、Redis Stream 和内存队列
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"

	commonerrors "seckill-common/errors"

	"seckill-processor-service/internal/domain/model"
)

const seckillQueueKey = "seckill:queue:messages"

// ==================== Redis 秒杀消息队列 ====================

// RedisMessageQueue 使用 Redis Stream 实现的秒杀消息队列
type RedisMessageQueue struct {
	client *goredis.Client
	logger *slog.Logger
}

// NewRedisMessageQueue 创建 Redis 秒杀消息队列
func NewRedisMessageQueue(client *goredis.Client, logger *slog.Logger) *RedisMessageQueue {
	return &RedisMessageQueue{client: client, logger: logger}
}

// Publish 发布秒杀消息
func (q *RedisMessageQueue) Publish(ctx context.Context, message model.SeckillMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal seckill message: %w", err)
	}
	if err := q.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: seckillQueueKey,
		Values: map[string]interface{}{"data": data},
	}).Err(); err != nil {
		return fmt.Errorf("publish redis seckill message: %w", err)
	}
	return nil
}

// Consume 消费秒杀消息
func (q *RedisMessageQueue) Consume(ctx context.Context, group string, consumer string, handler func(context.Context, model.SeckillMessage) error) error {
	// 创建消费组（如果不存在）
	if err := q.client.XGroupCreateMkStream(ctx, seckillQueueKey, group, "0").Err(); err != nil && q.logger != nil {
		q.logger.Debug("redis seckill consumer group create skipped or failed", "group", group, "error", err)
	}

	// 先恢复 pending 列表中的未确认消息（上次消费失败或崩溃遗留的）
	q.recoverPending(ctx, group, consumer, handler)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("seckill consumer context cancelled: %w", ctx.Err())
		default:
		}

		streams, err := q.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{seckillQueueKey, ">"},
			Count:    1,
			Block:    0,
		}).Result()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if q.logger != nil {
				q.logger.Warn("redis xreadgroup error", "error", err)
			}
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				q.handleMessage(ctx, group, msg, handler)
			}
		}
	}
}

// recoverPending 处理 pending 列表中未确认的消息
// 使用 XReadGroup 配合 ID "0" 读取该消费者尚未 ACK 的消息
func (q *RedisMessageQueue) recoverPending(ctx context.Context, group string, consumer string, handler func(context.Context, model.SeckillMessage) error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		streams, err := q.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{seckillQueueKey, "0"},
			Count:    10,
		}).Result()
		if err != nil {
			if q.logger != nil {
				q.logger.Warn("redis xreadgroup pending error", "error", err)
			}
			return
		}

		total := 0
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				q.handleMessage(ctx, group, msg, handler)
				total++
			}
		}
		// 没有待恢复的消息，退出
		if total == 0 {
			return
		}
	}
}

// handleMessage 处理单条消息，统一处理 JSON 解析、终端错误判断和确认逻辑
func (q *RedisMessageQueue) handleMessage(ctx context.Context, group string, msg goredis.XMessage, handler func(context.Context, model.SeckillMessage) error) {
	var message model.SeckillMessage
	if data, ok := msg.Values["data"].(string); ok {
		if err := json.Unmarshal([]byte(data), &message); err != nil {
			if q.logger != nil {
				q.logger.Warn("unmarshal seckill message failed", "error", err)
			}
			q.ack(ctx, group, msg.ID, "ack malformed seckill message")
			return
		}
	}

	if err := handler(ctx, message); err != nil {
		if commonerrors.IsTerminal(err) {
			// 终端错误，确认消息不再重试
			if q.logger != nil {
				q.logger.Warn("redis seckill handler terminal error, acking", "traceId", message.TraceID, "messageId", msg.ID, "error", err)
			}
			q.ack(ctx, group, msg.ID, "ack terminal seckill message")
			return
		}
		// 非终端错误，消息保留在 pending 列表中等待重新处理
		if q.logger != nil {
			q.logger.Warn("redis seckill handler failed, leaving message pending", "traceId", message.TraceID, "messageId", msg.ID, "error", err)
		}
		return
	}
	// 处理成功，确认消息
	q.ack(ctx, group, msg.ID, "ack seckill message")
}

// Close 关闭队列（无操作）
func (q *RedisMessageQueue) Close() error {
	return nil
}

// ack 确认消息
func (q *RedisMessageQueue) ack(ctx context.Context, group string, messageID string, action string) {
	if err := q.client.XAck(ctx, seckillQueueKey, group, messageID).Err(); err != nil && q.logger != nil {
		q.logger.Warn(action+" failed", "messageId", messageID, "error", err)
	}
}
