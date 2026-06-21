package rpc_test

import (
	"context"
	"testing"

	"seckill-common/domain"
	"seckill-common/eventbus"
	"seckill-stock-service/internal/application"
	"seckill-stock-service/internal/infrastructure/rpc"

	stockv1 "seckill-api/stock/v1"
)

// MockRepository for testing
type mockRepository struct {
	deductFunc  func(ctx context.Context, activityNo, skuNo string, userID int64, quantity, purchaseLimit int64, orderNo string) (bool, error)
	peekFunc    func(ctx context.Context, activityNo, skuNo string) (int64, error)
	releaseFunc func(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error
}

func (m *mockRepository) PeekStock(ctx context.Context, activityNo, skuNo string) (int64, error) {
	if m.peekFunc != nil {
		return m.peekFunc(ctx, activityNo, skuNo)
	}
	return 100, nil
}

func (m *mockRepository) DeductStockWithLimit(ctx context.Context, activityNo, skuNo string, userID int64, quantity, purchaseLimit int64, orderNo string) (bool, error) {
	if m.deductFunc != nil {
		return m.deductFunc(ctx, activityNo, skuNo, userID, quantity, purchaseLimit, orderNo)
	}
	return true, nil
}

func (m *mockRepository) ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error {
	if m.releaseFunc != nil {
		return m.releaseFunc(ctx, activityNo, skuNo, userID, quantity, orderNo)
	}
	return nil
}

func (m *mockRepository) CleanupActivityStock(ctx context.Context, activityNo string, skuNos []string) (int64, error) {
	return 0, nil
}

func (m *mockRepository) CleanupActivityPurchases(ctx context.Context, activityNo string) (int64, error) {
	return 0, nil
}

// mockEventBus for testing
type mockEventBus struct {
	publishFunc func(ctx context.Context, event domain.DomainEvent) error
}

func (m *mockEventBus) Publish(ctx context.Context, event domain.DomainEvent) error {
	if m.publishFunc != nil {
		return m.publishFunc(ctx, event)
	}
	return nil
}

func (m *mockEventBus) Subscribe(eventName string, handler eventbus.EventHandler) error {
	return nil
}

func (m *mockEventBus) Close() error {
	return nil
}

func TestStockPBService_Deduct_Success(t *testing.T) {
	repo := &mockRepository{
		deductFunc: func(ctx context.Context, activityNo, skuNo string, userID int64, quantity, purchaseLimit int64, orderNo string) (bool, error) {
			return true, nil
		},
		peekFunc: func(ctx context.Context, activityNo, skuNo string) (int64, error) {
			return 98, nil
		},
	}
	bus := &mockEventBus{}
	appService := application.NewStockAppService(repo, bus)
	svc := rpc.NewStockPBService(repo, appService)

	req := &stockv1.DeductRequest{
		ActivityNo:    "ACT001",
		SkuNo:         "SKU001",
		UserId:        1001,
		Quantity:      2,
		PurchaseLimit: 5,
		OrderNo:       "ORD001",
	}

	resp, err := svc.Deduct(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	if !resp.Ok {
		t.Errorf("expected Ok=true, got Ok=%v", resp.Ok)
	}
}

func TestStockPBService_Deduct_InsufficientStock(t *testing.T) {
	repo := &mockRepository{
		deductFunc: func(ctx context.Context, activityNo, skuNo string, userID int64, quantity, purchaseLimit int64, orderNo string) (bool, error) {
			return false, nil
		},
	}
	bus := &mockEventBus{}
	appService := application.NewStockAppService(repo, bus)
	svc := rpc.NewStockPBService(repo, appService)

	req := &stockv1.DeductRequest{
		ActivityNo:    "ACT001",
		SkuNo:         "SKU001",
		UserId:        1001,
		Quantity:      2,
		PurchaseLimit: 5,
		OrderNo:       "ORD001",
	}

	resp, err := svc.Deduct(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	if resp.Ok {
		t.Errorf("expected Ok=false (insufficient stock), got Ok=%v", resp.Ok)
	}
}

func TestStockPBService_Release_Success(t *testing.T) {
	repo := &mockRepository{}
	bus := &mockEventBus{}
	appService := application.NewStockAppService(repo, bus)
	svc := rpc.NewStockPBService(repo, appService)

	req := &stockv1.ReleaseRequest{
		ActivityNo: "ACT001",
		SkuNo:      "SKU001",
		UserId:     1001,
		Quantity:   1,
		OrderNo:    "ORD001",
	}

	resp, err := svc.Release(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	// Verify it's an empty response as expected
	if resp == nil {
		t.Error("expected non-nil response")
	}
}

func TestStockPBService_Deduct_ValidationError(t *testing.T) {
	repo := &mockRepository{}
	bus := &mockEventBus{}
	appService := application.NewStockAppService(repo, bus)
	svc := rpc.NewStockPBService(repo, appService)

	req := &stockv1.DeductRequest{
		// Missing required fields
	}

	_, err := svc.Deduct(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestStockPBService_Release_ValidationError(t *testing.T) {
	repo := &mockRepository{}
	bus := &mockEventBus{}
	appService := application.NewStockAppService(repo, bus)
	svc := rpc.NewStockPBService(repo, appService)

	req := &stockv1.ReleaseRequest{
		// Missing required fields
	}

	_, err := svc.Release(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestNewStockPBService(t *testing.T) {
	bus := &mockEventBus{}
	repo := &mockRepository{}
	appService := application.NewStockAppService(repo, bus)

	svc := rpc.NewStockPBService(repo, appService)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}
