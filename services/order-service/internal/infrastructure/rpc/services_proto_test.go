// Package rpc 提供 gRPC 服务的 Protobuf 实现
package rpc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orderv1 "seckill-api/order/v1"

	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/infrastructure/persistence"
)

// fakeStore 用于测试 RPC handler, 实现 OrderStore 接口
type fakeStore struct {
	orderByUserTrace map[string]entity.Order // key = "userID:traceID"
	createErr        error
}

func (f *fakeStore) CreateOrder(ctx context.Context, order entity.Order) error {
	return f.createErr
}

func (f *fakeStore) GetOrder(ctx context.Context, orderNo string) (entity.Order, error) {
	return entity.Order{}, persistence.ErrNotFound
}

// GetByUserAndTrace 根据 (userID, traceID) 双键查询订单
func (f *fakeStore) GetByUserAndTrace(ctx context.Context, userID int64, traceID string) (entity.Order, error) {
	key := fakeStoreKey(userID, traceID)
	if o, ok := f.orderByUserTrace[key]; ok {
		return o, nil
	}
	return entity.Order{}, persistence.ErrNotFound
}

func (f *fakeStore) ListOrdersByActivity(ctx context.Context, activityNo string) ([]entity.Order, error) {
	return nil, nil
}

func (f *fakeStore) ListOrdersByActivities(ctx context.Context, activityNos []string) (map[string][]entity.Order, error) {
	return nil, nil
}

func (f *fakeStore) ListOrdersByUser(ctx context.Context, userID int64) ([]entity.Order, error) {
	return nil, nil
}

func (f *fakeStore) MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error {
	return nil
}

func (f *fakeStore) CloseOrder(ctx context.Context, orderNo string) error {
	return nil
}

// fakeStoreKey 构造 fakeStore 内部 map key
func fakeStoreKey(userID int64, traceID string) string {
	return formatKey(userID, traceID)
}

// formatKey 把 userID 与 traceID 拼成稳定 key
func formatKey(userID int64, traceID string) string {
	// 使用 itoa 风格拼接; 避免在测试 helper 中引入额外 import
	return int64ToString(userID) + ":" + traceID
}

// int64ToString 简化版整数转字符串
func int64ToString(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// isGRPCCode 判断错误是否为给定 gRPC 状态码
func isGRPCCode(err error, c codes.Code) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == c
}

func TestOrderPBService_GetOrderByUserAndTrace_Found(t *testing.T) {
	store := &fakeStore{
		orderByUserTrace: map[string]entity.Order{
			fakeStoreKey(70001, "T1"): {OrderNo: "O1", TraceID: "T1", UserID: 70001, Status: "PENDING_PAY"},
		},
	}
	svc := NewOrderPBService(store)

	resp, err := svc.GetOrderByUserAndTrace(
		context.Background(),
		&orderv1.GetOrderByUserAndTraceRequest{UserId: 70001, TraceId: "T1"},
	)
	if err != nil {
		t.Fatalf("GetOrderByUserAndTrace returned error: %v", err)
	}
	if resp.GetOrder().GetOrderNo() != "O1" {
		t.Errorf("OrderNo = %s, want O1", resp.GetOrder().GetOrderNo())
	}
	if resp.GetOrder().GetTraceId() != "T1" {
		t.Errorf("TraceId = %s, want T1", resp.GetOrder().GetTraceId())
	}
	if resp.GetOrder().GetUserId() != 70001 {
		t.Errorf("UserId = %d, want 70001", resp.GetOrder().GetUserId())
	}
}

func TestOrderPBService_GetOrderByUserAndTrace_NotFound(t *testing.T) {
	svc := NewOrderPBService(&fakeStore{orderByUserTrace: map[string]entity.Order{}})

	_, err := svc.GetOrderByUserAndTrace(
		context.Background(),
		&orderv1.GetOrderByUserAndTraceRequest{UserId: 70001, TraceId: "MISSING"},
	)
	if err == nil {
		t.Fatal("expected error for missing trace_id, got nil")
	}
	if !isGRPCCode(err, codes.NotFound) {
		t.Errorf("expected codes.NotFound, got %v", err)
	}
}

func TestOrderPBService_GetOrderByUserAndTrace_InvalidUserID(t *testing.T) {
	svc := NewOrderPBService(&fakeStore{})

	_, err := svc.GetOrderByUserAndTrace(
		context.Background(),
		&orderv1.GetOrderByUserAndTraceRequest{UserId: 0, TraceId: "T1"},
	)
	if err == nil {
		t.Fatal("expected error for zero user_id, got nil")
	}
	if !isGRPCCode(err, codes.InvalidArgument) {
		t.Errorf("expected codes.InvalidArgument, got %v", err)
	}
}

func TestOrderPBService_GetOrderByUserAndTrace_EmptyTraceID(t *testing.T) {
	svc := NewOrderPBService(&fakeStore{})

	_, err := svc.GetOrderByUserAndTrace(
		context.Background(),
		&orderv1.GetOrderByUserAndTraceRequest{UserId: 70001, TraceId: ""},
	)
	if err == nil {
		t.Fatal("expected error for empty trace_id, got nil")
	}
	if !isGRPCCode(err, codes.InvalidArgument) {
		t.Errorf("expected codes.InvalidArgument, got %v", err)
	}
}
