// Package queue 提供消息队列的多种实现
// 支持 NATS JetStream、Redis Stream 和内存队列
package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"seckill-processor-service/internal/domain/model"
)

// MemoryMessageQueue 内存消息队列实现
// 仅用于测试，不支持持久化和集群
type MemoryMessageQueue struct {
	messages chan model.SeckillMessage
	mu       sync.Mutex
	closed   bool
	logger   *slog.Logger
}

// NewMemoryMessageQueue 创建内存消息队列
func NewMemoryMessageQueue(size int, logger *slog.Logger) *MemoryMessageQueue {
	if size <= 0 {
		size = 1024
	}
	return &MemoryMessageQueue{
		messages: make(chan model.SeckillMessage, size),
		logger:   logger,
	}
}

// Publish 发布秒杀消息
func (q *MemoryMessageQueue) Publish(_ context.Context, message model.SeckillMessage) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	select {
	case q.messages <- message:
	default:
		// 队列已满，丢弃消息
		if q.logger != nil {
			q.logger.Warn("memory queue full, dropping message", "traceId", message.TraceID)
		}
	}
	return nil
}

// Consume 消费秒杀消息
func (q *MemoryMessageQueue) Consume(ctx context.Context, _ string, _ string, handler func(context.Context, model.SeckillMessage) error) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context done: %w", ctx.Err())
		case msg, ok := <-q.messages:
			if !ok {
				return nil
			}
			if err := handler(ctx, msg); err != nil && q.logger != nil {
					q.logger.Warn("memory queue handler error", "error", err)
				}
		}
	}
}

// Close 关闭队列
func (q *MemoryMessageQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.messages)
	}
	return nil
}
