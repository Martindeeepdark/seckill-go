// Package queue 提供消息队列的多种实现
// 支持 NATS JetStream、Redis Stream 和内存队列
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	commonerrors "seckill-common/errors"
	"seckill-common/tracing"

	"seckill-processor-service/internal/config"
	"seckill-processor-service/internal/domain/model"
)

// ==================== NATS 秒杀消息队列 ====================

// NATSMessageQueue 使用 NATS JetStream 实现的秒杀消息队列
type NATSMessageQueue struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	stream  string
	subject string
	logger  *slog.Logger
}

// NewNATSMessageQueue 创建 NATS 秒杀消息队列
func NewNATSMessageQueue(cfg config.NATSConfig, logger *slog.Logger) (*NATSMessageQueue, error) {
	conn, err := connectNATS(cfg.URL, "seckill-processor")
	if err != nil {
		return nil, err
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create nats jetstream context: %w", err)
	}
	q := &NATSMessageQueue{
		conn:    conn,
		js:      js,
		stream:  cfg.Stream,
		subject: cfg.Subject,
		logger:  logger,
	}
	if err := q.ensureStream(); err != nil {
		conn.Close()
		return nil, err
	}
	return q, nil
}

// ensureStream 确保流和主题存在
func (q *NATSMessageQueue) ensureStream() error {
	return ensureStreamSubject(q.js, q.stream, q.subject)
}

// Publish 发布秒杀消息
func (q *NATSMessageQueue) Publish(ctx context.Context, message model.SeckillMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal seckill message: %w", err)
	}
	msg := &nats.Msg{
		Subject: q.subject,
		Header:  traceHeaders(ctx, firstNonEmpty(message.RequestTraceID, message.TraceID)),
		Data:    data,
	}
	if _, err := q.js.PublishMsg(msg, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish nats seckill message: %w", err)
	}
	return nil
}

// Consume 消费秒杀消息
func (q *NATSMessageQueue) Consume(ctx context.Context, group string, _ string, handler func(context.Context, model.SeckillMessage) error) error {
	if group == "" {
		group = "seckill-processor"
	}
	sub, err := q.js.PullSubscribe(q.subject, group, nats.BindStream(q.stream), nats.ManualAck())
	if err != nil {
		return fmt.Errorf("subscribe nats subject %s: %w", q.subject, err)
	}

	for {
		if ctx.Err() != nil {
			return fmt.Errorf("context done: %w", ctx.Err())
		}
		msgs, err := sub.Fetch(1, nats.MaxWait(time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("context done: %w", ctx.Err())
			}
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			if q.logger != nil {
				q.logger.Warn("nats fetch seckill message failed", "error", err)
			}
			continue
		}

		for _, msg := range msgs {
			var message model.SeckillMessage
			if err := json.Unmarshal(msg.Data, &message); err != nil {
				if q.logger != nil {
					q.logger.Warn("unmarshal nats seckill message failed", "error", err)
				}
				ackWithLog(q.logger, msg, "ack malformed seckill message")
				continue
			}
			msgCtx, traceID := contextFromNATSMessage(ctx, msg, message.RequestTraceID)
			if message.RequestTraceID == "" {
				message.RequestTraceID = traceID
			}
			if message.TraceID == "" {
				message.TraceID = traceID
			}
			if err := handler(msgCtx, message); err != nil {
				if commonerrors.IsTerminal(err) {
					// 终端错误，确认消息不再重试
					if q.logger != nil {
						q.logger.Warn("nats seckill handler terminal error, acking", "traceId", message.TraceID, "error", err)
					}
					ackWithLog(q.logger, msg, "ack terminal seckill message")
					continue
				}
				// 非终端错误，拒绝消息以便重试
				if q.logger != nil {
					q.logger.Warn("nats seckill handler failed", "traceId", message.TraceID, "error", err)
				}
				nakWithLog(q.logger, msg, "nak seckill message")
				continue
			}
			// 处理成功，确认消息
			ackWithLog(q.logger, msg, "ack seckill message")
		}
	}
}

