// Package usecase 提供应用层 Use Case 的单元测试
package usecase

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"seckill-processor-service/internal/domain/model"
	domainservice "seckill-processor-service/internal/domain/service"
	domainstatus "seckill-processor-service/internal/domain/status"
	"seckill-processor-service/internal/infrastructure/identity"
)

// fakeProcessorStore 实现 application.ProcessorStore 接口的内存版本
// 用于单元测试 SubmitSeckill 与 ProcessorStore 的交互
type fakeProcessorStore struct {
	tryStartCalled   bool
	tryStartResult   bool
	tryStartErr      error
	releaseCalled    bool
	releasedTraceIDs []string
	markSuccessCalls []struct{ TraceID, OrderNo string }
	markFailCalls    []struct{ TraceID, Reason string }
}

func (f *fakeProcessorStore) TryStart(_ context.Context, _ string, _ time.Duration) (bool, error) {
	f.tryStartCalled = true
	return f.tryStartResult, f.tryStartErr
}

func (f *fakeProcessorStore) MarkSuccess(_ context.Context, traceID, orderNo string, _ time.Duration) error {
	f.markSuccessCalls = append(f.markSuccessCalls, struct{ TraceID, OrderNo string }{traceID, orderNo})
	return nil
}

func (f *fakeProcessorStore) MarkFail(_ context.Context, traceID, reason string, _ time.Duration) error {
	f.markFailCalls = append(f.markFailCalls, struct{ TraceID, Reason string }{traceID, reason})
	return nil
}

func (f *fakeProcessorStore) Release(_ context.Context, traceID string) error {
	f.releaseCalled = true
	f.releasedTraceIDs = append(f.releasedTraceIDs, traceID)
	return nil
}

// newTestSeckillServiceForUseCase 构造一个标准测试用的领域服务
// orderErr 非 nil 时 CreateOrder 返回该错误
func newTestSeckillServiceForUseCase(orderErr error) *domainservice.SeckillService {
	return domainservice.NewSeckillService(
		&fakeActivity{activity: openActivity(), sku: model.SKUInfo{SKUNo: "S1", SeckillPrice: 9900}},
		&fakeStock{deducted: true},
		&fakeRisk{},
		&fakeOrders{err: orderErr},
		nil,
		identity.SnowflakeIDGenerator{},
		identity.RPCTemporaryChecker{},
		slog.Default(),
	)
}

