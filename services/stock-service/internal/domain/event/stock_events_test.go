package event_test

import (
	"testing"

	"seckill-common/domain"
	"seckill-stock-service/internal/domain/event"
)

func TestStockReservedEvent(t *testing.T) {
	evt := event.NewStockReservedEvent("ACT001", "SKU001", 2, 1001, "ORD001")

	if evt.EventName() != "stock.reserved" {
		t.Errorf("expected event name 'stock.reserved', got '%s'", evt.EventName())
	}
	if evt.ActivityNo != "ACT001" {
		t.Errorf("expected activity no 'ACT001', got '%s'", evt.ActivityNo)
	}
	if evt.SKUNo != "SKU001" {
		t.Errorf("expected sku no 'SKU001', got '%s'", evt.SKUNo)
	}
	if evt.Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", evt.Quantity)
	}
	if evt.UserID != 1001 {
		t.Errorf("expected user id 1001, got %d", evt.UserID)
	}
	if evt.OrderNo != "ORD001" {
		t.Errorf("expected order no 'ORD001', got '%s'", evt.OrderNo)
	}
	if evt.AggregateID() != "ACT001" {
		t.Errorf("expected aggregate id 'ACT001', got '%s'", evt.AggregateID())
	}
	if evt.OccurredAt().IsZero() {
		t.Error("expected occurred at to be set")
	}
}

func TestStockReleasedEvent(t *testing.T) {
	evt := event.NewStockReleasedEvent("ACT001", "SKU001", 1, 1001, "ORD001")

	if evt.EventName() != "stock.released" {
		t.Errorf("expected event name 'stock.released', got '%s'", evt.EventName())
	}
	if evt.ActivityNo != "ACT001" {
		t.Errorf("expected activity no 'ACT001', got '%s'", evt.ActivityNo)
	}
	if evt.SKUNo != "SKU001" {
		t.Errorf("expected sku no 'SKU001', got '%s'", evt.SKUNo)
	}
	if evt.Quantity != 1 {
		t.Errorf("expected quantity 1, got %d", evt.Quantity)
	}
	if evt.UserID != 1001 {
		t.Errorf("expected user id 1001, got %d", evt.UserID)
	}
	if evt.OrderNo != "ORD001" {
		t.Errorf("expected order no 'ORD001', got '%s'", evt.OrderNo)
	}
	if evt.AggregateID() != "ACT001" {
		t.Errorf("expected aggregate id 'ACT001', got '%s'", evt.AggregateID())
	}
}

func TestEventsImplementDomainEvent(t *testing.T) {
	// 确保事件实现了 domain.DomainEvent 接口
	var _ domain.DomainEvent = (*event.StockReservedEvent)(nil)
	var _ domain.DomainEvent = (*event.StockReleasedEvent)(nil)
}