// Close 关闭连接
func (q *NATSMessageQueue) Close() error {
	err := q.conn.Drain()
	q.conn.Close()
	if err != nil {
		return fmt.Errorf("drain nats connection: %w", err)
	}
	return nil
}

// IsHealthy 检查 NATS 连接是否健康
func (q *NATSMessageQueue) IsHealthy() bool {
	return q.conn != nil && q.conn.IsConnected()
}

// ==================== NATS 支付后任务队列 ====================

// NATSPostPayQueue 使用 NATS JetStream 实现的支付后任务队列
type NATSPostPayQueue struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	stream  string
	subject string
	logger  *slog.Logger
}

// NewNATSPostPayQueue 创建 NATS 支付后任务队列
func NewNATSPostPayQueue(cfg config.NATSConfig, logger *slog.Logger) (*NATSPostPayQueue, error) {
	conn, err := connectNATS(cfg.URL, "seckill-processor-post-pay")
	if err != nil {
		return nil, err
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create nats jetstream context: %w", err)
	}
	q := &NATSPostPayQueue{
		conn:    conn,
		js:      js,
		stream:  cfg.Stream,
		subject: cfg.PostPaySubject,
		logger:  logger,
	}
	if err := ensureStreamSubject(q.js, q.stream, q.subject); err != nil {
		conn.Close()
		return nil, err
	}
	return q, nil
}

// PublishPostPayTask 发布支付后任务
func (q *NATSPostPayQueue) PublishPostPayTask(ctx context.Context, task model.PostPayTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal post-pay task: %w", err)
	}
	msg := &nats.Msg{
		Subject: q.subject,
		Header:  traceHeaders(ctx, task.RequestTraceID),
		Data:    data,
	}
	if _, err := q.js.PublishMsg(msg, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish nats post-pay task: %w", err)
	}
	return nil
}

// ConsumePostPayTasks 消费支付后任务
func (q *NATSPostPayQueue) ConsumePostPayTasks(ctx context.Context, _ string, handler func(context.Context, model.PostPayTask) error) error {
	sub, err := q.js.PullSubscribe(q.subject, "seckill-postpay", nats.BindStream(q.stream), nats.ManualAck())
	if err != nil {
		return fmt.Errorf("subscribe nats post-pay subject %s: %w", q.subject, err)
	}
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("context done: %w", ctx.Err())
		}
		msgs, err := sub.Fetch(1, nats.MaxWait(time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("context done: %w", ctx.Err())
			}
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			if q.logger != nil {
				q.logger.Warn("nats fetch post-pay task failed", "error", err)
			}
			continue
		}
		for _, msg := range msgs {
			var task model.PostPayTask
			if err := json.Unmarshal(msg.Data, &task); err != nil {
				if q.logger != nil {
					q.logger.Warn("unmarshal nats post-pay task failed", "error", err)
				}
				ackWithLog(q.logger, msg, "ack malformed post-pay task")
				continue
			}
			msgCtx, traceID := contextFromNATSMessage(ctx, msg, task.RequestTraceID)
			if task.RequestTraceID == "" {
				task.RequestTraceID = traceID
			}
			if err := handler(msgCtx, task); err != nil {
				if commonerrors.IsTerminal(err) {
					if q.logger != nil {
						q.logger.Warn("nats post-pay handler terminal error, acking", "orderNo", task.OrderNo, "error", err)
					}
					ackWithLog(q.logger, msg, "ack terminal post-pay task")
					continue
				}
				if q.logger != nil {
					q.logger.Warn("nats post-pay handler failed", "orderNo", task.OrderNo, "error", err)
				}
				nakWithLog(q.logger, msg, "nak post-pay task")
				continue
			}
			ackWithLog(q.logger, msg, "ack post-pay task")
		}
	}
}

// Close 关闭连接
func (q *NATSPostPayQueue) Close() error {
	err := q.conn.Drain()
	q.conn.Close()
	if err != nil {
		return fmt.Errorf("drain nats connection: %w", err)
	}
	return nil
}

// IsHealthy 检查 NATS 连接是否健康
func (q *NATSPostPayQueue) IsHealthy() bool {
	return q.conn != nil && q.conn.IsConnected()
}