func TestSubmitSeckill_TryStartFails_SkipsProcessing(t *testing.T) {
	store := &fakeProcessorStore{tryStartResult: false} // SetNX 失败(已存在)
	seckill := newTestSeckillServiceForUseCase(nil)

	uc := NewSubmitSeckill(seckill, store, slog.Default())
	err := uc.Execute(context.Background(), model.SeckillMessage{
		TraceID:        "trace-dup",
		RequestTraceID: "11111111111111111111111111111111",
		ActivityNo:     "A1",
		SKUNo:          "S1",
		UserID:         7,
		Quantity:       1,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v, want nil (duplicate should be silently skipped)", err)
	}
	if !store.tryStartCalled {
		t.Error("TryStart was not called")
	}
	if store.releaseCalled {
		t.Error("Release should not be called when TryStart returns false")
	}
}

func TestSubmitSeckill_TryStartError_ReturnsError(t *testing.T) {
	store := &fakeProcessorStore{
		tryStartErr: errors.New("redis connection refused"),
	}
	seckill := newTestSeckillServiceForUseCase(nil)

	uc := NewSubmitSeckill(seckill, store, slog.Default())
	err := uc.Execute(context.Background(), model.SeckillMessage{
		TraceID:        "trace-err",
		RequestTraceID: "11111111111111111111111111111112",
		ActivityNo:     "A1",
		SKUNo:          "S1",
		UserID:         7,
		Quantity:       1,
	})
	if err == nil {
		t.Fatal("Execute returned nil, want error on TryStart failure")
	}
	if store.releaseCalled {
		t.Error("Release should not be called when TryStart errors")
	}
}

func TestSubmitSeckill_TryStartSucceeds_CallsSubmit(t *testing.T) {
	store := &fakeProcessorStore{tryStartResult: true}
	seckill := newTestSeckillServiceForUseCase(nil)

	uc := NewSubmitSeckill(seckill, store, slog.Default())
	err := uc.Execute(context.Background(), model.SeckillMessage{
		TraceID:        "trace-ok",
		RequestTraceID: "11111111111111111111111111111113",
		ActivityNo:     "A1",
		SKUNo:          "S1",
		UserID:         7,
		Quantity:       1,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v, want nil", err)
	}
	if store.releaseCalled {
		t.Error("Release should not be called on successful Submit")
	}
}

// TestSubmitSeckill_SubmitFails_ReleasesProcessorIdem
// 基础设施错误时调用 Release 删除 PROCESSING key,允许消息重试
func TestSubmitSeckill_SubmitFails_ReleasesProcessorIdem(t *testing.T) {
	store := &fakeProcessorStore{tryStartResult: true}
	seckill := newTestSeckillServiceForUseCase(
		status.Error(codes.Unavailable, "order service unavailable"),
	)

	uc := NewSubmitSeckill(seckill, store, slog.Default())
	err := uc.Execute(context.Background(), model.SeckillMessage{
		TraceID:        "trace-fail",
		RequestTraceID: "11111111111111111111111111111114",
		ActivityNo:     "A1",
		SKUNo:          "S1",
		UserID:         7,
		Quantity:       1,
	})
	if err == nil {
		t.Fatal("Execute returned nil, want error on Submit failure")
	}
	if !store.releaseCalled {
		t.Fatal("Release was not called on Submit failure")
	}
	if len(store.releasedTraceIDs) == 0 || store.releasedTraceIDs[0] != "trace-fail" {
		t.Fatalf("releasedTraceIDs = %v, want [trace-fail]", store.releasedTraceIDs)
	}
}

// TestSubmitSeckill_NilProcessorStore_SkipsLayer1
// processorStore 为 nil 时跳过 Layer 1 防重复(测试场景/降级)
func TestSubmitSeckill_NilProcessorStore_SkipsLayer1(t *testing.T) {
	seckill := newTestSeckillServiceForUseCase(nil)
	uc := NewSubmitSeckill(seckill, nil, slog.Default())
	err := uc.Execute(context.Background(), model.SeckillMessage{
		TraceID:        "trace-nil",
		RequestTraceID: "11111111111111111111111111111115",
		ActivityNo:     "A1",
		SKUNo:          "S1",
		UserID:         7,
		Quantity:       1,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// test fakes (在本测试文件内独立定义,避免与 application/seckill_test.go 冲突)
// ---------------------------------------------------------------------------

type fakeActivity struct {
	activity model.ActivityInfo
	sku      model.SKUInfo
}

func (g *fakeActivity) GetActivity(context.Context, string) (model.ActivityInfo, error) {
	return g.activity, nil
}

func (g *fakeActivity) GetSKU(context.Context, string, string) (model.SKUInfo, error) {
	return g.sku, nil
}

type fakeStock struct {
	deducted bool
}

func (g *fakeStock) DeductStockWithLimit(context.Context, string, string, int64, int64, int64, string) (bool, error) {
	return g.deducted, nil
}

func (g *fakeStock) ReleaseStock(context.Context, string, string, int64, int64, string) error {
	return nil
}

type fakeRisk struct{}

func (g *fakeRisk) Evaluate(context.Context, int64, string) (model.RiskResult, error) {
	return model.RiskResult{}, nil
}

type fakeOrders struct {
	err error
}

func (g *fakeOrders) CreateOrder(context.Context, model.OrderRequest) error {
	return g.err
}

// GetByUserAndTrace satisfies the extended OrderCreator interface; not used in submit_seckill tests
func (g *fakeOrders) GetByUserAndTrace(context.Context, int64, string) (model.OrderInfo, error) {
	return model.OrderInfo{}, errors.New("not implemented in fakeOrders")
}

func openActivity() model.ActivityInfo {
	return model.ActivityInfo{
		ActivityNo:    "A1",
		StartTime:     time.Now().Add(-time.Minute),
		EndTime:       time.Now().Add(time.Minute),
		Status:        domainstatus.ActivityOpen,
		PurchaseLimit: 5,
	}
}

// 编译期断言:确保 *fakeProcessorStore 实现 ProcessorStore 接口
var _ ProcessorStore = (*fakeProcessorStore)(nil)
