package application_test

import (
	"context"
	"testing"

	"seckill-common/domain"
	"seckill-common/eventbus"
	"seckill-stock-service/internal/application"
)

// MockRepository mock 仓储
type MockRepository struct {
	deductFunc  func(ctx context.Context, activityNo, skuNo string, userID int64, quantity, purchaseLimit int64, orderNo string) (bool, error)
	peekFunc    func(ctx context.Context, activityNo, skuNo string) (int64, error)
	releaseFunc func(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error
}

func (m *MockRepository) PeekStock(ctx context.Context, activityNo, skuNo string) (int64, error) {
	if m.peekFunc != nil {
		return m.peekFunc(ctx, activityNo, skuNo)
	}
	return 100, nil
}

func (m *MockRepository) DeductStockWithLimit(ctx context.Context, activityNo, skuNo string, userID int64, quantity, purchaseLimit int64, orderNo string) (bool, error) {
	if m.deductFunc != nil {
		return m.deductFunc(ctx, activityNo, skuNo, userID, quantity, purchaseLimit, orderNo)
	}
	return true, nil
}

func (m *MockRepository) ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error {
	if m.releaseFunc != nil {
		return m.releaseFunc(ctx, activityNo, skuNo, userID, quantity, orderNo)
	}
	return nil
}

func (m *MockRepository) CleanupActivityStock(ctx context.Context, activityNo string, skuNos []string) (int64, error) {
	return 0, nil
}

func (m *MockRepository) CleanupActivityPurchases(ctx context.Context, activityNo string) (int64, error) {
	return 0, nil
}

// MockEventBus mock 事件总线
type MockEventBus struct {
	publishedEvents []domain.DomainEvent
	publishFunc     func(ctx context.Context, event domain.DomainEvent) error
	subscribeFunc   func(eventName string, handler eventbus.EventHandler) error
}

func (m *MockEventBus) Publish(ctx context.Context, event domain.DomainEvent) error {
	if m.publishFunc != nil {
		return m.publishFunc(ctx, event)
	}
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

func (m *MockEventBus) Subscribe(eventName string, handler eventbus.EventHandler) error {
	if m.subscribeFunc != nil {
		return m.subscribeFunc(eventName, handler)
	}
	return nil
}

func (m *MockEventBus) Close() error {
	return nil
}

func TestStockAppService_ReserveStock_Success(t *testing.T) {
	repo := &MockRepository{
		deductFunc: func(ctx context.Context, activityNo, skuNo string, userID int64, quantity, purchaseLimit int64, orderNo string) (bool, error) {
			return true, nil
		},
		peekFunc: func(ctx context.Context, activityNo, skuNo string) (int64, error) {
			return 98, nil
		},
	}
	bus := &MockEventBus{}
	svc := application.NewStockAppService(repo, bus)

	cmd := application.ReserveStockCommand{
		ActivityNo:    "ACT001",
		SKUNo:         "SKU001",
		UserID:        1001,
		Quantity:      2,
		PurchaseLimit: 5,
		OrderNo:       "ORD001",
	}

	err := svc.ReserveStock(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 验证事件已发布
	if len(bus.publishedEvents) != 1 {
		t.Errorf("expected 1 event published, got %d", len(bus.publishedEvents))
	}
}

func TestStockAppService_ReserveStock_InsufficientStock(t *testing.T) {
	repo := &MockRepository{
		deductFunc: func(ctx context.Context, activityNo, skuNo string, userID int64, quantity, purchaseLimit int64, orderNo string) (bool, error) {
			return false, nil
		},
	}
	bus := &MockEventBus{}
	svc := application.NewStockAppService(repo, bus)

	cmd := application.ReserveStockCommand{
		ActivityNo:    "ACT001",
		SKUNo:         "SKU001",
		UserID:        1001,
		Quantity:      2,
		PurchaseLimit: 5,
		OrderNo:       "ORD001",
	}

	err := svc.ReserveStock(context.Background(), cmd)
	if err != application.ErrStockInsufficient {
		t.Fatalf("expected ErrStockInsufficient, got %v", err)
	}

	// 验证没有事件发布
	if len(bus.publishedEvents) != 0 {
		t.Errorf("expected 0 events published on insufficient stock, got %d", len(bus.publishedEvents))
	}
}

func TestStockAppService_ReserveStock_InvalidCommand(t *testing.T) {
	repo := &MockRepository{}
	bus := &MockEventBus{}
	svc := application.NewStockAppService(repo, bus)

	cmd := application.ReserveStockCommand{
		// Missing required fields
	}

	err := svc.ReserveStock(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected validation error")
	}

	// 验证没有事件发布
	if len(bus.publishedEvents) != 0 {
		t.Errorf("expected 0 events published on invalid command, got %d", len(bus.publishedEvents))
	}
}

func TestStockAppService_ReleaseStock_Success(t *testing.T) {
	repo := &MockRepository{
		peekFunc: func(ctx context.Context, activityNo, skuNo string) (int64, error) {
			return 100, nil
		},
	}
	bus := &MockEventBus{}
	svc := application.NewStockAppService(repo, bus)

	cmd := application.ReleaseStockCommand{
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		UserID:     1001,
		Quantity:   1,
		OrderNo:    "ORD001",
	}

	err := svc.ReleaseStock(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 验证事件已发布
	if len(bus.publishedEvents) != 1 {
		t.Errorf("expected 1 event published, got %d", len(bus.publishedEvents))
	}
}

func TestStockAppService_ReleaseStock_InvalidCommand(t *testing.T) {
	repo := &MockRepository{}
	bus := &MockEventBus{}
	svc := application.NewStockAppService(repo, bus)

	cmd := application.ReleaseStockCommand{
		// Missing required fields
	}

	err := svc.ReleaseStock(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected validation error")
	}

	// 验证没有事件发布
	if len(bus.publishedEvents) != 0 {
		t.Errorf("expected 0 events published on invalid command, got %d", len(bus.publishedEvents))
	}
}