// ==================== NATS 支付超时任务队列 ====================

// NATSPaymentTimeoutQueue 使用 NATS JetStream 实现的支付超时任务队列
// 通过 NakWithDelay 实现延迟投递
type NATSPaymentTimeoutQueue struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	stream  string
	subject string
	logger  *slog.Logger
}

// NewNATSPaymentTimeoutQueue 创建 NATS 支付超时任务队列
func NewNATSPaymentTimeoutQueue(cfg config.NATSConfig, logger *slog.Logger) (*NATSPaymentTimeoutQueue, error) {
	conn, err := connectNATS(cfg.URL, "seckill-processor-payment-timeout")
	if err != nil {
		return nil, err
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create nats jetstream context: %w", err)
	}
	q := &NATSPaymentTimeoutQueue{
		conn:    conn,
		js:      js,
		stream:  cfg.Stream,
		subject: cfg.PaymentTimeoutSubject,
		logger:  logger,
	}
	if err := ensureStreamSubject(q.js, q.stream, q.subject); err != nil {
		conn.Close()
		return nil, err
	}
	return q, nil
}

// PublishPaymentTimeout 发布支付超时任务
func (q *NATSPaymentTimeoutQueue) PublishPaymentTimeout(ctx context.Context, task model.PaymentTimeoutTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal payment timeout task: %w", err)
	}
	msg := &nats.Msg{
		Subject: q.subject,
		Header:  traceHeaders(ctx, task.RequestTraceID),
		Data:    data,
	}
	if _, err := q.js.PublishMsg(msg, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish nats payment timeout task: %w", err)
	}
	return nil
}

// ConsumePaymentTimeouts 消费支付超时任务
// 通过 NakWithDelay 实现延迟处理
func (q *NATSPaymentTimeoutQueue) ConsumePaymentTimeouts(ctx context.Context, _ string, handler func(context.Context, model.PaymentTimeoutTask) error) error {
	sub, err := q.js.PullSubscribe(q.subject, "seckill-payment-timeout", nats.BindStream(q.stream), nats.ManualAck())
	if err != nil {
		return fmt.Errorf("subscribe nats payment-timeout subject %s: %w", q.subject, err)
	}
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("context done: %w", ctx.Err())
		}
		msgs, err := sub.Fetch(1, nats.MaxWait(time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("context done: %w", ctx.Err())
			}
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			if q.logger != nil {
				q.logger.Warn("nats fetch payment timeout task failed", "error", err)
			}
			continue
		}
		for _, msg := range msgs {
			var task model.PaymentTimeoutTask
			if err := json.Unmarshal(msg.Data, &task); err != nil {
				if q.logger != nil {
					q.logger.Warn("unmarshal nats payment timeout task failed", "error", err)
				}
				ackWithLog(q.logger, msg, "ack malformed payment timeout task")
				continue
			}
			// 检查是否到达到期时间，未到达则延迟重试
			if delay := time.Until(task.DueAt); delay > 0 {
				nakWithDelayLog(q.logger, msg, delay, "delay payment timeout task")
				continue
			}
			msgCtx, traceID := contextFromNATSMessage(ctx, msg, task.RequestTraceID)
			if task.RequestTraceID == "" {
				task.RequestTraceID = traceID
			}
			if err := handler(msgCtx, task); err != nil {
				if commonerrors.IsTerminal(err) {
					if q.logger != nil {
						q.logger.Warn("nats payment timeout handler terminal error, acking", "orderNo", task.OrderNo, "error", err)
					}
					ackWithLog(q.logger, msg, "ack terminal payment timeout task")
					continue
				}
				if q.logger != nil {
					q.logger.Warn("nats payment timeout handler failed", "orderNo", task.OrderNo, "error", err)
				}
				// 处理失败，1秒后重试
				nakWithDelayLog(q.logger, msg, time.Second, "retry payment timeout task")
				continue
			}
			// 处理成功，确认消息
			ackWithLog(q.logger, msg, "ack payment timeout task")
		}
	}
}

