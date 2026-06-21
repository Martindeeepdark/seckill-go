// Package eventbus 提供统一的事件总线抽象
package eventbus

import (
	"context"
	"seckill-common/domain"
)

// Bus 事件总线接口
type Bus interface {
	// Publish 发布事件
	Publish(ctx context.Context, event domain.DomainEvent) error

	// Subscribe 订阅事件
	Subscribe(eventName string, handler EventHandler) error

	// Close 关闭事件总线
	Close() error
}

// EventHandler 事件处理器
type EventHandler func(event domain.DomainEvent) error
