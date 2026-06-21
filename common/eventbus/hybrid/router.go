package hybrid

import "seckill-common/domain"

// EventRouter 事件路由器接口
type EventRouter interface {
	// ShouldPublishRemote 判断事件是否需要发布到远程
	ShouldPublishRemote(event domain.DomainEvent) bool
}

// DefaultRouter 默认路由器
type DefaultRouter struct {
	remoteEvents map[string]bool
}

// NewDefaultRouter 创建默认路由器
func NewDefaultRouter(remoteEvents []string) *DefaultRouter {
	eventsMap := make(map[string]bool)
	for _, eventName := range remoteEvents {
		eventsMap[eventName] = true
	}

	return &DefaultRouter{
		remoteEvents: eventsMap,
	}
}

// ShouldPublishRemote 实现 EventRouter 接口
func (r *DefaultRouter) ShouldPublishRemote(event domain.DomainEvent) bool {
	return r.remoteEvents[event.EventName()]
}
