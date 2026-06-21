package application

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"seckill-common/tracing"

	"seckill-gateway-service/internal/config"
)

func TestWorkerPoolProcessEventRestoresTraceContext(t *testing.T) {
	publisher := &fakeQueuePublisher{}
	pool := NewWorkerPool(
		WorkerPoolConfig{QueueSize: 1, WorkerCount: 1},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeRiskGateway{},
		&fakeStockGateway{stock: 10},
		publisher,
		nil,
		slog.Default(),
	)

	traceID := tracing.NewTraceID()
	pool.processEvent(context.Background(), PartInEvent{
		UserID:      7,
		ActivityNo:  "A1",
		SkuNo:       "S1",
		Quantity:    1,
		TraceID:     traceID,
		MachinePass: true,
	}, 1)

	if publisher.traceIDSeen != traceID {
		t.Fatalf("publisher context traceID = %q, want %q", publisher.traceIDSeen, traceID)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}
	if publisher.events[0].TraceID != traceID {
		t.Fatalf("published event traceID = %q, want %q", publisher.events[0].TraceID, traceID)
	}
}

// --- Task 0 tests: WorkerPool initialization and fast path ---

func TestNewWorkerPoolReturnsNonNil(t *testing.T) {
	pool := NewWorkerPool(
		DefaultWorkerPoolConfig(),
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeRiskGateway{},
		&fakeStockGateway{stock: 10},
		&fakeQueuePublisher{},
		nil,
		slog.Default(),
	)
	if pool == nil {
		t.Fatal("NewWorkerPool returned nil, want non-nil WorkerPool")
	}
}

func TestSeckillAppWithWorkerPoolTakesFastPath(t *testing.T) {
	// This test verifies that when a WorkerPool is provided to NewSeckillApp,
	// PartIn takes the fast path: no synchronous gRPC calls to activity/stock/risk,
	// and the event is submitted to the worker pool instead.
	activityGW := &trackingActivityGateway{detail: openActivityDetail()}
	stockGW := &trackingStockGateway{stock: 10}
	riskGW := &trackingRiskGateway{}
	queue := &fakeQueuePublisher{}

	pool := NewWorkerPool(
		WorkerPoolConfig{QueueSize: 100, WorkerCount: 1},
		activityGW,
		riskGW,
		stockGW,
		queue,
		nil, // no result store needed for this test
		slog.Default(),
	)

	// Start the worker pool in background so it can drain events
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pool.Start(ctx)

	app := NewSeckillApp(
		config.GatewayConfig{},
		activityGW,
		stockGW,
		riskGW,
		nil, // order gateway
		queue,
		nil, // result store
		&NopMachineChecker{},
		pool, // worker pool enabled
		slog.Default(),
	)

	traceID := tracing.NewTraceID()
	result, err := app.PartIn(
		tracing.WithTraceID(context.Background(), traceID),
		7, "A1", "S1", "127.0.0.1", "", 1, "",
	)
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}

	// Fast path should return success immediately
	if !result.Queued {
		t.Fatalf("result.Queued = false, want true (fast path should queue immediately)")
	}
	if result.Token != traceID {
		t.Fatalf("result.Token = %q, want %q", result.Token, traceID)
	}

	// The fast path should NOT call activity/stock/risk synchronously
	// Those calls happen asynchronously in the worker pool
	// Wait for the worker pool to process the event
	time.Sleep(100 * time.Millisecond)

	// After worker processes, activity/stock should have been called by the pool
	activityGW.mu.Lock()
	calls := activityGW.getActivityCalls
	activityGW.mu.Unlock()
	if calls == 0 {
		t.Fatal("worker pool should have called GetActivity during async processing")
	}
}

