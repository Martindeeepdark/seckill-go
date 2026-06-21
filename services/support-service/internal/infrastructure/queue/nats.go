// Package queue 提供消息队列实现
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"seckill-common/tracing"
	paymentdomain "seckill-support-service/internal/domain/entity"
)

// NATSPostPayPublisher NATS支付后任务发布器
type NATSPostPayPublisher struct {
	conn    *nats.Conn            // NATS连接
	js      nats.JetStreamContext // JetStream上下文
	stream  string               // 流名称
	subject string               // 主题
	logger  *slog.Logger         // 日志记录器
}

// NewNATSPostPayPublisher 创建NATS支付后任务发布器
func NewNATSPostPayPublisher(url string, stream string, subject string, logger *slog.Logger) (*NATSPostPayPublisher, error) {
	// 连接NATS服务器
	conn, err := nats.Connect(
		url,
		nats.Name("support-service-post-pay"),
		nats.Timeout(3*time.Second),
		nats.ReconnectWait(time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("connect nats")
	}
	// 创建JetStream上下文
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create nats jetstream context")
	}
	p := &NATSPostPayPublisher{conn: conn, js: js, stream: stream, subject: subject, logger: logger}
	// 确保流存在
	if err := p.ensureStream(); err != nil {
		conn.Close()
		return nil, err
	}
	return p, nil
}

// PublishPostPayTask 发布支付后任务
func (p *NATSPostPayPublisher) PublishPostPayTask(ctx context.Context, task paymentdomain.PostPayTask) error {
	// 序列化任务
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal post-pay task")
	}
	// 创建消息
	msg := &nats.Msg{
		Subject: p.subject,
		Header:  traceHeaders(ctx, task.RequestTraceID),
		Data:    data,
	}
	// 发布消息
	if _, err := p.js.PublishMsg(msg, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish nats post-pay task")
	}
	if p.logger != nil {
		p.logger.Info("post-pay task published to nats",
			"traceId", task.RequestTraceID,
			"orderNo", task.OrderNo,
			"type", task.Type,
			"subject", p.subject,
		)
	}
	return nil
}

// Close 关闭发布器
func (p *NATSPostPayPublisher) Close() error {
	// 排空连接
	err := p.conn.Drain()
	p.conn.Close()
	if err != nil {
		return fmt.Errorf("drain nats post-pay publisher")
	}
	return nil
}

// ensureStream 确保流存在
func (p *NATSPostPayPublisher) ensureStream() error {
	// 校验参数
	if p.stream == "" || p.subject == "" {
		return fmt.Errorf("nats stream and subject are required")
	}
	// 获取流信息
	info, err := p.js.StreamInfo(p.stream)
	if err != nil {
		// 流不存在，创建新流
		if _, err := p.js.AddStream(&nats.StreamConfig{
			Name:      p.stream,
			Subjects:  []string{p.subject},
			Retention: nats.LimitsPolicy,
			Storage:   nats.FileStorage,
		}); err != nil {
			return fmt.Errorf("ensure nats stream")
		}
		return nil
	}
	// 流已存在，检查是否需要添加主题
	cfg := info.Config
	for _, existing := range cfg.Subjects {
		if existing == p.subject {
			return nil
		}
	}
	// 添加新主题
	cfg.Subjects = append(cfg.Subjects, p.subject)
	if _, err := p.js.UpdateStream(&cfg); err != nil {
		return fmt.Errorf("update nats stream")
	}
	return nil
}

// traceHeaders 创建追踪头
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

