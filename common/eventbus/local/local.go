package local

import (
	"context"
	"fmt"
	"log/slog"
	"seckill-common/domain"
	"seckill-common/eventbus"
	"sync"
)

// LocalBus 进程内事件总线实现
type LocalBus struct {
	handlers map[string][]eventbus.EventHandler
	mu       sync.RWMutex
}

// NewLocalBus 创建本地事件总线
func NewLocalBus() *LocalBus {
	return &LocalBus{
		handlers: make(map[string][]eventbus.EventHandler),
	}
}

// Publish 发布事件（同步调用所有订阅者）
func (b *LocalBus) Publish(ctx context.Context, event domain.DomainEvent) error {
	b.mu.RLock()
	handlers := b.handlers[event.EventName()]
	b.mu.RUnlock()

	if handlers == nil {
		return nil
	}

	// 同步调用所有处理器
	for _, handler := range handlers {
		// 继续执行其他处理器，即使某个失败
		if err := handler(event); err != nil {
			slog.Default().Error("event handler failed", "event", event.EventName(), "error", err)
		}
	}

	return nil
}

// Subscribe 订阅事件
func (b *LocalBus) Subscribe(eventName string, handler eventbus.EventHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventName] = append(b.handlers[eventName], handler)
	return nil
}

// Close 关闭事件总线
func (b *LocalBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers = make(map[string][]eventbus.EventHandler)
	return nil
}
