package hybrid

import (
	"context"
	"seckill-common/domain"
	"seckill-common/eventbus"
	"testing"
)

type testEvent struct {
	domain.BaseEvent
	name string
}

func (e *testEvent) EventName() string {
	return e.name
}

type mockBus struct {
	publishedEvents []domain.DomainEvent
}

func (m *mockBus) Publish(ctx context.Context, event domain.DomainEvent) error {
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

func (m *mockBus) Subscribe(eventName string, handler eventbus.EventHandler) error {
	return nil
}

func (m *mockBus) Close() error {
	return nil
}

func TestHybridBus_PublishLocal(t *testing.T) {
	// Given
	local := &mockBus{}
	remote := &mockBus{}
	router := NewDefaultRouter([]string{"remote.event"})
	bus, err := NewHybridBus(local, remote, router)
	if err != nil {
		t.Fatalf("create hybrid bus: %v", err)
	}

	// When - 发布本地事件
	localEvent := &testEvent{BaseEvent: domain.NewBaseEvent("agg-1"), name: "local.event"}
	err = bus.Publish(context.Background(), localEvent)

	// Then
	if err != nil {
		t.Errorf("publish failed: %v", err)
	}
	if len(local.publishedEvents) != 1 {
		t.Errorf("expected 1 local event, got %d", len(local.publishedEvents))
	}
	if len(remote.publishedEvents) != 0 {
		t.Errorf("expected 0 remote events, got %d", len(remote.publishedEvents))
	}
}

func TestHybridBus_PublishRemote(t *testing.T) {
	// Given
	local := &mockBus{}
	remote := &mockBus{}
	router := NewDefaultRouter([]string{"remote.event"})
	bus, err := NewHybridBus(local, remote, router)
	if err != nil {
		t.Fatalf("create hybrid bus: %v", err)
	}

	// When - 发布远程事件
	remoteEvent := &testEvent{BaseEvent: domain.NewBaseEvent("agg-1"), name: "remote.event"}
	err = bus.Publish(context.Background(), remoteEvent)

	// Then
	if err != nil {
		t.Errorf("publish failed: %v", err)
	}
	if len(local.publishedEvents) != 1 {
		t.Errorf("expected 1 local event, got %d", len(local.publishedEvents))
	}
	if len(remote.publishedEvents) != 1 {
		t.Errorf("expected 1 remote event, got %d", len(remote.publishedEvents))
	}
}

func TestHybridBus_Subscribe(t *testing.T) {
	// Given
	local := &mockBus{}
	remote := &mockBus{}
	router := NewDefaultRouter([]string{})
	bus, err := NewHybridBus(local, remote, router)
	if err != nil {
		t.Fatalf("create hybrid bus: %v", err)
	}

	handler := eventbus.EventHandler(func(event domain.DomainEvent) error {
		return nil
	})

	// When
	err = bus.Subscribe("test.event", handler)

	// Then
	if err != nil {
		t.Errorf("subscribe failed: %v", err)
	}
}

func TestHybridBus_Close(t *testing.T) {
	// Given
	local := &mockBus{}
	remote := &mockBus{}
	router := NewDefaultRouter([]string{})
	bus, err := NewHybridBus(local, remote, router)
	if err != nil {
		t.Fatalf("create hybrid bus: %v", err)
	}

	// When
	err = bus.Close()

	// Then
	if err != nil {
		t.Errorf("close failed: %v", err)
	}
}

func TestHybridBus_NewHybridBusValidation(t *testing.T) {
	tests := []struct {
		name    string
		local   eventbus.Bus
		remote  eventbus.Bus
		router  EventRouter
		wantErr bool
	}{
		{
			name:    "all valid",
			local:   &mockBus{},
			remote:  &mockBus{},
			router:  NewDefaultRouter([]string{}),
			wantErr: false,
		},
		{
			name:    "nil local",
			local:   nil,
			remote:  &mockBus{},
			router:  NewDefaultRouter([]string{}),
			wantErr: true,
		},
		{
			name:    "nil remote",
			local:   &mockBus{},
			remote:  nil,
			router:  NewDefaultRouter([]string{}),
			wantErr: true,
		},
		{
			name:    "nil router",
			local:   &mockBus{},
			remote:  &mockBus{},
			router:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus, err := NewHybridBus(tt.local, tt.remote, tt.router)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if bus != nil {
					t.Error("expected nil bus on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if bus == nil {
					t.Error("expected bus on success")
				}
			}
		})
	}
}
