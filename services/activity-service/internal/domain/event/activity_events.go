// Package event 定义活动领域的领域事件
package event

import (
	"seckill-common/domain"
	"time"
)

// ActivityStartedEvent 活动开始事件
type ActivityStartedEvent struct {
	domain.BaseEvent
	ActivityNo string    `json:"activityNo"`
	StartedAt  time.Time `json:"startedAt"`
}

// NewActivityStartedEvent 创建活动开始事件
func NewActivityStartedEvent(activityNo string, startedAt time.Time) *ActivityStartedEvent {
	return &ActivityStartedEvent{
		BaseEvent:  domain.NewBaseEvent(activityNo),
		ActivityNo: activityNo,
		StartedAt:  startedAt,
	}
}

// EventName 实现 DomainEvent 接口
func (e *ActivityStartedEvent) EventName() string {
	return "activity.started"
}

// ActivityEndedEvent 活动结束事件
type ActivityEndedEvent struct {
	domain.BaseEvent
	ActivityNo string    `json:"activityNo"`
	Reason     string    `json:"reason"`
	EndedAt    time.Time `json:"endedAt"`
}

// NewActivityEndedEvent 创建活动结束事件
func NewActivityEndedEvent(activityNo, reason string, endedAt time.Time) *ActivityEndedEvent {
	return &ActivityEndedEvent{
		BaseEvent:  domain.NewBaseEvent(activityNo),
		ActivityNo: activityNo,
		Reason:     reason,
		EndedAt:    endedAt,
	}
}

// EventName 实现 DomainEvent 接口
func (e *ActivityEndedEvent) EventName() string {
	return "activity.ended"
}

// SKUAddedEvent SKU 添加事件
type SKUAddedEvent struct {
	domain.BaseEvent
	ActivityNo string `json:"activityNo"`
	SKUNo      string `json:"skuNo"`
	Stock      int64  `json:"stock"`
	Price      int64  `json:"price"`
}

// NewSKUAddedEvent 创建 SKU 添加事件
func NewSKUAddedEvent(activityNo, skuNo string, stock, price int64) *SKUAddedEvent {
	return &SKUAddedEvent{
		BaseEvent:  domain.NewBaseEvent(activityNo),
		ActivityNo: activityNo,
		SKUNo:      skuNo,
		Stock:      stock,
		Price:      price,
	}
}

// EventName 实现 DomainEvent 接口
func (e *SKUAddedEvent) EventName() string {
	return "activity.sku.added"
}

// SKURemovedEvent SKU 移除事件
type SKURemovedEvent struct {
	domain.BaseEvent
	ActivityNo string `json:"activityNo"`
	SKUNo      string `json:"skuNo"`
	Reason     string `json:"reason"`
}

// NewSKURemovedEvent 创建 SKU 移除事件
func NewSKURemovedEvent(activityNo, skuNo, reason string) *SKURemovedEvent {
	return &SKURemovedEvent{
		BaseEvent:  domain.NewBaseEvent(activityNo),
		ActivityNo: activityNo,
		SKUNo:      skuNo,
		Reason:     reason,
	}
}

// EventName 实现 DomainEvent 接口
func (e *SKURemovedEvent) EventName() string {
	return "activity.sku.removed"
}
