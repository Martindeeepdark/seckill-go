package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"seckill-common/domain"
	"seckill-common/eventbus"
	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/domain/repository"
)

// MockOrderRepository is a map-based in-memory implementation
type MockOrderRepository struct {
	mu     sync.RWMutex
	orders map[string]*entity.Order
}

func NewMockOrderRepository() *MockOrderRepository {
	return &MockOrderRepository{
		orders: make(map[string]*entity.Order),
	}
}

func (m *MockOrderRepository) Save(ctx context.Context, order *entity.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.orders[order.OrderNo] = order
	return nil
}

func (m *MockOrderRepository) GetByOrderNo(ctx context.Context, orderNo string) (*entity.Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	order, exists := m.orders[orderNo]
	if !exists {
		return nil, repository.ErrNotFound
	}
	return order, nil
}

func (m *MockOrderRepository) GetByUserID(ctx context.Context, userID int64) ([]*entity.Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var orders []*entity.Order
	for _, order := range m.orders {
		if order.UserID == userID {
			orders = append(orders, order)
		}
	}
	return orders, nil
}

func (m *MockOrderRepository) GetPendingOrders(ctx context.Context, beforeTime time.Time, limit int) ([]*entity.Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var orders []*entity.Order
	for _, order := range m.orders {
		if order.IsPending() && order.CreatedAt.Before(beforeTime) {
			orders = append(orders, order)
			if len(orders) >= limit {
				break
			}
		}
	}
	return orders, nil
}

// MockEventBus is a slice-based in-memory implementation
type MockEventBus struct {
	mu     sync.Mutex
	events []domain.DomainEvent
}

func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		events: make([]domain.DomainEvent, 0),
	}
}

func (m *MockEventBus) Publish(ctx context.Context, event domain.DomainEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, event)
	return nil
}

func (m *MockEventBus) Subscribe(eventType string, handler eventbus.EventHandler) error {
	return nil
}

func (m *MockEventBus) Close() error {
	return nil
}

func (m *MockEventBus) GetEvents() []domain.DomainEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.events
}

func (m *MockEventBus) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = make([]domain.DomainEvent, 0)
}

// TestCreateOrder tests the CreateOrder use case
func TestCreateOrder(t *testing.T) {
	repo := NewMockOrderRepository()
	eventBus := NewMockEventBus()
	service := NewOrderAppService(repo, eventBus, nil)

	ctx := context.Background()
	cmd := CreateOrderCommand{
		OrderNo:        "ORD001",
		UserID:         1001,
		ActivityNo:     "ACT001",
		SKUNo:          "SKU001",
		Quantity:       2,
		PayAmount:      20000,
		TraceID:        "trace-123",
		RequestTraceID: "req-trace-456",
	}

	err := service.CreateOrder(ctx, cmd)

	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Verify order was saved
	order, err := repo.GetByOrderNo(ctx, "ORD001")
	if err != nil {
		t.Fatalf("Failed to retrieve saved order: %v", err)
	}

	if order.OrderNo != "ORD001" {
		t.Errorf("Expected order no ORD001, got %s", order.OrderNo)
	}

	if order.UserID != 1001 {
		t.Errorf("Expected user ID 1001, got %d", order.UserID)
	}

	if order.Status != entity.OrderPending {
		t.Errorf("Expected status %s, got %s", entity.OrderPending, order.Status)
	}

	// Verify event was published
	events := eventBus.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
}

func TestCreateOrderInvalidCommand(t *testing.T) {
	repo := NewMockOrderRepository()
	eventBus := NewMockEventBus()
	service := NewOrderAppService(repo, eventBus, nil)

	ctx := context.Background()
	cmd := CreateOrderCommand{
		OrderNo: "", // Invalid: empty order no
		UserID:  1001,
	}

	err := service.CreateOrder(ctx, cmd)

	if err == nil {
		t.Fatal("Expected error for invalid command, got nil")
	}

	// Verify no order was saved
	_, err = repo.GetByOrderNo(ctx, "")
	if err != repository.ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}

	// Verify no event was published
	events := eventBus.GetEvents()
	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
}

// TestPayOrder tests the PayOrder use case
func TestPayOrder(t *testing.T) {
	repo := NewMockOrderRepository()
	eventBus := NewMockEventBus()
	service := NewOrderAppService(repo, eventBus, nil)

	ctx := context.Background()

	// First create an order
	createCmd := CreateOrderCommand{
		OrderNo:    "ORD002",
		UserID:     1001,
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		Quantity:   1,
		PayAmount:  10000,
		TraceID:    "trace-123",
	}

	err := service.CreateOrder(ctx, createCmd)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	eventBus.Clear()

	// Now pay for the order
	paidAt := time.Now()
	payCmd := PayOrderCommand{
		OrderNo:       "ORD002",
		TransactionNo: "TXN123",
		Amount:        10000,
		PaidAt:        paidAt,
	}

	err = service.PayOrder(ctx, payCmd)
	if err != nil {
		t.Fatalf("PayOrder failed: %v", err)
	}

	// Verify order was updated
	order, err := repo.GetByOrderNo(ctx, "ORD002")
	if err != nil {
		t.Fatalf("Failed to retrieve order: %v", err)
	}

	if order.Status != entity.OrderPaid {
		t.Errorf("Expected status %s, got %s", entity.OrderPaid, order.Status)
	}

	if order.TransactionNo != "TXN123" {
		t.Errorf("Expected transaction no TXN123, got %s", order.TransactionNo)
	}

	if order.PaidAt == nil {
		t.Fatal("Expected PaidAt to be set, got nil")
	}

	// Verify event was published
	events := eventBus.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
}

