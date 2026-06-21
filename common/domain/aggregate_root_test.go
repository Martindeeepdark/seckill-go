package domain

import (
	"testing"
)

// 聚合根测试用的事件
type AggregateTestEvent struct {
	BaseEvent
	name string
}

func (e *AggregateTestEvent) EventName() string {
	return e.name
}

func TestAggregateRoot_RecordAndGetEvents(t *testing.T) {
	// Given
	root := &AggregateRoot{}
	event1 := &AggregateTestEvent{BaseEvent: NewBaseEvent("agg-1"), name: "test.event1"}
	event2 := &AggregateTestEvent{BaseEvent: NewBaseEvent("agg-1"), name: "test.event2"}

	// When
	root.RecordEvent(event1)
	root.RecordEvent(event2)

	// Then
	events := root.GetUncommittedEvents()
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
	if events[0].EventName() != "test.event1" {
		t.Errorf("expected first event name test.event1, got %s", events[0].EventName())
	}
	if events[1].EventName() != "test.event2" {
		t.Errorf("expected second event name test.event2, got %s", events[1].EventName())
	}
}

func TestAggregateRoot_ClearEvents(t *testing.T) {
	// Given
	root := &AggregateRoot{}
	event := &AggregateTestEvent{BaseEvent: NewBaseEvent("agg-1"), name: "test.event"}
	root.RecordEvent(event)

	// When
	root.ClearEvents()

	// Then
	events := root.GetUncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events after clear, got %d", len(events))
	}
}

func TestAggregateRoot_Version(t *testing.T) {
	// Given
	root := &AggregateRoot{}

	// When
	version := root.GetVersion()

	// Then
	if version != 0 {
		t.Errorf("expected initial version 0, got %d", version)
	}

	// When
	root.IncrementVersion()

	// Then
	if root.GetVersion() != 1 {
		t.Errorf("expected version 1 after increment, got %d", root.GetVersion())
	}
}

func TestAggregateRoot_GetUncommittedEvents_ReturnsCopy(t *testing.T) {
	// Given
	root := &AggregateRoot{}
	event := &AggregateTestEvent{BaseEvent: NewBaseEvent("agg-1"), name: "test.event"}
	root.RecordEvent(event)

	// When - 获取事件切片
	events := root.GetUncommittedEvents()

	// Then - 修改返回的切片
	events[0] = nil

	// 验证内部状态未被修改（说明返回的是副本）
	internalEvents := root.GetUncommittedEvents()
	if len(internalEvents) != 1 {
		t.Errorf("expected 1 internal event after external modification, got %d", len(internalEvents))
	}
	if internalEvents[0] == nil {
		t.Errorf("expected internal event to be non-nil after external modification")
	}
}

func TestAggregateRoot_NewAggregateRoot(t *testing.T) {
	// When
	root := NewAggregateRoot()

	// Then
	if root == nil {
		t.Fatal("expected non-nil AggregateRoot")
	}
	if root.GetVersion() != 0 {
		t.Errorf("expected initial version 0, got %d", root.GetVersion())
	}
	events := root.GetUncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}
