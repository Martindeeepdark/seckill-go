package entity_test

import (
	"testing"

	"seckill-stock-service/internal/domain/entity"
	"seckill-stock-service/internal/domain/event"
)

func TestNewStockAggregate(t *testing.T) {
	stock, err := entity.NewStock("ACT001", "SKU001", 100, 100)
	if err != nil {
		t.Fatalf("failed to create stock: %v", err)
	}

	agg := entity.NewStockAggregate(stock)
	if agg == nil {
		t.Fatal("expected non-nil aggregate")
	}
	// 检查 Stock 值对象是否正确存储
	if agg.GetStock().ActivityNo() != "ACT001" {
		t.Errorf("expected activity no 'ACT001', got '%s'", agg.GetStock().ActivityNo())
	}
	if agg.GetVersion() != 0 {
		t.Errorf("expected version 0, got %d", agg.GetVersion())
	}
}

func TestStockAggregateRecordReserved(t *testing.T) {
	stock, _ := entity.NewStock("ACT001", "SKU001", 100, 100)
	agg := entity.NewStockAggregate(stock)

	agg.RecordReserved(2, 1001, "ORD001")

	events := agg.GetDomainEvents()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}

	evt, ok := events[0].(*event.StockReservedEvent)
	if !ok {
		t.Fatal("expected StockReservedEvent")
	}

	if evt.ActivityNo != "ACT001" {
		t.Errorf("expected activity no 'ACT001', got '%s'", evt.ActivityNo)
	}
	if evt.Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", evt.Quantity)
	}
	if evt.OrderNo != "ORD001" {
		t.Errorf("expected order no 'ORD001', got '%s'", evt.OrderNo)
	}
}

func TestStockAggregateRecordReleased(t *testing.T) {
	stock, _ := entity.NewStock("ACT001", "SKU001", 100, 100)
	agg := entity.NewStockAggregate(stock)

	agg.RecordReleased(1, 1001, "ORD001")

	events := agg.GetDomainEvents()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}

	evt, ok := events[0].(*event.StockReleasedEvent)
	if !ok {
		t.Fatal("expected StockReleasedEvent")
	}

	if evt.Quantity != 1 {
		t.Errorf("expected quantity 1, got %d", evt.Quantity)
	}
}

func TestStockAggregateClearEvents(t *testing.T) {
	stock, _ := entity.NewStock("ACT001", "SKU001", 100, 100)
	agg := entity.NewStockAggregate(stock)

	agg.RecordReserved(2, 1001, "ORD001")
	if len(agg.GetDomainEvents()) != 1 {
		t.Error("expected 1 event before clear")
	}

	agg.ClearDomainEvents()
	if len(agg.GetDomainEvents()) != 0 {
		t.Error("expected 0 events after clear")
	}
}

func TestStockAggregateGetStock(t *testing.T) {
	stock, _ := entity.NewStock("ACT001", "SKU001", 100, 80)
	agg := entity.NewStockAggregate(stock)

	retrieved := agg.GetStock()
	if retrieved.Available() != 80 {
		t.Errorf("expected available 80, got %d", retrieved.Available())
	}
}
