package hybrid

import (
	"context"
	"fmt"
	"seckill-common/domain"
	"seckill-common/eventbus"
	"strings"
)

// HybridBus 混合事件总线
type HybridBus struct {
	local  eventbus.Bus
	remote eventbus.Bus
	router EventRouter
}

// NewHybridBus 创建混合事件总线
func NewHybridBus(local, remote eventbus.Bus, router EventRouter) (*HybridBus, error) {
	if local == nil {
		return nil, fmt.Errorf("local bus cannot be nil")
	}
	if remote == nil {
		return nil, fmt.Errorf("remote bus cannot be nil")
	}
	if router == nil {
		return nil, fmt.Errorf("router cannot be nil")
	}

	return &HybridBus{
		local:  local,
		remote: remote,
		router: router,
	}, nil
}

// Publish 发布事件（本地总是发布，远程根据路由决定）
func (b *HybridBus) Publish(ctx context.Context, event domain.DomainEvent) error {
	// 本地订阅者总是通知（低延迟）
	if err := b.local.Publish(ctx, event); err != nil {
		return fmt.Errorf("publish to local: %w", err)
	}

	// 根据路由策略决定是否发布到远程
	if b.router.ShouldPublishRemote(event) {
		if err := b.remote.Publish(ctx, event); err != nil {
			// 远程发布失败不影响本地，只记录错误
			return fmt.Errorf("publish to remote: %w", err)
		}
	}

	return nil
}

// Subscribe 订阅事件（同时订阅本地和远程）
//
// 如果本地订阅失败，直接返回错误。
// 如果远程订阅失败，本地订阅已完成但远程订阅失败，返回错误。
// 这种部分失败状态需要调用者处理：可能需要清理本地订阅或重试整个订阅操作。
func (b *HybridBus) Subscribe(eventName string, handler eventbus.EventHandler) error {
	if err := b.local.Subscribe(eventName, handler); err != nil {
		return fmt.Errorf("subscribe local: %w", err)
	}

	if err := b.remote.Subscribe(eventName, handler); err != nil {
		return fmt.Errorf("subscribe remote: %w", err)
	}

	return nil
}

// Close 关闭事件总线
func (b *HybridBus) Close() error {
	var errs []string

	if err := b.local.Close(); err != nil {
		errs = append(errs, fmt.Sprintf("local close: %v", err))
	}

	if err := b.remote.Close(); err != nil {
		errs = append(errs, fmt.Sprintf("remote close: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %s", strings.Join(errs, ", "))
	}

	return nil
}