func TestPayOrderInvalidCommand(t *testing.T) {
	repo := NewMockOrderRepository()
	eventBus := NewMockEventBus()
	service := NewOrderAppService(repo, eventBus, nil)

	ctx := context.Background()

	// Create an order first
	createCmd := CreateOrderCommand{
		OrderNo:    "ORD003",
		UserID:     1001,
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		Quantity:   1,
		PayAmount:  10000,
		TraceID:    "trace-123",
	}

	err := service.CreateOrder(ctx, createCmd)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	eventBus.Clear()

	// Try to pay with invalid command
	payCmd := PayOrderCommand{
		OrderNo: "", // Invalid: empty order no
	}

	err = service.PayOrder(ctx, payCmd)
	if err == nil {
		t.Fatal("Expected error for invalid command, got nil")
	}

	// Verify no new event was published
	events := eventBus.GetEvents()
	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
}

func TestPayOrderAmountMismatch(t *testing.T) {
	repo := NewMockOrderRepository()
	eventBus := NewMockEventBus()
	service := NewOrderAppService(repo, eventBus, nil)

	ctx := context.Background()

	// Create an order
	createCmd := CreateOrderCommand{
		OrderNo:    "ORD004",
		UserID:     1001,
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		Quantity:   1,
		PayAmount:  10000,
		TraceID:    "trace-123",
	}

	err := service.CreateOrder(ctx, createCmd)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	eventBus.Clear()

	// Try to pay with wrong amount
	payCmd := PayOrderCommand{
		OrderNo:       "ORD004",
		TransactionNo: "TXN123",
		Amount:        5000, // Wrong amount
		PaidAt:        time.Now(),
	}

	err = service.PayOrder(ctx, payCmd)
	if err == nil {
		t.Fatal("Expected error for amount mismatch, got nil")
	}

	// Verify order status is still pending
	order, err := repo.GetByOrderNo(ctx, "ORD004")
	if err != nil {
		t.Fatalf("Failed to retrieve order: %v", err)
	}

	if order.Status != entity.OrderPending {
		t.Errorf("Expected status %s, got %s", entity.OrderPending, order.Status)
	}
}

func TestCloseOrder(t *testing.T) {
	repo := NewMockOrderRepository()
	eventBus := NewMockEventBus()
	service := NewOrderAppService(repo, eventBus, nil)

	ctx := context.Background()

	// Create an order
	createCmd := CreateOrderCommand{
		OrderNo:    "ORD005",
		UserID:     1001,
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		Quantity:   1,
		PayAmount:  10000,
		TraceID:    "trace-123",
	}

	err := service.CreateOrder(ctx, createCmd)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	eventBus.Clear()

	// Close the order
	closedAt := time.Now()
	closeCmd := CloseOrderCommand{
		OrderNo:  "ORD005",
		Reason:   "timeout",
		ClosedAt: closedAt,
	}

	err = service.CloseOrder(ctx, closeCmd)
	if err != nil {
		t.Fatalf("CloseOrder failed: %v", err)
	}

	// Verify order was closed
	order, err := repo.GetByOrderNo(ctx, "ORD005")
	if err != nil {
		t.Fatalf("Failed to retrieve order: %v", err)
	}

	if order.Status != entity.OrderClosed {
		t.Errorf("Expected status %s, got %s", entity.OrderClosed, order.Status)
	}

	if order.ClosedAt == nil {
		t.Fatal("Expected ClosedAt to be set, got nil")
	}

	// Verify event was published
	events := eventBus.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
}

func TestCloseOrderInvalidCommand(t *testing.T) {
	repo := NewMockOrderRepository()
	eventBus := NewMockEventBus()
	service := NewOrderAppService(repo, eventBus, nil)

	ctx := context.Background()

	// Create an order
	createCmd := CreateOrderCommand{
		OrderNo:    "ORD006",
		UserID:     1001,
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		Quantity:   1,
		PayAmount:  10000,
		TraceID:    "trace-123",
	}

	err := service.CreateOrder(ctx, createCmd)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	eventBus.Clear()

	// Try to close with invalid command
	closeCmd := CloseOrderCommand{
		OrderNo: "", // Invalid: empty order no
		Reason:  "timeout",
	}

	err = service.CloseOrder(ctx, closeCmd)
	if err == nil {
		t.Fatal("Expected error for invalid command, got nil")
	}

	// Verify no new event was published
	events := eventBus.GetEvents()
	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
}

func TestCloseOrderNotFound(t *testing.T) {
	repo := NewMockOrderRepository()
	eventBus := NewMockEventBus()
	service := NewOrderAppService(repo, eventBus, nil)

	ctx := context.Background()

	closeCmd := CloseOrderCommand{
		OrderNo:  "NONEXISTENT",
		Reason:   "timeout",
		ClosedAt: time.Now(),
	}

	err := service.CloseOrder(ctx, closeCmd)
	if err == nil {
		t.Fatal("Expected error for non-existent order, got nil")
	}

	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}
