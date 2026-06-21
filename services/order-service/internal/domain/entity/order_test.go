package entity

import (
	"testing"
	"time"

	"seckill-order-service/internal/domain/event"
)

func TestCreateOrder(t *testing.T) {
	tests := []struct {
		name        string
		orderNo     string
		userID      int64
		activityNo  string
		skuNo       string
		quantity    int64
		payAmount   int64
		traceID     string
		wantErr     bool
		errMsg      string
	}{
		{
			name:       "valid order creation",
			orderNo:    "ORD001",
			userID:     1001,
			activityNo: "ACT001",
			skuNo:      "SKU001",
			quantity:   2,
			payAmount:  20000,
			traceID:    "trace-123",
			wantErr:    false,
		},
		{
			name:       "empty order number",
			orderNo:    "",
			userID:     1001,
			activityNo: "ACT001",
			skuNo:      "SKU001",
			quantity:   2,
			payAmount:  20000,
			traceID:    "trace-123",
			wantErr:    true,
			errMsg:     "order number cannot be empty",
		},
		{
			name:       "zero user ID",
			orderNo:    "ORD001",
			userID:     0,
			activityNo: "ACT001",
			skuNo:      "SKU001",
			quantity:   2,
			payAmount:  20000,
			traceID:    "trace-123",
			wantErr:    true,
			errMsg:     "user ID cannot be zero",
		},
		{
			name:       "zero quantity",
			orderNo:    "ORD001",
			userID:     1001,
			activityNo: "ACT001",
			skuNo:      "SKU001",
			quantity:   0,
			payAmount:  20000,
			traceID:    "trace-123",
			wantErr:    true,
			errMsg:     "quantity must be positive",
		},
		{
			name:       "negative pay amount",
			orderNo:    "ORD001",
			userID:     1001,
			activityNo: "ACT001",
			skuNo:      "SKU001",
			quantity:   2,
			payAmount:  -100,
			traceID:    "trace-123",
			wantErr:    true,
			errMsg:     "pay amount must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := CreateOrder(tt.orderNo, tt.userID, tt.activityNo, tt.skuNo, tt.quantity, tt.payAmount, tt.traceID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateOrder() expected error but got none")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("CreateOrder() error message = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateOrder() unexpected error: %v", err)
				return
			}

			if order == nil {
				t.Errorf("CreateOrder() returned nil order")
				return
			}

			// Verify basic fields
			if order.OrderNo != tt.orderNo {
				t.Errorf("CreateOrder() OrderNo = %v, want %v", order.OrderNo, tt.orderNo)
			}
			if order.UserID != tt.userID {
				t.Errorf("CreateOrder() UserID = %v, want %v", order.UserID, tt.userID)
			}
			if order.ActivityNo != tt.activityNo {
				t.Errorf("CreateOrder() ActivityNo = %v, want %v", order.ActivityNo, tt.activityNo)
			}
			if order.SKUNo != tt.skuNo {
				t.Errorf("CreateOrder() SKUNo = %v, want %v", order.SKUNo, tt.skuNo)
			}
			if order.Quantity != tt.quantity {
				t.Errorf("CreateOrder() Quantity = %v, want %v", order.Quantity, tt.quantity)
			}
			if order.PayAmount != tt.payAmount {
				t.Errorf("CreateOrder() PayAmount = %v, want %v", order.PayAmount, tt.payAmount)
			}
			if order.TraceID != tt.traceID {
				t.Errorf("CreateOrder() TraceID = %v, want %v", order.TraceID, tt.traceID)
			}

			// Verify initial status
			if order.Status != OrderPending {
				t.Errorf("CreateOrder() Status = %v, want %v", order.Status, OrderPending)
			}

			// Verify timestamps
			if order.CreatedAt.IsZero() {
				t.Errorf("CreateOrder() CreatedAt should be set")
			}
			if order.PaidAt != nil {
				t.Errorf("CreateOrder() PaidAt should be nil initially")
			}
			if order.ClosedAt != nil {
				t.Errorf("CreateOrder() ClosedAt should be nil initially")
			}

			// Verify domain event was recorded
			events := order.GetUncommittedEvents()
			if len(events) != 1 {
				t.Errorf("CreateOrder() expected 1 event, got %d", len(events))
				return
			}

			orderCreatedEvent, ok := events[0].(*event.OrderCreatedEvent)
			if !ok {
				t.Errorf("CreateOrder() expected OrderCreatedEvent, got %T", events[0])
				return
			}

			if orderCreatedEvent.OrderNo != tt.orderNo {
				t.Errorf("OrderCreatedEvent.OrderNo = %v, want %v", orderCreatedEvent.OrderNo, tt.orderNo)
			}
		})
	}
}

