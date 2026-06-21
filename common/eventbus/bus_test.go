package eventbus

import (
	"context"
	"errors"
	"seckill-common/domain"
	"sync"
	"testing"
)

// MockBus 用于测试的 Mock 实现
type MockBus struct {
	publishedEvents []domain.DomainEvent
	handlers        map[string][]EventHandler
	publishError    error
	mu              sync.RWMutex
}

func NewMockBus() *MockBus {
	return &MockBus{
		handlers: make(map[string][]EventHandler),
	}
}

func (m *MockBus) Publish(ctx context.Context, event domain.DomainEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.publishError != nil {
		return m.publishError
	}
	m.publishedEvents = append(m.publishedEvents, event)

	// 触发订阅者
	if handlers, ok := m.handlers[event.EventName()]; ok {
		for _, handler := range handlers {
			_ = handler(event)
		}
	}
	return nil
}

func (m *MockBus) Subscribe(eventName string, handler EventHandler) error {
	if eventName == "" {
		return errors.New("event name cannot be empty")
	}
	if handler == nil {
		return errors.New("handler cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[eventName] = append(m.handlers[eventName], handler)
	return nil
}

func (m *MockBus) Close() error {
	return nil
}

func (m *MockBus) SetPublishError(err error) {
	m.publishError = err
}

func (m *MockBus) GetPublishedEvents() []domain.DomainEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.publishedEvents
}

// 测试用事件
type testEvent struct {
	domain.BaseEvent
	name string
}

func (e *testEvent) EventName() string {
	return e.name
}

func TestMockBus_PublishAndSubscribe(t *testing.T) {
	// Given
	bus := NewMockBus()
	var receivedEvent domain.DomainEvent

	err := bus.Subscribe("test.event", func(event domain.DomainEvent) error {
		receivedEvent = event
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// When
	event := &testEvent{BaseEvent: domain.NewBaseEvent("agg-1"), name: "test.event"}
	err = bus.Publish(context.Background(), event)

	// Then
	if err != nil {
		t.Errorf("publish failed: %v", err)
	}
	if receivedEvent == nil {
		t.Fatal("event not received")
	}
	if receivedEvent.EventName() != "test.event" {
		t.Errorf("expected event name test.event, got %s", receivedEvent.EventName())
	}
}

func TestMockBus_PublishError(t *testing.T) {
	// Given
	bus := NewMockBus()
	bus.SetPublishError(errors.New("publish failed"))

	// When
	event := &testEvent{BaseEvent: domain.NewBaseEvent("agg-1"), name: "test.event"}
	err := bus.Publish(context.Background(), event)

	// Then
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestMockBus_SubscribeValidation(t *testing.T) {
	bus := NewMockBus()

	// Test empty event name
	err := bus.Subscribe("", func(event domain.DomainEvent) error {
		return nil
	})
	if err == nil {
		t.Error("expected error for empty event name, got nil")
	}

	// Test nil handler
	err = bus.Subscribe("test.event", nil)
	if err == nil {
		t.Error("expected error for nil handler, got nil")
	}
}
