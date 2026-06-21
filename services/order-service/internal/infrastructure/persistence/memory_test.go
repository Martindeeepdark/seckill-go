package persistence

import (
	"context"
	"testing"
	"time"

	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/domain/status"
)

func TestMemoryStore_CreateOrder(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	order := entity.Order{
		OrderNo:   "ORD001",
		UserID:    1,
		ActivityNo: "ACT001",
		SKUNo:     "SKU001",
		Quantity:  1,
		PayAmount: 100,
		Status:    status.OrderPending,
		TraceID:   "TRACE001",
		CreatedAt: time.Now(),
	}
	if err := s.CreateOrder(ctx, order); err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if err := s.CreateOrder(ctx, order); err != ErrDuplicate {
		t.Fatalf("duplicate CreateOrder: got %v, want ErrDuplicate", err)
	}
}

func TestMemoryStore_GetOrder(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	_, err := s.GetOrder(ctx, "MISSING")
	if err != ErrNotFound {
		t.Fatalf("GetOrder missing: got %v, want ErrNotFound", err)
	}
	order := entity.Order{OrderNo: "ORD001", UserID: 1, ActivityNo: "A1", SKUNo: "S1", Quantity: 1, PayAmount: 100, Status: status.OrderPending, TraceID: "T1", CreatedAt: time.Now()}
	_ = s.CreateOrder(ctx, order)
	got, err := s.GetOrder(ctx, "ORD001")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if got.OrderNo != "ORD001" {
		t.Fatalf("GetOrder OrderNo: got %s, want ORD001", got.OrderNo)
	}
}

func TestMemoryStore_MarkOrderPaid(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	order := entity.Order{OrderNo: "ORD001", UserID: 1, ActivityNo: "A1", SKUNo: "S1", Quantity: 1, PayAmount: 100, Status: status.OrderPending, TraceID: "T1", CreatedAt: time.Now()}
	_ = s.CreateOrder(ctx, order)
	paidAt := time.Now()
	if err := s.MarkOrderPaid(ctx, "ORD001", "TX001", paidAt); err != nil {
		t.Fatalf("MarkOrderPaid: %v", err)
	}
	got, _ := s.GetOrder(ctx, "ORD001")
	if got.Status != status.OrderPaid {
		t.Fatalf("status: got %s, want PAID", got.Status)
	}
	if got.TransactionNo != "TX001" {
		t.Fatalf("transactionNo: got %s, want TX001", got.TransactionNo)
	}
}

func TestMemoryStore_CloseOrder(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	order := entity.Order{OrderNo: "ORD001", UserID: 1, ActivityNo: "A1", SKUNo: "S1", Quantity: 1, PayAmount: 100, Status: status.OrderPending, TraceID: "T1", CreatedAt: time.Now()}
	_ = s.CreateOrder(ctx, order)
	if err := s.CloseOrder(ctx, "ORD001"); err != nil {
		t.Fatalf("CloseOrder: %v", err)
	}
	got, _ := s.GetOrder(ctx, "ORD001")
	if got.Status != status.OrderClosed {
		t.Fatalf("status: got %s, want CLOSED", got.Status)
	}
}

func TestMemoryStore_ListOrdersByActivity(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	now := time.Now()
	_ = s.CreateOrder(ctx, entity.Order{OrderNo: "ORD001", UserID: 1, ActivityNo: "A1", SKUNo: "S1", Quantity: 1, PayAmount: 100, Status: status.OrderPending, TraceID: "T1", CreatedAt: now})
	_ = s.CreateOrder(ctx, entity.Order{OrderNo: "ORD002", UserID: 2, ActivityNo: "A2", SKUNo: "S1", Quantity: 1, PayAmount: 200, Status: status.OrderPending, TraceID: "T2", CreatedAt: now})
	orders, err := s.ListOrdersByActivity(ctx, "A1")
	if err != nil {
		t.Fatalf("ListOrdersByActivity: %v", err)
	}
	if len(orders) != 1 || orders[0].OrderNo != "ORD001" {
		t.Fatalf("ListOrdersByActivity: got %v", orders)
	}
}

func TestMemoryStore_ListOrdersByUser(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	now := time.Now()
	_ = s.CreateOrder(ctx, entity.Order{OrderNo: "ORD001", UserID: 1, ActivityNo: "A1", SKUNo: "S1", Quantity: 1, PayAmount: 100, Status: status.OrderPending, TraceID: "T1", CreatedAt: now})
	_ = s.CreateOrder(ctx, entity.Order{OrderNo: "ORD002", UserID: 2, ActivityNo: "A2", SKUNo: "S1", Quantity: 1, PayAmount: 200, Status: status.OrderPending, TraceID: "T2", CreatedAt: now})
	orders, err := s.ListOrdersByUser(ctx, 1)
	if err != nil {
		t.Fatalf("ListOrdersByUser: %v", err)
	}
	if len(orders) != 1 || orders[0].OrderNo != "ORD001" {
		t.Fatalf("ListOrdersByUser: got %v", orders)
	}
}

func TestMemoryStore_ListOrdersByActivities(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	now := time.Now()
	_ = s.CreateOrder(ctx, entity.Order{OrderNo: "ORD001", UserID: 1, ActivityNo: "A1", SKUNo: "S1", Quantity: 1, PayAmount: 100, Status: status.OrderPending, TraceID: "T1", CreatedAt: now})
	_ = s.CreateOrder(ctx, entity.Order{OrderNo: "ORD002", UserID: 2, ActivityNo: "A2", SKUNo: "S1", Quantity: 1, PayAmount: 200, Status: status.OrderPending, TraceID: "T2", CreatedAt: now})
	result, err := s.ListOrdersByActivities(ctx, []string{"A1", "A2"})
	if err != nil {
		t.Fatalf("ListOrdersByActivities: %v", err)
	}
	if len(result["A1"]) != 1 || len(result["A2"]) != 1 {
		t.Fatalf("ListOrdersByActivities: got %v", result)
	}
}