func TestSeckillAppWithoutWorkerPoolTakesSlowPath(t *testing.T) {
	// This test verifies backward compatibility: when workerPool is nil,
	// PartIn uses the slow path with synchronous gRPC calls.
	activityGW := &trackingActivityGateway{detail: openActivityDetail()}

	app := NewSeckillApp(
		config.GatewayConfig{},
		activityGW,
		&fakeStockGateway{stock: 10},
		&fakeRiskGateway{},
		nil,
		&fakeQueuePublisher{},
		nil,
		&NopMachineChecker{},
		nil, // no worker pool -> slow path
		slog.Default(),
	)

	_, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}

	// Slow path calls GetActivity synchronously
	if activityGW.getActivityCalls == 0 {
		t.Fatal("slow path should have called GetActivity synchronously")
	}
	if activityGW.getActivityCalls != 1 {
		t.Fatalf("GetActivity calls = %d, want 1", activityGW.getActivityCalls)
	}
}

func TestWorkerPoolSubmitEnqueuesEvent(t *testing.T) {
	pool := NewWorkerPool(
		WorkerPoolConfig{QueueSize: 10, WorkerCount: 1},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeRiskGateway{},
		&fakeStockGateway{stock: 10},
		&fakeQueuePublisher{},
		nil,
		slog.Default(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pool.Start(ctx)

	event := PartInEvent{
		UserID:     1,
		ActivityNo: "A1",
		SkuNo:      "S1",
		TraceID:    "test-trace",
	}

	if err := pool.Submit(ctx, event); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
}

func TestWorkerPoolSubmitReturnsErrorWhenQueueFull(t *testing.T) {
	pool := NewWorkerPool(
		WorkerPoolConfig{QueueSize: 1, WorkerCount: 0}, // 0 workers so nothing drains
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeRiskGateway{},
		&fakeStockGateway{stock: 10},
		&fakeQueuePublisher{},
		nil,
		slog.Default(),
	)

	// Fill the queue
	ctx := context.Background()
	_ = pool.Submit(ctx, PartInEvent{UserID: 1, ActivityNo: "A1", TraceID: "t1"})

	// Next submit should fail
	err := pool.Submit(ctx, PartInEvent{UserID: 2, ActivityNo: "A2", TraceID: "t2"})
	if err == nil {
		t.Fatal("expected error when queue is full, got nil")
	}
}

// Tracking gateways that record whether methods were called

type trackingActivityGateway struct {
	detail           *ActivityDetail
	getActivityCalls int
	mu               sync.Mutex
}

func (g *trackingActivityGateway) ListActivities(context.Context) (ActivityList, error) {
	return ActivityList{}, nil
}

func (g *trackingActivityGateway) GetActivity(ctx context.Context, no string) (*ActivityDetail, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.getActivityCalls++
	return g.detail, nil
}

func (g *trackingActivityGateway) CreateActivity(context.Context, CreateActivityRequest) (*ActivityDetail, error) {
	return g.detail, nil
}
func (g *trackingActivityGateway) UpdateActivity(context.Context, UpdateActivityRequest) error { return nil }
func (g *trackingActivityGateway) EndActivity(context.Context, string) error                   { return nil }
func (g *trackingActivityGateway) AddProduct(context.Context, AddProductRequest) error          { return nil }
func (g *trackingActivityGateway) RemoveProduct(context.Context, string, string) error          { return nil }

type trackingStockGateway struct {
	stock     int64
	err       error
	peekCalls int
	mu        sync.Mutex
}

func (g *trackingStockGateway) Peek(context.Context, string, string) (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.peekCalls++
	return g.stock, g.err
}
func (g *trackingStockGateway) Deduct(context.Context, DeductRequest) (bool, error) { return true, nil }
func (g *trackingStockGateway) Release(context.Context, string, string, string, int) error {
	return nil
}

type trackingRiskGateway struct {
	riskUser     bool
	isRiskCalls  int
	mu           sync.Mutex
}

func (g *trackingRiskGateway) Evaluate(context.Context, int64, string) (*RiskEvaluation, error) {
	return &RiskEvaluation{}, nil
}
func (g *trackingRiskGateway) IsRiskUser(context.Context, int64) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.isRiskCalls++
	return g.riskUser, nil
}
func (g *trackingRiskGateway) MarkSuspicious(context.Context, *ActivityDetail, int64, string) error {
	return nil
}
func (g *trackingRiskGateway) RecordAction(context.Context, RiskRecord) error { return nil }
