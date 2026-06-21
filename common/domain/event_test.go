package domain

import (
	"testing"
	"time"
)

// TestEvent 实现 DomainEvent 接口用于测试
type TestEvent struct {
	BaseEvent
	eventName string
}

// NewTestEvent 创建测试事件
func NewTestEvent(aggregateID, eventName string) TestEvent {
	return TestEvent{
		BaseEvent:  NewBaseEvent(aggregateID),
		eventName: eventName,
	}
}

// EventName 实现 DomainEvent 接口
func (e TestEvent) EventName() string {
	return e.eventName
}

func TestBaseEvent(t *testing.T) {
	tests := []struct {
		name         string
		aggregateID  string
		eventName    string
		wantName     string
		wantAggID    string
	}{
		{
			name:        "basic event creation",
			aggregateID: "agg-123",
			eventName:   "TestCreated",
			wantName:    "TestCreated",
			wantAggID:   "agg-123",
		},
		{
			name:        "event with empty aggregate ID",
			aggregateID: "",
			eventName:   "EmptyAggregate",
			wantName:    "EmptyAggregate",
			wantAggID:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewTestEvent(tt.aggregateID, tt.eventName)

			if event.EventName() != tt.wantName {
				t.Errorf("EventName() = %v, want %v", event.EventName(), tt.wantName)
			}

			if event.AggregateID() != tt.wantAggID {
				t.Errorf("AggregateID() = %v, want %v", event.AggregateID(), tt.wantAggID)
			}

			if event.OccurredAt().IsZero() {
				t.Error("OccurredAt() should not be zero")
			}

			// Verify occurred time is recent (within last second)
			if time.Since(event.OccurredAt()) > time.Second {
				t.Error("OccurredAt() should be recent")
			}
		})
	}
}

func TestDomainEventInterface(t *testing.T) {
	// Verify that TestEvent implements DomainEvent interface
	var _ DomainEvent = TestEvent{}

	event := NewTestEvent("test-id", "TestEvent")

	// Test interface methods
	if event.EventName() != "TestEvent" {
		t.Errorf("EventName() = %v, want %v", event.EventName(), "TestEvent")
	}

	if event.AggregateID() != "test-id" {
		t.Errorf("AggregateID() = %v, want %v", event.AggregateID(), "test-id")
	}

	if event.OccurredAt().IsZero() {
		t.Error("OccurredAt() should not be zero")
	}
}
