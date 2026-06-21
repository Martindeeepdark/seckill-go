// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"seckill-common/eventbus/local"
	"seckill-order-service/internal/application"
	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/infrastructure/persistence"
)

func TestOrderLifecycle_CreateAndPay(t *testing.T) {
	// Setup
	repo := persistence.NewMemoryOrderRepository()
	bus := local.NewLocalBus()
	svc := application.NewOrderAppService(repo, bus, nil)

	ctx := context.Background()

	// 1. 创建订单
	createCmd := application.CreateOrderCommand{
		OrderNo:    "ORD-TEST-001",
		UserID:     12345,
		ActivityNo: "ACT-001",
		SKUNo:      "SKU-001",
		Quantity:   1,
		PayAmount:  10000,
		TraceID:    "trace-001",
	}

	err := svc.CreateOrder(ctx, createCmd)
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// 验证订单状态
	order, err := repo.GetByOrderNo(ctx, "ORD-TEST-001")
	if err != nil {
		t.Fatalf("GetOrder failed: %v", err)
	}
	if order.Status != entity.OrderPending {
		t.Errorf("expected status PENDING_PAY, got %s", order.Status)
	}

	// 2. 支付订单
	payCmd := application.PayOrderCommand{
		OrderNo:       "ORD-TEST-001",
		TransactionNo: "TXN-001",
		Amount:        10000,
		PaidAt:        time.Now(),
	}

	err = svc.PayOrder(ctx, payCmd)
	if err != nil {
		t.Fatalf("PayOrder failed: %v", err)
	}

	// 验证订单状态已更新
	order, err = repo.GetByOrderNo(ctx, "ORD-TEST-001")
	if err != nil {
		t.Fatalf("GetOrder failed: %v", err)
	}
	if order.Status != entity.OrderPaid {
		t.Errorf("expected status PAID, got %s", order.Status)
	}
	if order.TransactionNo != "TXN-001" {
		t.Errorf("expected transaction no TXN-001, got %s", order.TransactionNo)
	}
}

func TestOrderLifecycle_CreateAndClose(t *testing.T) {
	// Setup
	repo := persistence.NewMemoryOrderRepository()
	bus := local.NewLocalBus()
	svc := application.NewOrderAppService(repo, bus, nil)

	ctx := context.Background()

	// 1. 创建订单
	createCmd := application.CreateOrderCommand{
		OrderNo:    "ORD-TEST-002",
		UserID:     12345,
		ActivityNo: "ACT-001",
		SKUNo:      "SKU-001",
		Quantity:   1,
		PayAmount:  10000,
		TraceID:    "trace-002",
	}

	err := svc.CreateOrder(ctx, createCmd)
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// 2. 关闭订单
	closeCmd := application.CloseOrderCommand{
		OrderNo:  "ORD-TEST-002",
		Reason:   "payment_timeout",
		ClosedAt: time.Now(),
	}

	err = svc.CloseOrder(ctx, closeCmd)
	if err != nil {
		t.Fatalf("CloseOrder failed: %v", err)
	}

	// 验证订单状态
	order, err := repo.GetByOrderNo(ctx, "ORD-TEST-002")
	if err != nil {
		t.Fatalf("GetOrder failed: %v", err)
	}
	if order.Status != entity.OrderClosed {
		t.Errorf("expected status CLOSED, got %s", order.Status)
	}
}