func TestMarkAsPaid(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		setupOrder    func() *Order
		transactionNo string
		amount        int64
		paidAt        time.Time
		wantErr       bool
		errMsg        string
		verifyEvent   bool
	}{
		{
			name: "valid payment",
			setupOrder: func() *Order {
				order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
				return order
			},
			transactionNo: "TXN001",
			amount:        20000,
			paidAt:        now,
			wantErr:       false,
			verifyEvent:   true,
		},
		{
			name: "already paid",
			setupOrder: func() *Order {
				order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
				_ = order.MarkAsPaid("TXN001", 20000, now)
				return order
			},
			transactionNo: "TXN002",
			amount:        20000,
			paidAt:        now,
			wantErr:       true,
			errMsg:        "order ORD001: order is already paid",
		},
		{
			name: "amount mismatch",
			setupOrder: func() *Order {
				order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
				return order
			},
			transactionNo: "TXN001",
			amount:        15000,
			paidAt:        now,
			wantErr:       true,
			errMsg:        "payment amount 15000 does not match order amount 20000: payment amount does not match order amount",
		},
		{
			name: "close cannot mark as paid",
			setupOrder: func() *Order {
				order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
				_ = order.Close(now)
				return order
			},
			transactionNo: "TXN001",
			amount:        20000,
			paidAt:        now,
			wantErr:       true,
			errMsg:        "order ORD001: order is already closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := tt.setupOrder()
			initialEventCount := len(order.GetUncommittedEvents())

			err := order.MarkAsPaid(tt.transactionNo, tt.amount, tt.paidAt)

			if tt.wantErr {
				if err == nil {
					t.Errorf("MarkAsPaid() expected error but got none")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("MarkAsPaid() error message = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("MarkAsPaid() unexpected error: %v", err)
				return
			}

			// Verify status changed
			if order.Status != OrderPaid {
				t.Errorf("MarkAsPaid() Status = %v, want %v", order.Status, OrderPaid)
			}

			// Verify transaction number set
			if order.TransactionNo != tt.transactionNo {
				t.Errorf("MarkAsPaid() TransactionNo = %v, want %v", order.TransactionNo, tt.transactionNo)
			}

			// Verify paid time set
			if order.PaidAt == nil {
				t.Errorf("MarkAsPaid() PaidAt should be set")
				return
			}
			if !order.PaidAt.Equal(tt.paidAt) {
				t.Errorf("MarkAsPaid() PaidAt = %v, want %v", order.PaidAt, tt.paidAt)
			}

			// Verify domain event was recorded
			events := order.GetUncommittedEvents()
			if len(events) != initialEventCount+1 {
				t.Errorf("MarkAsPaid() expected %d events, got %d", initialEventCount+1, len(events))
				return
			}

			if tt.verifyEvent {
				orderPaidEvent, ok := events[len(events)-1].(*event.OrderPaidEvent)
				if !ok {
					t.Errorf("MarkAsPaid() expected OrderPaidEvent, got %T", events[len(events)-1])
					return
				}

				if orderPaidEvent.TransactionNo != tt.transactionNo {
					t.Errorf("OrderPaidEvent.TransactionNo = %v, want %v", orderPaidEvent.TransactionNo, tt.transactionNo)
				}
				if orderPaidEvent.Amount != tt.amount {
					t.Errorf("OrderPaidEvent.Amount = %v, want %v", orderPaidEvent.Amount, tt.amount)
				}
			}
		})
	}
}

