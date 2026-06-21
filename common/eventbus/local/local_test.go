package local

import (
	"context"
	"seckill-common/domain"
	"sync"
	"testing"
)

type testEvent struct {
	domain.BaseEvent
	name string
}

func (e *testEvent) EventName() string {
	return e.name
}

func TestLocalBus_PublishAndSubscribe(t *testing.T) {
	// Given
	bus := NewLocalBus()
	var receivedEvent domain.DomainEvent
	var mu sync.Mutex

	err := bus.Subscribe("test.event", func(event domain.DomainEvent) error {
		mu.Lock()
		defer mu.Unlock()
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

	mu.Lock()
	defer mu.Unlock()
	if receivedEvent == nil {
		t.Fatal("event not received")
	}
	if receivedEvent.EventName() != "test.event" {
		t.Errorf("expected event name test.event, got %s", receivedEvent.EventName())
	}
}

func TestLocalBus_MultipleSubscribers(t *testing.T) {
	// Given
	bus := NewLocalBus()
	count := 0
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		err := bus.Subscribe("test.event", func(event domain.DomainEvent) error {
			mu.Lock()
			defer mu.Unlock()
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("subscribe failed: %v", err)
		}
	}

	// When
	event := &testEvent{BaseEvent: domain.NewBaseEvent("agg-1"), name: "test.event"}
	err := bus.Publish(context.Background(), event)

	// Then
	if err != nil {
		t.Errorf("publish failed: %v", err)
	}

	// LocalBus is synchronous, no need to wait
	mu.Lock()
	defer mu.Unlock()
	if count != 3 {
		t.Errorf("expected 3 handlers called, got %d", count)
	}
}

func TestLocalBus_Close(t *testing.T) {
	// Given
	bus := NewLocalBus()

	// When
	err := bus.Close()

	// Then
	if err != nil {
		t.Errorf("close failed: %v", err)
	}
}

func TestLocalBus_SubscribeNilHandler(t *testing.T) {
	// Given
	bus := NewLocalBus()

	// When
	err := bus.Subscribe("test.event", nil)

	// Then
	if err == nil {
		t.Error("expected error for nil handler, got nil")
	}
}