// Close 关闭连接
func (q *NATSPaymentTimeoutQueue) Close() error {
	err := q.conn.Drain()
	q.conn.Close()
	if err != nil {
		return fmt.Errorf("drain nats connection: %w", err)
	}
	return nil
}

// IsHealthy 检查 NATS 连接是否健康
func (q *NATSPaymentTimeoutQueue) IsHealthy() bool {
	return q.conn != nil && q.conn.IsConnected()
}

// ==================== NATS 辅助函数 ====================

// connectNATS 连接 NATS 服务器
func connectNATS(url string, name string) (*nats.Conn, error) {
	conn, err := nats.Connect(
		url,
		nats.Name(name),
		nats.Timeout(3*time.Second),
		nats.ReconnectWait(time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("connect nats %s: %w", url, err)
	}
	return conn, nil
}

// traceHeaders 从上下文提取追踪信息并转换为 NATS 消息头
func traceHeaders(ctx context.Context, fallbackTraceID string) nats.Header {
	headers := nats.Header{}
	traceID := tracing.TraceID(ctx)
	if traceID == "" {
		traceID = fallbackTraceID
	}
	if traceID != "" {
		headers.Set(tracing.HeaderTraceID, traceID)
		headers.Set(tracing.HeaderRequestID, traceID)
	}
	if traceParent := tracing.TraceParent(ctx); traceParent != "" {
		headers.Set(tracing.HeaderTraceParent, traceParent)
	}
	return headers
}

// contextFromNATSMessage 从 NATS 消息提取追踪信息并创建上下文
func contextFromNATSMessage(ctx context.Context, msg *nats.Msg, fallbackTraceID string) (context.Context, string) {
	if msg == nil {
		ctx, traceID := tracing.EnsureTraceID(ctx, fallbackTraceID)
		return ctx, traceID
	}
	traceID := tracing.TraceIDFromCarrier(func(key string) string {
		return msg.Header.Get(key)
	})
	ctx, traceID = tracing.EnsureTraceID(ctx, firstNonEmpty(traceID, fallbackTraceID))
	return ctx, traceID
}

// firstNonEmpty 返回第一个非空字符串
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// ensureStreamSubject 确保 NATS 流和主题存在
func ensureStreamSubject(js nats.JetStreamContext, stream string, subject string) error {
	if stream == "" || subject == "" {
		return fmt.Errorf("nats stream and subject are required")
	}
	info, err := js.StreamInfo(stream)
	if err != nil {
		// 流不存在，创建新流
		if _, err := js.AddStream(&nats.StreamConfig{
			Name:      stream,
			Subjects:  []string{subject},
			Retention: nats.LimitsPolicy,
			Storage:   nats.FileStorage,
		}); err != nil {
			return fmt.Errorf("ensure nats stream %s: %w", stream, err)
		}
		return nil
	}
	// 流存在，检查主题是否已绑定
	for _, existing := range info.Config.Subjects {
		if existing == subject {
			return nil
		}
	}
	// 添加新主题到流
	cfg := info.Config
	cfg.Subjects = append(cfg.Subjects, subject)
	if _, err := js.UpdateStream(&cfg); err != nil {
		return fmt.Errorf("update nats stream %s subjects: %w", stream, err)
	}
	return nil
}

// ackWithLog 确认消息并记录错误
func ackWithLog(logger *slog.Logger, msg *nats.Msg, action string) {
	if err := msg.Ack(); err != nil && logger != nil {
		logger.Warn(action+" failed", "error", err)
	}
}

// nakWithLog 拒绝消息并记录错误
func nakWithLog(logger *slog.Logger, msg *nats.Msg, action string) {
	if err := msg.Nak(); err != nil && logger != nil {
		logger.Warn(action+" failed", "error", err)
	}
}

// nakWithDelayLog 延迟拒绝消息并记录错误
func nakWithDelayLog(logger *slog.Logger, msg *nats.Msg, delay time.Duration, action string) {
	if err := msg.NakWithDelay(delay); err != nil && logger != nil {
		logger.Warn(action+" failed", "delay", delay.String(), "error", err)
	}
}
