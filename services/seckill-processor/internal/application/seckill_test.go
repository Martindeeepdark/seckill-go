package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/Martindeeepdark/go-common/eventbus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"seckill-processor-service/internal/application/usecase"
	"seckill-processor-service/internal/domain/event"
	"seckill-processor-service/internal/domain/model"
	domainservice "seckill-processor-service/internal/domain/service"
	domainstatus "seckill-processor-service/internal/domain/status"
	"seckill-processor-service/internal/infrastructure/identity"
)

// ---------------------------------------------------------------------------
// SeckillApp table-driven tests for HandleSeckill
// ---------------------------------------------------------------------------

func TestHandleSeckill(t *testing.T) {
	tests := []struct {
		name       string
		traces     *fakeTraceResults
		proc       *fakeProcessorStore
		seckill    *domainservice.SeckillService
		message    model.SeckillMessage
		wantErr    bool
		assertFunc func(t *testing.T, traces *fakeTraceResults, proc *fakeProcessorStore)
	}{
		{
			name:   "try start returns false already processing",
			traces: &fakeTraceResults{},
			proc:   &fakeProcessorStore{tryStartResult: false},
			seckill: newTestSeckillService(nil),
			message: model.SeckillMessage{
				TraceID:        "trace-dup",
				RequestTraceID: "11111111111111111111111111111111",
				ActivityNo:     "A1",
				SKUNo:          "S1",
				UserID:         7,
				Quantity:       1,
			},
			wantErr: false,
			assertFunc: func(t *testing.T, traces *fakeTraceResults, proc *fakeProcessorStore) {
				if proc.releaseCalled {
					t.Fatal("expected processor idem NOT to be released on duplicate")
				}
			},
		},
		{
			name:   "try start error returns error",
			traces: &fakeTraceResults{},
			proc:   &fakeProcessorStore{tryStartErr: errors.New("redis connection refused")},
			seckill: newTestSeckillService(nil),
			message: model.SeckillMessage{
				TraceID:        "trace-err",
				RequestTraceID: "11111111111111111111111111111112",
				ActivityNo:     "A1",
				SKUNo:          "S1",
				UserID:         7,
				Quantity:       1,
			},
			wantErr: true,
			assertFunc: func(t *testing.T, traces *fakeTraceResults, proc *fakeProcessorStore) {
				if proc.releaseCalled {
					t.Fatal("expected processor idem NOT to be released on try start error")
				}
			},
		},
		{
			name:   "submit success returns nil",
			traces: &fakeTraceResults{},
			proc:   &fakeProcessorStore{tryStartResult: true},
			seckill: newTestSeckillService(nil),
			message: model.SeckillMessage{
				TraceID:        "trace-ok",
				RequestTraceID: "11111111111111111111111111111113",
				ActivityNo:     "A1",
				SKUNo:          "S1",
				UserID:         7,
				Quantity:       1,
			},
			wantErr: false,
			assertFunc: func(t *testing.T, traces *fakeTraceResults, proc *fakeProcessorStore) {
				if proc.releaseCalled {
					t.Fatal("expected processor idem NOT to be released on success")
				}
			},
		},
		{
			name:   "submit error releases processor idem key",
			traces: &fakeTraceResults{},
			proc:   &fakeProcessorStore{tryStartResult: true},
			seckill: newTestSeckillService(
				status.Error(codes.Unavailable, "order svc down"),
			),
			message: model.SeckillMessage{
				TraceID:        "trace-fail",
				RequestTraceID: "11111111111111111111111111111114",
				ActivityNo:     "A1",
				SKUNo:          "S1",
				UserID:         7,
				Quantity:       1,
			},
			wantErr: true,
			assertFunc: func(t *testing.T, traces *fakeTraceResults, proc *fakeProcessorStore) {
				if !proc.releaseCalled {
					t.Fatal("expected processor idem to be released on submit error")
				}
				if len(proc.releasedTraceIDs) == 0 || proc.releasedTraceIDs[0] != "trace-fail" {
					t.Fatalf("releasedTraceIDs = %v, want [trace-fail]", proc.releasedTraceIDs)
				}
			},
		},
		{
			name:    "nil stores skips dedup",
			traces:  nil,
			proc:    nil,
			seckill: newTestSeckillService(nil),
			message: model.SeckillMessage{
				TraceID:        "trace-nil",
				RequestTraceID: "11111111111111111111111111111115",
				ActivityNo:     "A1",
				SKUNo:          "S1",
				UserID:         7,
				Quantity:       1,
			},
			wantErr: false,
		},
		{
			name:   "default quantity when zero",
			traces: &fakeTraceResults{},
			proc:   &fakeProcessorStore{tryStartResult: true},
			seckill: newTestSeckillService(nil),
			message: model.SeckillMessage{
				TraceID:        "trace-q",
				RequestTraceID: "11111111111111111111111111111116",
				ActivityNo:     "A1",
				SKUNo:          "S1",
				UserID:         7,
				Quantity:       0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := eventbus.NewEventBus()
			var opts []SeckillAppOption
			if tt.traces != nil {
				opts = append(opts, WithTraceResults(tt.traces))
			}
			// 注意: tt.proc 是 *fakeProcessorStore,直接传给接口参数会包装为非 nil 接口,
			// 导致 SubmitSeckill 内部 != nil 判断错误.这里使用明确的 interface 变量传递.
			var processorStore usecase.ProcessorStore
			if tt.proc != nil {
				processorStore = tt.proc
				opts = append(opts, WithProcessorStore(tt.proc))
			}
			submitUC := usecase.NewSubmitSeckill(tt.seckill, processorStore, slog.Default())
			app := NewSeckillApp(submitUC, bus, nil, slog.Default(), opts...)
			if err := app.RegisterHandlers(); err != nil {
				t.Fatalf("RegisterHandlers: %v", err)
			}

			err := app.HandleSeckill(context.Background(), tt.message)

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.assertFunc != nil {
				tt.assertFunc(t, tt.traces, tt.proc)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// newTestSeckillService creates a SeckillService with standard test deps.
// If orderErr is non-nil, CreateOrder returns that error.
// ---------------------------------------------------------------------------
func newTestSeckillService(orderErr error) *domainservice.SeckillService {
	return domainservice.NewSeckillService(
		&appFakeActivity{
			activity: appOpenActivity(),
			sku:      model.SKUInfo{SKUNo: "S1", SeckillPrice: 9900},
		},
		&appFakeStock{deducted: true},
		&appFakeRisk{},
		&appFakeOrders{err: orderErr},
		eventbus.NewEventBus(),
		identity.SnowflakeIDGenerator{},
		identity.RPCTemporaryChecker{},
		slog.Default(),
	)
}

// ---------------------------------------------------------------------------
// Existing tests for RegisterHandlers
// ---------------------------------------------------------------------------

func TestRegisterHandlersOrderCreatedPublishesTimeoutAndMarksSuccess(t *testing.T) {
	bus := eventbus.NewEventBus()
	traces := &fakeTraceResults{}
	proc := &fakeProcessorStore{}
	timeouts := &fakePaymentTimeouts{}
	submitUC := usecase.NewSubmitSeckill(nil, proc, slog.Default())
	app := NewSeckillApp(submitUC, bus, nil, slog.Default(),
		WithTraceResults(traces), WithProcessorStore(proc), WithPaymentTimeouts(timeouts, time.Minute))
	if err := app.RegisterHandlers(); err != nil {
		t.Fatalf("RegisterHandlers returned error: %v", err)
	}

	bus.Publish(event.TopicOrderCreated, event.OrderCreated{
		OrderNo:        "O1",
		TraceID:        "trace-1",
		RequestTraceID: "11111111111111111111111111111111",
	})

	// 验证双 key 写入: gateway result key + processor idem key 都被写
	if traces.successTraceID != "trace-1" || traces.successOrderNo != "O1" {
		t.Fatalf("gateway result key success = %s/%s, want trace-1/O1",
			traces.successTraceID, traces.successOrderNo)
	}
	if len(proc.markSuccessCalls) == 0 {
		t.Fatal("processor idem key MarkSuccess not called")
	}
	if proc.markSuccessCalls[0].TraceID != "trace-1" || proc.markSuccessCalls[0].OrderNo != "O1" {
		t.Fatalf("processor idem MarkSuccess = %v, want trace-1/O1", proc.markSuccessCalls[0])
	}
	if timeouts.published.OrderNo != "O1" {
		t.Fatalf("timeout order = %s, want O1", timeouts.published.OrderNo)
	}
	if timeouts.published.DueAt.IsZero() {
		t.Fatal("timeout due time was not set")
	}
}

func TestRegisterHandlersRejectedMarksFail(t *testing.T) {
	bus := eventbus.NewEventBus()
	traces := &fakeTraceResults{}
	proc := &fakeProcessorStore{}
	submitUC := usecase.NewSubmitSeckill(nil, proc, slog.Default())
	app := NewSeckillApp(submitUC, bus, nil, slog.Default(),
		WithTraceResults(traces), WithProcessorStore(proc))
	if err := app.RegisterHandlers(); err != nil {
		t.Fatalf("RegisterHandlers returned error: %v", err)
	}

	bus.Publish(event.TopicSeckillRejected, event.SeckillRejected{
		TraceID: "trace-1",
		Reason:  event.ReasonStockEmpty,
	})

	// 验证双 key 写入失败路径
	if traces.failTraceID != "trace-1" || traces.failReason != event.ReasonStockEmpty {
		t.Fatalf("gateway result key fail = %s/%s, want trace-1/%s",
			traces.failTraceID, traces.failReason, event.ReasonStockEmpty)
	}
	if len(proc.markFailCalls) == 0 {
		t.Fatal("processor idem key MarkFail not called")
	}
	if proc.markFailCalls[0].TraceID != "trace-1" || proc.markFailCalls[0].Reason != event.ReasonStockEmpty {
		t.Fatalf("processor idem MarkFail = %v, want trace-1/%s", proc.markFailCalls[0], event.ReasonStockEmpty)
	}
}

func TestHandleSeckillReleasesTraceAndReturnsErrorForTemporaryRPCFailure(t *testing.T) {
	bus := eventbus.NewEventBus()
	traces := &fakeTraceResults{}
	proc := &fakeProcessorStore{tryStartResult: true}
	seckill := domainservice.NewSeckillService(
		&appFakeActivity{activity: appOpenActivity(), sku: model.SKUInfo{SKUNo: "S1", SeckillPrice: 9900}},
		&appFakeStock{deducted: true},
		&appFakeRisk{},
		&appFakeOrders{err: status.Error(codes.Unavailable, "order service unavailable")},
		bus,
		identity.SnowflakeIDGenerator{},
		identity.RPCTemporaryChecker{},
		slog.Default(),
	)
	submitUC := usecase.NewSubmitSeckill(seckill, proc, slog.Default())
	app := NewSeckillApp(submitUC, bus, nil, slog.Default(),
		WithTraceResults(traces), WithProcessorStore(proc))
	if err := app.RegisterHandlers(); err != nil {
		t.Fatalf("RegisterHandlers returned error: %v", err)
	}

	err := app.HandleSeckill(context.Background(), model.SeckillMessage{
		TraceID:        "trace-1",
		RequestTraceID: "11111111111111111111111111111111",
		ActivityNo:     "A1",
		SKUNo:          "S1",
		UserID:         7,
		Quantity:       1,
	})
	if err == nil {
		t.Fatal("HandleSeckill returned nil error, want retryable failure")
	}
	// Release 应只删 processor idem key,不动 gateway result key
	if !proc.releaseCalled {
		t.Fatal("expected processor idem Release to be called")
	}
	if len(proc.releasedTraceIDs) == 0 || proc.releasedTraceIDs[0] != "trace-1" {
		t.Fatalf("releasedTraceIDs = %v, want [trace-1]", proc.releasedTraceIDs)
	}
	if traces.failTraceID != "" {
		t.Fatalf("gateway result key fail should be empty on infra error (PROCESSING retained), got %q", traces.failTraceID)
	}
}

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

// fakeTraceResults 模拟 gateway result key 写入(application.TraceResultStore)
type fakeTraceResults struct {
	tryStartResult  bool
	tryStartErr     error
	successTraceID  string
	successOrderNo  string
	failTraceID     string
	failReason      string
	deletedTraceID  string
}

func (s *fakeTraceResults) TryStart(context.Context, string, time.Duration) (bool, error) {
	return s.tryStartResult, s.tryStartErr
}

func (s *fakeTraceResults) MarkSuccess(_ context.Context, traceID, orderNo string, _ time.Duration) error {
	s.successTraceID = traceID
	s.successOrderNo = orderNo
	return nil
}

func (s *fakeTraceResults) MarkFail(_ context.Context, traceID, reason string, _ time.Duration) error {
	s.failTraceID = traceID
	s.failReason = reason
	return nil
}

func (s *fakeTraceResults) Delete(_ context.Context, traceID string) error {
	s.deletedTraceID = traceID
	return nil
}

// fakeProcessorStore 模拟 processor idem key 写入(application.ProcessorStore)
type fakeProcessorStore struct {
	tryStartCalled   bool
	tryStartResult   bool
	tryStartErr      error
	releaseCalled    bool
	releasedTraceIDs []string
	markSuccessCalls []struct{ TraceID, OrderNo string }
	markFailCalls    []struct{ TraceID, Reason string }
}

func (f *fakeProcessorStore) TryStart(context.Context, string, time.Duration) (bool, error) {
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

type fakePaymentTimeouts struct {
	published model.PaymentTimeoutTask
}

func (p *fakePaymentTimeouts) PublishPaymentTimeout(_ context.Context, task model.PaymentTimeoutTask) error {
	p.published = task
	return nil
}

type appFakeActivity struct {
	activity model.ActivityInfo
	sku      model.SKUInfo
}

func (g *appFakeActivity) GetActivity(context.Context, string) (model.ActivityInfo, error) {
	return g.activity, nil
}

func (g *appFakeActivity) GetSKU(context.Context, string, string) (model.SKUInfo, error) {
	return g.sku, nil
}

type appFakeStock struct {
	deducted bool
}

func (g *appFakeStock) DeductStockWithLimit(context.Context, string, string, int64, int64, int64, string) (bool, error) {
	return g.deducted, nil
}

func (g *appFakeStock) ReleaseStock(context.Context, string, string, int64, int64, string) error {
	return nil
}

type appFakeRisk struct{}

func (g *appFakeRisk) Evaluate(context.Context, int64, string) (model.RiskResult, error) {
	return model.RiskResult{}, nil
}

type appFakeOrders struct {
	err error
}

func (g *appFakeOrders) CreateOrder(context.Context, model.OrderRequest) error {
	return g.err
}

// GetByUserAndTrace satisfies the extended OrderCreator interface; not used in app-level tests
func (g *appFakeOrders) GetByUserAndTrace(context.Context, int64, string) (model.OrderInfo, error) {
	return model.OrderInfo{}, errors.New("not implemented in appFakeOrders")
}

func appOpenActivity() model.ActivityInfo {
	return model.ActivityInfo{
		ActivityNo:    "A1",
		StartTime:     time.Now().Add(-time.Minute),
		EndTime:       time.Now().Add(time.Minute),
		Status:        domainstatus.ActivityOpen,
		PurchaseLimit: 5,
	}
}
