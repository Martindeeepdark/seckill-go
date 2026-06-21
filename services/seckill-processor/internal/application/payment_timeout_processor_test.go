package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"seckill-processor-service/internal/application/usecase"
	"seckill-processor-service/internal/domain/model"
	"seckill-processor-service/internal/domain/service"
	"seckill-processor-service/internal/domain/status"

	commonerrors "seckill-common/errors"
)

// newTestPaymentTimeoutProcessor 创建测试用的 PaymentTimeoutProcessor
func newTestPaymentTimeoutProcessor(
	orders *timeoutFakeOrders,
	stock *timeoutFakeStock,
	payments *timeoutFakePayments,
) *PaymentTimeoutProcessor {
	uc := usecase.NewHandlePaymentTimeout(orders, stock, payments, slog.Default())
	return NewPaymentTimeoutProcessor(uc, nil, slog.Default())
}

// ---------------------------------------------------------------------------
// PaymentTimeoutProcessor table-driven tests
// ---------------------------------------------------------------------------

func TestPaymentTimeoutHandle(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		orders     *timeoutFakeOrders
		stock      *timeoutFakeStock
		payments   *timeoutFakePayments
		wantErr    bool
		wantRetry  bool // true => error is retriable (NOT terminal)
		wantTerm   bool // true => error is terminal (NOT retriable)
		assertFunc func(t *testing.T, o *timeoutFakeOrders, s *timeoutFakeStock, p *timeoutFakePayments)
	}{
		{
			name: "empty order no returns nil",
			orders: &timeoutFakeOrders{},
			stock:  &timeoutFakeStock{},
			payments: &timeoutFakePayments{},
			wantErr: false,
		},
		{
			name: "order not found via ErrNotFound returns nil",
			orders: &timeoutFakeOrders{getErr: service.ErrNotFound},
			stock:  &timeoutFakeStock{},
			payments: &timeoutFakePayments{},
			wantErr: false,
		},
		{
			name: "order not found via RPC NotFound returns nil",
			orders: &timeoutFakeOrders{getErr: grpcstatus.Error(codes.NotFound, "not found")},
			stock:  &timeoutFakeStock{},
			payments: &timeoutFakePayments{},
			wantErr: false,
		},
		{
			name: "order already closed returns nil",
			orders: &timeoutFakeOrders{order: model.OrderInfo{OrderNo: "O1", Status: status.OrderClosed}},
			stock:  &timeoutFakeStock{},
			payments: &timeoutFakePayments{},
			wantErr: false,
		},
		{
			name: "order already paid returns nil",
			orders: &timeoutFakeOrders{order: model.OrderInfo{OrderNo: "O1", Status: status.OrderPaid}},
			stock:  &timeoutFakeStock{},
			payments: &timeoutFakePayments{},
			wantErr: false,
		},
		{
			name:    "get order temporary RPC error returns error for retry",
			orders:  &timeoutFakeOrders{getErr: grpcstatus.Error(codes.Unavailable, "order svc down")},
			stock:   &timeoutFakeStock{},
			payments: &timeoutFakePayments{},
			wantErr: true,
			wantRetry: true,
		},
		{
			name:    "get order permanent RPC error returns error",
			orders:  &timeoutFakeOrders{getErr: grpcstatus.Error(codes.PermissionDenied, "denied")},
			stock:   &timeoutFakeStock{},
			payments: &timeoutFakePayments{},
			wantErr: true,
		},
		{
			name: "query payment shows already paid marks order paid",
			orders: &timeoutFakeOrders{order: model.OrderInfo{OrderNo: "O1", Status: status.OrderPending}},
			stock:  &timeoutFakeStock{},
			payments: &timeoutFakePayments{
				queryResult: model.PayQueryResult{
					OrderNo:       "O1",
					PayStatus:     status.PayStatusPaid,
					TransactionNo: "TX1",
					PaidAt:        &now,
				},
			},
			wantErr: false,
			assertFunc: func(t *testing.T, o *timeoutFakeOrders, _ *timeoutFakeStock, _ *timeoutFakePayments) {
				if !o.markedPaid {
					t.Fatal("expected order to be marked paid")
				}
			},
		},
		{
			name:    "query payment temporary error returns error for retry",
			orders:  &timeoutFakeOrders{order: model.OrderInfo{OrderNo: "O1", Status: status.OrderPending}},
			stock:   &timeoutFakeStock{},
			payments: &timeoutFakePayments{queryErr: grpcstatus.Error(codes.Unavailable, "payment svc down")},
			wantErr: true,
			wantRetry: true,
		},
		{
			name: "query payment not found proceeds to close order",
			orders: &timeoutFakeOrders{order: model.OrderInfo{
				OrderNo: "O1", Status: status.OrderPending,
				UserID: 7, ActivityNo: "A1", SKUNo: "S1", Quantity: 1,
			}},
			stock:    &timeoutFakeStock{},
			payments: &timeoutFakePayments{queryErr: service.ErrNotFound},
			wantErr:  false,
			assertFunc: func(t *testing.T, o *timeoutFakeOrders, s *timeoutFakeStock, _ *timeoutFakePayments) {
				if !o.closed {
					t.Fatal("expected order to be closed")
				}
				if !s.released {
					t.Fatal("expected stock to be released")
				}
			},
		},
		{
			name: "query payment RPC not found proceeds to close order",
			orders: &timeoutFakeOrders{order: model.OrderInfo{
				OrderNo: "O1", Status: status.OrderPending,
				UserID: 7, ActivityNo: "A1", SKUNo: "S1", Quantity: 1,
			}},
			stock:    &timeoutFakeStock{},
			payments: &timeoutFakePayments{queryErr: grpcstatus.Error(codes.NotFound, "payment not found")},
			wantErr:  false,
			assertFunc: func(t *testing.T, o *timeoutFakeOrders, s *timeoutFakeStock, _ *timeoutFakePayments) {
				if !o.closed {
					t.Fatal("expected order to be closed")
				}
				if !s.released {
					t.Fatal("expected stock to be released")
				}
			},
		},
		{
			name: "close order invalid state returns nil",
			orders: &timeoutFakeOrders{
				order:    model.OrderInfo{OrderNo: "O1", Status: status.OrderPending},
				closeErr: grpcstatus.Error(codes.FailedPrecondition, "already closed"),
			},
			stock:    &timeoutFakeStock{},
			payments: &timeoutFakePayments{queryErr: service.ErrNotFound},
			wantErr:  false,
		},
		{
			name: "close order invalid state via ErrInvalidState returns nil",
			orders: &timeoutFakeOrders{
				order:    model.OrderInfo{OrderNo: "O1", Status: status.OrderPending},
				closeErr: usecase.ErrInvalidState,
			},
			stock:    &timeoutFakeStock{},
			payments: &timeoutFakePayments{queryErr: service.ErrNotFound},
			wantErr:  false,
		},
		{
			name:    "close order temporary error returns error for retry",
			orders:  &timeoutFakeOrders{
				order:    model.OrderInfo{OrderNo: "O1", Status: status.OrderPending},
				closeErr: grpcstatus.Error(codes.Unavailable, "order svc down"),
			},
			stock:    &timeoutFakeStock{},
			payments: &timeoutFakePayments{queryErr: service.ErrNotFound},
			wantErr:  true,
		},
		{
			name:    "release stock temporary error returns error for retry",
			orders:  &timeoutFakeOrders{
				order: model.OrderInfo{
					OrderNo: "O1", Status: status.OrderPending,
					UserID: 7, ActivityNo: "A1", SKUNo: "S1", Quantity: 1,
				},
				closedOrder: model.OrderInfo{OrderNo: "O1", Status: status.OrderClosed},
			},
			stock:    &timeoutFakeStock{releaseErr: grpcstatus.Error(codes.Unavailable, "stock svc down")},
			payments: &timeoutFakePayments{queryErr: service.ErrNotFound},
			wantErr:  true,
			wantRetry: true,
		},
		{
			name:    "release stock permanent error returns error (not terminal by default)",
			orders:  &timeoutFakeOrders{
				order: model.OrderInfo{
					OrderNo: "O1", Status: status.OrderPending,
					UserID: 7, ActivityNo: "A1", SKUNo: "S1", Quantity: 1,
				},
				closedOrder: model.OrderInfo{OrderNo: "O1", Status: status.OrderClosed},
			},
			stock:    &timeoutFakeStock{releaseErr: grpcstatus.Error(codes.PermissionDenied, "forbidden")},
			payments: &timeoutFakePayments{queryErr: service.ErrNotFound},
			wantErr:  true,
		},
		{
			name: "happy path closes order releases stock closes payment",
			orders: &timeoutFakeOrders{
				order: model.OrderInfo{
					OrderNo: "O1", Status: status.OrderPending,
					UserID: 7, ActivityNo: "A1", SKUNo: "S1", Quantity: 1,
				},
				closedOrder: model.OrderInfo{OrderNo: "O1", Status: status.OrderClosed},
			},
			stock:    &timeoutFakeStock{},
			payments: &timeoutFakePayments{queryErr: service.ErrNotFound},
			wantErr:  false,
			assertFunc: func(t *testing.T, o *timeoutFakeOrders, s *timeoutFakeStock, p *timeoutFakePayments) {
				if !o.closed {
					t.Fatal("expected order to be closed")
				}
				if !s.released {
					t.Fatal("expected stock to be released")
				}
				if !p.paymentClosed {
					t.Fatal("expected payment to be closed")
				}
			},
		},
		{
			name: "close payment not found is ignored",
			orders: &timeoutFakeOrders{
				order: model.OrderInfo{
					OrderNo: "O1", Status: status.OrderPending,
					UserID: 7, ActivityNo: "A1", SKUNo: "S1", Quantity: 1,
				},
				closedOrder: model.OrderInfo{OrderNo: "O1", Status: status.OrderClosed},
			},
			stock:    &timeoutFakeStock{},
			payments: &timeoutFakePayments{
				queryErr:         service.ErrNotFound,
				closePaymentErr:  grpcstatus.Error(codes.NotFound, "payment not found"),
			},
			wantErr: false,
			assertFunc: func(t *testing.T, o *timeoutFakeOrders, s *timeoutFakeStock, _ *timeoutFakePayments) {
				if !o.closed {
					t.Fatal("expected order to be closed")
				}
				if !s.released {
					t.Fatal("expected stock to be released")
				}
			},
		},
		{
			name:    "close payment temporary error returns error for retry",
			orders:  &timeoutFakeOrders{
				order: model.OrderInfo{
					OrderNo: "O1", Status: status.OrderPending,
					UserID: 7, ActivityNo: "A1", SKUNo: "S1", Quantity: 1,
				},
				closedOrder: model.OrderInfo{OrderNo: "O1", Status: status.OrderClosed},
			},
			stock:    &timeoutFakeStock{},
			payments: &timeoutFakePayments{
				queryErr:         service.ErrNotFound,
				closePaymentErr:  grpcstatus.Error(codes.Unavailable, "payment svc down"),
			},
			wantErr: true,
		},
		{
			name: "query payment non-temporary non-not-found error returns error",
			orders: &timeoutFakeOrders{order: model.OrderInfo{OrderNo: "O1", Status: status.OrderPending}},
			stock:  &timeoutFakeStock{},
			payments: &timeoutFakePayments{queryErr: grpcstatus.Error(codes.Internal, "internal error")},
			wantErr: true,
		},
		{
			name: "verify closed order not actually closed returns nil",
			orders: &timeoutFakeOrders{
				order: model.OrderInfo{
					OrderNo: "O1", Status: status.OrderPending,
					UserID: 7, ActivityNo: "A1", SKUNo: "S1", Quantity: 1,
				},
				// After closing, GetOrder still returns OrderPending
				closedOrder: model.OrderInfo{OrderNo: "O1", Status: status.OrderPending},
			},
			stock:    &timeoutFakeStock{},
			payments: &timeoutFakePayments{queryErr: service.ErrNotFound},
			wantErr:  false,
			assertFunc: func(t *testing.T, o *timeoutFakeOrders, s *timeoutFakeStock, _ *timeoutFakePayments) {
				if !o.closed {
					t.Fatal("expected CloseOrder to be called")
				}
				if s.released {
					t.Fatal("expected stock NOT to be released because order not actually closed")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestPaymentTimeoutProcessor(tt.orders, tt.stock, tt.payments)
			err := p.Handle(context.Background(), model.PaymentTimeoutTask{OrderNo: "O1"})

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantRetry && err != nil {
				// The error should NOT be terminal (it should be retriable)
				if commonerrors.IsTerminal(err) {
					t.Fatalf("expected retriable (non-terminal) error, got terminal: %v", err)
				}
			}

			if tt.wantTerm && err != nil {
				if !commonerrors.IsTerminal(err) {
					t.Fatalf("expected terminal error, got non-terminal: %v", err)
				}
			}

			if tt.assertFunc != nil {
				tt.assertFunc(t, tt.orders, tt.stock, tt.payments)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Existing individual tests kept for backwards compatibility
// ---------------------------------------------------------------------------

func TestPaymentTimeoutHandle_QueryTemporaryError(t *testing.T) {
	p := newTestPaymentTimeoutProcessor(
		&timeoutFakeOrders{order: model.OrderInfo{OrderNo: "O1", Status: status.OrderPending}},
		&timeoutFakeStock{},
		&timeoutFakePayments{queryErr: grpcstatus.Error(codes.Unavailable, "payment service down")},
	)
	err := p.Handle(context.Background(), model.PaymentTimeoutTask{OrderNo: "O1"})
	if err == nil {
		t.Fatal("expected error for temporary RPC failure, got nil")
	}
	if !commonerrors.IsTemporaryRPCError(errors.Unwrap(err)) {
		t.Fatalf("expected temporary RPC error, got: %v", err)
	}
}

func TestPaymentTimeoutHandle_QueryPaid(t *testing.T) {
	now := time.Now()
	orders := &timeoutFakeOrders{order: model.OrderInfo{OrderNo: "O1", Status: status.OrderPending}}
	p := newTestPaymentTimeoutProcessor(
		orders,
		&timeoutFakeStock{},
		&timeoutFakePayments{queryResult: model.PayQueryResult{
			OrderNo:       "O1",
			PayStatus:     status.PayStatusPaid,
			TransactionNo: "TX1",
			PaidAt:        &now,
		}},
	)
	err := p.Handle(context.Background(), model.PaymentTimeoutTask{OrderNo: "O1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !orders.markedPaid {
		t.Fatal("expected order to be marked paid")
	}
}

func TestPaymentTimeoutHandle_QueryNotFound_ClosesOrder(t *testing.T) {
	orders := &timeoutFakeOrders{order: model.OrderInfo{
		OrderNo: "O1", Status: status.OrderPending,
		UserID: 7, ActivityNo: "A1", SKUNo: "S1", Quantity: 1,
	}}
	stock := &timeoutFakeStock{}
	p := newTestPaymentTimeoutProcessor(
		orders,
		stock,
		&timeoutFakePayments{queryErr: service.ErrNotFound},
	)
	err := p.Handle(context.Background(), model.PaymentTimeoutTask{OrderNo: "O1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !orders.closed {
		t.Fatal("expected order to be closed")
	}
	if !stock.released {
		t.Fatal("expected stock to be released")
	}
}

func TestPaymentTimeoutHandle_OrderAlreadyPaid(t *testing.T) {
	p := newTestPaymentTimeoutProcessor(
		&timeoutFakeOrders{order: model.OrderInfo{OrderNo: "O1", Status: status.OrderPaid}},
		&timeoutFakeStock{},
		&timeoutFakePayments{},
	)
	err := p.Handle(context.Background(), model.PaymentTimeoutTask{OrderNo: "O1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPaymentTimeoutHandle_OrderNotFound(t *testing.T) {
	p := newTestPaymentTimeoutProcessor(
		&timeoutFakeOrders{getErr: service.ErrNotFound},
		&timeoutFakeStock{},
		&timeoutFakePayments{},
	)
	err := p.Handle(context.Background(), model.PaymentTimeoutTask{OrderNo: "O1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPaymentTimeoutHandle_EmptyOrderNo(t *testing.T) {
	uc := usecase.NewHandlePaymentTimeout(nil, nil, nil, slog.Default())
	p := NewPaymentTimeoutProcessor(uc, nil, slog.Default())
	err := p.Handle(context.Background(), model.PaymentTimeoutTask{OrderNo: "  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPaymentTimeoutHandle_CloseOrderInvalidState(t *testing.T) {
	orders := &timeoutFakeOrders{
		order:     model.OrderInfo{OrderNo: "O1", Status: status.OrderPending},
		closeErr:  grpcstatus.Error(codes.FailedPrecondition, "already closed"),
	}
	p := newTestPaymentTimeoutProcessor(
		orders,
		&timeoutFakeStock{},
		&timeoutFakePayments{queryErr: service.ErrNotFound},
	)
	err := p.Handle(context.Background(), model.PaymentTimeoutTask{OrderNo: "O1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if orders.closed {
		t.Fatal("close should not have succeeded")
	}
}

func TestPaymentTimeoutHandle_ClosePaymentTemporaryError(t *testing.T) {
	orders := &timeoutFakeOrders{
		order: model.OrderInfo{
			OrderNo: "O1", Status: status.OrderPending,
			UserID: 7, ActivityNo: "A1", SKUNo: "S1", Quantity: 1,
		},
		closedOrder: model.OrderInfo{OrderNo: "O1", Status: status.OrderClosed},
	}
	p := newTestPaymentTimeoutProcessor(
		orders,
		&timeoutFakeStock{},
		&timeoutFakePayments{
			queryErr:      service.ErrNotFound,
			closePaymentErr: grpcstatus.Error(codes.Unavailable, "payment svc down"),
		},
	)
	err := p.Handle(context.Background(), model.PaymentTimeoutTask{OrderNo: "O1"})
	if err == nil {
		t.Fatal("expected error for close payment temporary failure")
	}
}

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type timeoutFakeOrders struct {
	order       model.OrderInfo
	closedOrder model.OrderInfo
	getErr      error
	closeErr    error
	closed      bool
	markedPaid  bool
}

func (o *timeoutFakeOrders) CreateOrder(context.Context, model.OrderRequest) error {
	return nil
}

// GetByUserAndTrace satisfies the extended OrderCreator interface; not used in payment timeout tests
func (o *timeoutFakeOrders) GetByUserAndTrace(context.Context, int64, string) (model.OrderInfo, error) {
	return model.OrderInfo{}, errors.New("not implemented in timeoutFakeOrders")
}

func (o *timeoutFakeOrders) GetOrder(_ context.Context, orderNo string) (model.OrderInfo, error) {
	if o.getErr != nil {
		return model.OrderInfo{}, o.getErr
	}
	if o.closed {
		if o.closedOrder.OrderNo != "" {
			return o.closedOrder, nil
		}
		return model.OrderInfo{OrderNo: orderNo, Status: status.OrderClosed}, nil
	}
	return o.order, nil
}

func (o *timeoutFakeOrders) MarkOrderPaid(_ context.Context, _, _ string, _ time.Time) error {
	o.markedPaid = true
	return nil
}

func (o *timeoutFakeOrders) CloseOrder(_ context.Context, _ string) error {
	if o.closeErr != nil {
		return o.closeErr
	}
	o.closed = true
	return nil
}

type timeoutFakeStock struct {
	released   bool
	releaseErr error
}

func (s *timeoutFakeStock) DeductStockWithLimit(context.Context, string, string, int64, int64, int64, string) (bool, error) {
	return true, nil
}

func (s *timeoutFakeStock) ReleaseStock(context.Context, string, string, int64, int64, string) error {
	if s.releaseErr != nil {
		return s.releaseErr
	}
	s.released = true
	return nil
}

type timeoutFakePayments struct {
	queryResult     model.PayQueryResult
	queryErr        error
	closePaymentErr error
	paymentClosed   bool
}

func (p *timeoutFakePayments) QueryPayment(context.Context, string) (model.PayQueryResult, error) {
	return p.queryResult, p.queryErr
}

func (p *timeoutFakePayments) ClosePayment(context.Context, string) error {
	p.paymentClosed = true
	return p.closePaymentErr
}
