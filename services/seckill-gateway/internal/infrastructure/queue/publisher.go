// Package queue 提供消息队列发布器实现
// 支持 NATS 和 Redis 作为消息队列后端
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"seckill-gateway-service/internal/application"
)

const seckillQueueKey = "seckill:queue:messages"

// seckillOrderMessage 秒杀订单消息格式
type seckillOrderMessage struct {
	TraceID        string `json:"traceId"`
	RequestTraceID string `json:"requestTraceId"`
	ActivityNo     string `json:"activityNo"`
	SKUNo          string `json:"skuNo"`
	UserID         int64  `json:"userId"`
	Quantity       int    `json:"quantity"`
	TotalFee       int64  `json:"totalFee"`
	RequestIP      string `json:"requestIp,omitempty"`
	RunID          string `json:"runId,omitempty"` // smoke 压测 run-id
	MachinePass    bool   `json:"machinePass,omitempty"`
}

// RedisPublisher 将 PartIn 事件发布到 Redis 列表
type RedisPublisher struct {
	client *redis.Client
	logger *slog.Logger
}

// NewRedisPublisher 创建由 Redis 支持的队列发布器
func NewRedisPublisher(addr, password string, db int, logger *slog.Logger) (*RedisPublisher, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisPublisher{client: client, logger: logger}, nil
}

// Publish 将 PartInEvent 推送到 Redis 流
func (p *RedisPublisher) Publish(ctx context.Context, event application.PartInEvent) error {
	data, err := json.Marshal(toMessage(event))
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: seckillQueueKey,
		Values: map[string]any{"data": data},
	}).Err(); err != nil {
		return fmt.Errorf("xadd event: %w", err)
	}
	p.logger.Info("part-in event published to redis stream", "userId", event.UserID, "activityNo", event.ActivityNo)
	return nil
}

// Close 关闭 Redis 客户端
func (p *RedisPublisher) Close() error {
	if err := p.client.Close(); err != nil {
		return fmt.Errorf("close redis client: %w", err)
	}
	return nil
}

// NopPublisher 是空操作的队列发布器，用于禁用异步处理时
type NopPublisher struct{}

// Publish NopPublisher 丢弃所有事件，直接返回成功。
func (n *NopPublisher) Publish(_ context.Context, _ application.PartInEvent) error { return nil }

// toMessage 将 PartInEvent 转换为消息格式
func toMessage(event application.PartInEvent) seckillOrderMessage {
	return seckillOrderMessage{
		TraceID:        event.TraceID,
		RequestTraceID: event.TraceID,
		ActivityNo:     event.ActivityNo,
		SKUNo:          event.SkuNo,
		UserID:         event.UserID,
		Quantity:       event.Quantity,
		TotalFee:       event.TotalFee,
		RequestIP:      event.RequestIP,
		RunID:          event.RunID,
		MachinePass:    event.MachinePass,
	}
}
