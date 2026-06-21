// Package domain 提供领域驱动设计的基础接口和类型
package domain

import "time"

// Event 领域事件接口
type Event interface {
	// EventName 事件名称
	EventName() string
	// AggregateID 聚合根ID
	AggregateID() string
	// OccurredAt 事件发生时间
	OccurredAt() time.Time
}

// DomainEvent 是领域事件接口的兼容别名。
type DomainEvent = Event

// BaseEvent 基础领域事件
type BaseEvent struct {
	aggregateID string
	occurredAt  time.Time
}

// NewBaseEvent 创建基础事件
func NewBaseEvent(aggregateID string) BaseEvent {
	return BaseEvent{
		aggregateID: aggregateID,
		occurredAt:  time.Now(),
	}
}

// AggregateID 返回聚合根ID
func (e BaseEvent) AggregateID() string {
	return e.aggregateID
}

// OccurredAt 返回事件发生时间
func (e BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}
