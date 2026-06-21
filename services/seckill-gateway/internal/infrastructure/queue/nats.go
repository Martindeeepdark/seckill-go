// Package queue 提供消息队列发布器实现
// 支持 NATS 和 Redis 作为消息队列后端
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"seckill-common/tracing"

	"seckill-gateway-service/internal/application"
	"seckill-gateway-service/internal/config"
)

// NATSPublisher 将 PartIn 事件发布到 NATS JetStream
type NATSPublisher struct {
	conn           *nats.Conn
	js             nats.JetStreamContext
	stream         string
	subject        string
	postPaySubject string
	logger         *slog.Logger
}

// NewNATSPublisher 连接 NATS 并创建秒杀流
func NewNATSPublisher(mq config.MQConfig, logger *slog.Logger) (*NATSPublisher, error) {
	conn, err := nats.Connect(
		mq.NATSURL,
		nats.Name("seckill-gateway"),
		nats.Timeout(3*time.Second),
		nats.ReconnectWait(time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("connect nats %s: %w", mq.NATSURL, err)
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create nats jetstream context: %w", err)
	}
	publisher := &NATSPublisher{
		conn:           conn,
		js:             js,
		stream:         mq.NATSStream,
		subject:        mq.NATSSubject,
		postPaySubject: mq.NATSPostPaySubject,
		logger:         logger,
	}
	if err := publisher.ensureStream(); err != nil {
		conn.Close()
		return nil, err
	}
	return publisher, nil
}

// ensureStream 确保流和主题存在
func (p *NATSPublisher) ensureStream() error {
	if p.stream == "" || p.subject == "" || p.postPaySubject == "" {
		return fmt.Errorf("nats stream and subject are required")
	}
	return ensureStreamSubjects(p.js, p.stream, []string{p.subject, p.postPaySubject})
}

// Publish 将 PartInEvent 推送到 NATS JetStream
func (p *NATSPublisher) Publish(ctx context.Context, event application.PartInEvent) error {
	data, err := json.Marshal(toMessage(event))
	if err != nil {
		return fmt.Errorf("marshal seckill message: %w", err)
	}
	msg := &nats.Msg{
		Subject: p.subject,
		Header:  nats.Header{},
		Data:    data,
	}
	if event.TraceID != "" {
		msg.Header.Set(tracing.HeaderTraceID, event.TraceID)
		msg.Header.Set(tracing.HeaderRequestID, event.TraceID)
	}
	if traceParent := tracing.TraceParent(ctx); traceParent != "" {
		msg.Header.Set(tracing.HeaderTraceParent, traceParent)
	}
	if _, err := p.js.PublishMsg(msg, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish nats seckill message: %w", err)
	}
	if p.logger != nil {
		p.logger.Info("part-in event published to nats",
			"traceId", event.TraceID,
			"userId", event.UserID,
			"activityNo", event.ActivityNo,
			"subject", p.subject,
		)
	}
	return nil
}

// PublishPostPayTask 发布支付后任务到 NATS
func (p *NATSPublisher) PublishPostPayTask(ctx context.Context, task application.PostPayTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal post-pay task: %w", err)
	}
	msg := &nats.Msg{
		Subject: p.postPaySubject,
		Header:  nats.Header{},
		Data:    data,
	}
	if task.RequestTraceID != "" {
		msg.Header.Set(tracing.HeaderTraceID, task.RequestTraceID)
		msg.Header.Set(tracing.HeaderRequestID, task.RequestTraceID)
	}
	if traceParent := tracing.TraceParent(ctx); traceParent != "" {
		msg.Header.Set(tracing.HeaderTraceParent, traceParent)
	}
	if _, err := p.js.PublishMsg(msg, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish nats post-pay task: %w", err)
	}
	if p.logger != nil {
		p.logger.Info("post-pay task published to nats",
			"traceId", task.RequestTraceID,
			"orderNo", task.OrderNo,
			"type", task.Type,
			"subject", p.postPaySubject,
		)
	}
	return nil
}

// Close 排空并关闭 NATS 连接
func (p *NATSPublisher) Close() error {
	err := p.conn.Drain()
	p.conn.Close()
	if err != nil {
		return fmt.Errorf("drain nats connection: %w", err)
	}
	return nil
}

// ensureStreamSubjects 确保流的主题存在
func ensureStreamSubjects(js nats.JetStreamContext, stream string, subjects []string) error {
	info, err := js.StreamInfo(stream)
	if err != nil {
		if _, err := js.AddStream(&nats.StreamConfig{
			Name:      stream,
			Subjects:  uniqueSubjects(subjects),
			Retention: nats.LimitsPolicy,
			Storage:   nats.FileStorage,
		}); err != nil {
			return fmt.Errorf("ensure nats stream %s: %w", stream, err)
		}
		return nil
	}
	cfg := info.Config
	changed := false
	for _, subject := range subjects {
		if subject == "" {
			continue
		}
		found := false
		for _, existing := range cfg.Subjects {
			if existing == subject {
				found = true
				break
			}
		}
		if !found {
			cfg.Subjects = append(cfg.Subjects, subject)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if _, err := js.UpdateStream(&cfg); err != nil {
		return fmt.Errorf("update nats stream %s subjects: %w", stream, err)
	}
	return nil
}

// uniqueSubjects 返回去重后的主题列表
func uniqueSubjects(subjects []string) []string {
	seen := make(map[string]struct{}, len(subjects))
	result := make([]string, 0, len(subjects))
	for _, subject := range subjects {
		if subject == "" {
			continue
		}
		if _, ok := seen[subject]; ok {
			continue
		}
		seen[subject] = struct{}{}
		result = append(result, subject)
	}
	return result
}
