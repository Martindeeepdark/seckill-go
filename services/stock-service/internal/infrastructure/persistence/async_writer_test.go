package persistence

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"seckill-common/eventbus/local"
	"seckill-stock-service/internal/domain/event"
)

func TestAsyncDBWriter_HandleReserved(t *testing.T) {
	db := NewMockDB()
	bus := local.NewLocalBus()
	writer := NewAsyncDBWriter(db, bus, zap.NewNop())

	// 模拟事件
	evt := event.NewStockReservedEvent("ACT001", "SKU001", 2, 1001, "ORD001")

	// 处理事件
	err := writer.handleReserved(evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !db.InsertDeductionCalled() {
		t.Error("expected InsertStockDeduction to be called")
	}

	lastDed := db.LastDeduction()
	if lastDed.OrderNo != "ORD001" {
		t.Errorf("expected order no 'ORD001', got '%s'", lastDed.OrderNo)
	}

	if lastDed.ActivityNo != "ACT001" {
		t.Errorf("expected activity no 'ACT001', got '%s'", lastDed.ActivityNo)
	}

	if lastDed.SKUNo != "SKU001" {
		t.Errorf("expected sku no 'SKU001', got '%s'", lastDed.SKUNo)
	}

	if lastDed.Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", lastDed.Quantity)
	}

	if lastDed.UserID != 1001 {
		t.Errorf("expected user id 1001, got %d", lastDed.UserID)
	}
}

func TestAsyncDBWriter_HandleReserved_Idempotent(t *testing.T) {
	db := NewMockDB()
	db.InsertStockDeductionFunc = func(ctx context.Context, deduction StockDeduction) error {
		return ErrDuplicate
	}
	bus := local.NewLocalBus()
	writer := NewAsyncDBWriter(db, bus, zap.NewNop())

	evt := event.NewStockReservedEvent("ACT001", "SKU001", 2, 1001, "ORD001")

	// 第一次处理（重复）
	err := writer.handleReserved(evt)
	if err != nil {
		t.Fatalf("expected no error on duplicate, got %v", err)
	}
}

func TestAsyncDBWriter_HandleReleased(t *testing.T) {
	called := false
	db := NewMockDB()
	db.InsertStockReleaseFunc = func(ctx context.Context, release StockRelease) error {
		called = true
		return nil
	}
	bus := local.NewLocalBus()
	writer := NewAsyncDBWriter(db, bus, zap.NewNop())

	evt := event.NewStockReleasedEvent("ACT001", "SKU001", 1, 1001, "ORD001")

	err := writer.handleReleased(evt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !called {
		t.Error("expected InsertStockRelease to be called")
	}
}

func TestAsyncDBWriter_HandleReleased_Idempotent(t *testing.T) {
	db := NewMockDB()
	db.InsertStockReleaseFunc = func(ctx context.Context, release StockRelease) error {
		return ErrDuplicate
	}
	bus := local.NewLocalBus()
	writer := NewAsyncDBWriter(db, bus, zap.NewNop())

	evt := event.NewStockReleasedEvent("ACT001", "SKU001", 1, 1001, "ORD001")

	err := writer.handleReleased(evt)
	if err != nil {
		t.Fatalf("expected no error on duplicate, got %v", err)
	}
}

func TestAsyncDBWriter_Start_SubscribesToEvents(t *testing.T) {
	db := NewMockDB()
	bus := local.NewLocalBus()
	writer := NewAsyncDBWriter(db, bus, zap.NewNop())

	ctx := context.Background()

	err := writer.Start(ctx)
	if err != nil {
		t.Fatalf("expected no error on Start, got %v", err)
	}
}
