package nats

import (
	"context"
	"seckill-common/domain"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type testEvent struct {
	domain.BaseEvent
	Name string `json:"name"`
	Data string `json:"data"`
}

func (e *testEvent) EventName() string {
	return e.Name
}

func TestNATSBus_Integration(t *testing.T) {
	// Skip if NATS server not available
	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skip("NATS server not available, skipping integration test")
		return
	}
	defer conn.Close()

	// Given
	encoder := NewJSONEncoder()
	encoder.Register("test.event", func() domain.DomainEvent {
		return &testEvent{}
	})

	bus, err := NewNATSBus(conn, encoder)
	if err != nil {
		t.Fatalf("create NATS bus failed: %v", err)
	}
	defer bus.Close()

	// When - Subscribe
	received := make(chan domain.DomainEvent, 1)
	err = bus.Subscribe("test.event", func(event domain.DomainEvent) error {
		received <- event
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Wait for subscription to be ready
	time.Sleep(100 * time.Millisecond)

	// When - Publish
	event := &testEvent{
		BaseEvent: domain.NewBaseEvent("agg-1"),
		Name:      "test.event",
		Data:      "test data",
	}
	err = bus.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Then
	select {
	case receivedEvent := <-received:
		if receivedEvent.EventName() != "test.event" {
			t.Errorf("expected event name test.event, got %s", receivedEvent.EventName())
		}
		te, ok := receivedEvent.(*testEvent)
		if !ok {
			t.Fatal("event type mismatch")
		}
		if te.Data != "test data" {
			t.Errorf("expected data 'test data', got %s", te.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}
