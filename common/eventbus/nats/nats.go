package nats

import (
	"context"
	"fmt"
	"seckill-common/domain"
	"seckill-common/eventbus"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSBus NATS 事件总线实现
type NATSBus struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	encoder Encoder
	subs    []*nats.Subscription
}

// NewNATSBus 创建 NATS 事件总线
func NewNATSBus(conn *nats.Conn, encoder Encoder) (*NATSBus, error) {
	js, err := conn.JetStream()
	if err != nil {
		return nil, fmt.Errorf("create jetstream context: %w", err)
	}

	return &NATSBus{
		conn:    conn,
		js:      js,
		encoder: encoder,
		subs:    make([]*nats.Subscription, 0),
	}, nil
}

// Publish 发布事件到 NATS
func (b *NATSBus) Publish(ctx context.Context, event domain.DomainEvent) error {
	// Add timeout if context doesn't have deadline
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		_, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	subject := fmt.Sprintf("events.%s", event.EventName())

	data, err := b.encoder.Encode(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}

	_, err = b.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("publish to NATS: %w", err)
	}

	return nil
}

// Subscribe 订阅 NATS 事件
func (b *NATSBus) Subscribe(eventName string, handler eventbus.EventHandler) error {
	subject := fmt.Sprintf("events.%s", eventName)
	streamName := fmt.Sprintf("EVENTS-%s", eventName)

	// 替换事件名中的无效字符
	streamName = strings.ReplaceAll(streamName, ".", "-")

	// 创建或更新 stream
	_, err := b.js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{subject},
	})
	if err != nil {
		return fmt.Errorf("create stream: %w", err)
	}

	sub, err := b.js.Subscribe(subject, func(msg *nats.Msg) {
		event, err := b.encoder.Decode(msg.Data, eventName)
		if err != nil {
			// 解码失败，Nak 重试
			_ = msg.Nak() //nolint:errcheck // nack failure is non-critical
			return
		}

		if event == nil {
			// 未注册的事件类型，Ack 确认
			_ = msg.Ack() //nolint:errcheck // ack failure is non-critical
			return
		}

		// Create context with timeout for handler execution
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Wrap handler to respect context
		handlerWrapper := func() error {
			return handler(event)
		}

		// Execute handler with context check
		errChan := make(chan error, 1)
		go func() {
			errChan <- handlerWrapper()
		}()

		select {
		case <-ctx.Done():
			// Handler timeout, Nak to retry
			_ = msg.Nak() //nolint:errcheck // nack failure is non-critical
			return
		case err := <-errChan:
			if err != nil {
				// 处理失败，Nak 重试
				_ = msg.Nak() //nolint:errcheck // nack failure is non-critical
				return
			}
		}

		// 处理成功，Ack 确认
		_ = msg.Ack() //nolint:errcheck // ack failure is non-critical
	}, nats.AckExplicit())

	if err != nil {
		return fmt.Errorf("subscribe to NATS: %w", err)
	}

	b.subs = append(b.subs, sub)
	return nil
}

// Close 关闭 NATS 连接
func (b *NATSBus) Close() error {
	for _, sub := range b.subs {
		_ = sub.Unsubscribe() //nolint:errcheck // cleanup on close
	}
	b.subs = nil
	return nil
}