func TestClose(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		setupOrder    func() *Order
		closedAt      time.Time
		wantErr       bool
		errMsg        string
		verifyStatus  bool
		expectedStatus string
		verifyEvent   bool
	}{
		{
			name: "close pending order",
			setupOrder: func() *Order {
				order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
				return order
			},
			closedAt:      now,
			wantErr:       false,
			verifyStatus:  true,
			expectedStatus: OrderClosed,
			verifyEvent:   true,
		},
		{
			name: "cannot close paid order",
			setupOrder: func() *Order {
				order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
				_ = order.MarkAsPaid("TXN001", 20000, now)
				return order
			},
			closedAt:      now,
			wantErr:       true,
			errMsg:        "paid order ORD001: paid order cannot be closed",
		},
		{
			name: "cannot close already closed order",
			setupOrder: func() *Order {
				order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
				_ = order.Close(now)
				return order
			},
			closedAt:      now,
			wantErr:       true,
			errMsg:        "order ORD001: order is already closed",
		},
		{
			name: "cannot close refunded order",
			setupOrder: func() *Order {
				order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
				order.Status = OrderRefund // Simulate refunded status
				return order
			},
			closedAt:      now,
			wantErr:       true,
			errMsg:        "refunded order ORD001: refunded order cannot be closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := tt.setupOrder()
			initialEventCount := len(order.GetUncommittedEvents())

			err := order.Close(tt.closedAt)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Close() expected error but got none")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Close() error message = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Close() unexpected error: %v", err)
				return
			}

			// Verify status changed
			if tt.verifyStatus && order.Status != tt.expectedStatus {
				t.Errorf("Close() Status = %v, want %v", order.Status, tt.expectedStatus)
			}

			// Verify closed time set
			if order.ClosedAt == nil {
				t.Errorf("Close() ClosedAt should be set")
				return
			}
			if !order.ClosedAt.Equal(tt.closedAt) {
				t.Errorf("Close() ClosedAt = %v, want %v", order.ClosedAt, tt.closedAt)
			}

			// Verify domain event was recorded
			events := order.GetUncommittedEvents()
			if len(events) != initialEventCount+1 {
				t.Errorf("Close() expected %d events, got %d", initialEventCount+1, len(events))
				return
			}

			if tt.verifyEvent {
				orderClosedEvent, ok := events[len(events)-1].(*event.OrderClosedEvent)
				if !ok {
					t.Errorf("Close() expected OrderClosedEvent, got %T", events[len(events)-1])
					return
				}

				if orderClosedEvent.OrderNo != order.OrderNo {
					t.Errorf("OrderClosedEvent.OrderNo = %v, want %v", orderClosedEvent.OrderNo, order.OrderNo)
				}
			}
		})
	}
}

func TestHelperMethods(t *testing.T) {
	t.Run("IsPending", func(t *testing.T) {
		order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
		if !order.IsPending() {
			t.Errorf("IsPending() = false, want true")
		}

		order.Status = OrderPaid
		if order.IsPending() {
			t.Errorf("IsPending() = true, want false")
		}
	})

	t.Run("IsPaid", func(t *testing.T) {
		order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
		if order.IsPaid() {
			t.Errorf("IsPaid() = true, want false")
		}

		_ = order.MarkAsPaid("TXN001", 20000, time.Now())
		if !order.IsPaid() {
			t.Errorf("IsPaid() = false, want true")
		}
	})

	t.Run("CanBeReconciled", func(t *testing.T) {
		tests := []struct {
			name     string
			status   string
			expected bool
		}{
			{"pending status", OrderPending, true},
			{"paid status", OrderPaid, true},
			{"closed status", OrderClosed, false},
			{"refunded status", OrderRefund, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
				order.Status = tt.status
				if order.CanBeReconciled() != tt.expected {
					t.Errorf("CanBeReconciled() = %v, want %v", order.CanBeReconciled(), tt.expected)
				}
			})
		}
	})
}

func TestGetUncommittedEvents(t *testing.T) {
	t.Run("get domain events", func(t *testing.T) {
		order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
		events := order.GetUncommittedEvents()

		if len(events) != 1 {
			t.Errorf("GetUncommittedEvents() = %d events, want 1", len(events))
		}

		_, ok := events[0].(*event.OrderCreatedEvent)
		if !ok {
			t.Errorf("GetUncommittedEvents()[0] type = %T, want *event.OrderCreatedEvent", events[0])
		}
	})

	t.Run("clear domain events", func(t *testing.T) {
		order, _ := CreateOrder("ORD001", 1001, "ACT001", "SKU001", 2, 20000, "trace-123")
		order.ClearEvents()

		events := order.GetUncommittedEvents()
		if len(events) != 0 {
			t.Errorf("ClearEvents() = %d events, want 0", len(events))
		}
	})
}
