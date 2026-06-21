package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"seckill-order-service/internal/domain/entity"
)

func TestIsDuplicate_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "unique_violation 23505 returns true",
			err:  &pgconn.PgError{Code: "23505", Message: "duplicate key"},
			want: true,
		},
		{
			name: "foreign_key_violation 23503 returns false",
			err:  &pgconn.PgError{Code: "23503", Message: "fk violation"},
			want: false,
		},
		{
			name: "not_null_violation 23502 returns false",
			err:  &pgconn.PgError{Code: "23502"},
			want: false,
		},
		{
			name: "wrapped 23505 still detected via errors.As",
			err:  wrapErr(&pgconn.PgError{Code: "23505"}),
			want: true,
		},
		{
			name: "pgx.ErrNoRows returns false (NOT duplicate)",
			err:  pgx.ErrNoRows,
			want: false,
		},
		{
			name: "generic error returns false",
			err:  errors.New("connection refused"),
			want: false,
		},
		{
			name: "nil error returns false",
			err:  nil,
			want: false,
		},
		{
			name: "error message containing 'duplicate' but not PgError returns false",
			err:  errors.New("duplicate something in app layer"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDuplicate(tt.err); got != tt.want {
				t.Errorf("isDuplicate(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// wrapErr 模拟 fmt.Errorf("...: %w", err) 的多层包装
func wrapErr(err error) error {
	return &wrappedErr{inner: err, msg: "create order failed"}
}

type wrappedErr struct {
	inner error
	msg   string
}

func (w *wrappedErr) Error() string { return w.msg + ": " + w.inner.Error() }
func (w *wrappedErr) Unwrap() error { return w.inner }

// TestGetByUserAndTrace_NotFound_ReturnsErrNotFound 验证空存储查询返回 ErrNotFound
func TestGetByUserAndTrace_NotFound_ReturnsErrNotFound(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.GetByUserAndTrace(context.Background(), 50001, "NOT-EXIST")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByUserAndTrace(missing) err = %v, want ErrNotFound", err)
	}
}

// TestGetByUserAndTrace_Found_ReturnsOrder 验证 (userID, traceID) 组合命中订单
// 并验证不同 user_id 同 trace_id 不命中（INDEX 维度含 user_id）
func TestGetByUserAndTrace_Found_ReturnsOrder(t *testing.T) {
	store := NewMemoryStore()
	order := entity.Order{
		OrderNo:    "O-FOUND-1",
		UserID:     50001,
		ActivityNo: "ACT-1",
		SKUNo:      "SKU-1",
		Quantity:   2,
		PayAmount:  199,
		Status:     "PENDING_PAY",
		TraceID:    "TRACE-FOUND-1",
	}
	if err := store.CreateOrder(context.Background(), order); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	got, err := store.GetByUserAndTrace(context.Background(), 50001, "TRACE-FOUND-1")
	if err != nil {
		t.Fatalf("GetByUserAndTrace: %v", err)
	}
	if got.OrderNo != "O-FOUND-1" {
		t.Errorf("OrderNo = %s, want O-FOUND-1", got.OrderNo)
	}
	if got.TraceID != "TRACE-FOUND-1" {
		t.Errorf("TraceID = %s, want TRACE-FOUND-1", got.TraceID)
	}

	// 不同 user_id 同 trace_id, 应该 NotFound（INDEX 维度含 user_id）
	_, err = store.GetByUserAndTrace(context.Background(), 59999, "TRACE-FOUND-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByUserAndTrace(cross-user) err = %v, want ErrNotFound", err)
	}
}
