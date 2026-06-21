package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"seckill-common/tracing"

	"seckill-gateway-service/internal/config"
)

func TestPartInRecordsSeckillActionAfterSuccessfulQueue(t *testing.T) {
	risk := &fakeRiskGateway{}
	queue := &fakeQueuePublisher{}
	app := NewSeckillApp(
		config.GatewayConfig{Risk: config.RiskConfig{Enabled: true}},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		risk,
		nil,
		queue,
		nil,
		&NopMachineChecker{},
		nil, // worker pool
		slog.Default(),
	)

	traceID := "11111111111111111111111111111111"
	result, err := app.PartIn(tracing.WithTraceID(context.Background(), traceID), 7, "A1", "S1", "127.0.0.1", "", 2, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if result.Token != traceID || !result.Queued {
		t.Fatalf("result = %+v, want queued %s", result, traceID)
	}
	if result.Code != seckillSuccessCode || result.Message != seckillSuccessMessage || result.SKUNo != "S1" {
		t.Fatalf("result = %+v, want Java success code/message/skuId", result)
	}
	if len(queue.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(queue.events))
	}
	if len(risk.records) != 1 {
		t.Fatalf("risk records = %d, want 1", len(risk.records))
	}
	record := risk.records[0]
	if record.UserID != 7 || record.ActionType != RiskActionSeckill || record.RiskLevel != RiskLevelNormal || record.RequestIP != "127.0.0.1" {
		t.Fatalf("risk record = %+v, want user/action/level/ip populated", record)
	}
	if record.CreatedAt.IsZero() {
		t.Fatal("risk record CreatedAt is zero")
	}
	for _, want := range []string{"activityNo=A1", "skuNo=S1", "quantity=2", "traceId=" + traceID} {
		if !strings.Contains(record.RequestInfo, want) {
			t.Fatalf("request info %q missing %q", record.RequestInfo, want)
		}
	}
}

func TestPartInDoesNotRecordSeckillActionWhenRiskDisabled(t *testing.T) {
	risk := &fakeRiskGateway{}
	queue := &fakeQueuePublisher{}
	app := NewSeckillApp(
		config.GatewayConfig{Risk: config.RiskConfig{Enabled: false}},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		risk,
		nil,
		queue,
		nil,
		&NopMachineChecker{},
		nil, // worker pool
		slog.Default(),
	)

	_, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if len(risk.records) != 0 {
		t.Fatalf("risk records = %d, want 0", len(risk.records))
	}
}

func TestPartInMarksSuspiciousWhenActivityNotOpen(t *testing.T) {
	risk := &fakeRiskGateway{}
	app := NewSeckillApp(
		config.GatewayConfig{Risk: config.RiskConfig{Enabled: true}},
		&fakeActivityGateway{detail: &ActivityDetail{ActivityNo: "A1", ActivityOpen: false}},
		nil,
		risk,
		nil,
		nil,
		nil,
		&NopMachineChecker{},
		nil, // worker pool
		slog.Default(),
	)

	result, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if result.Code != seckillClosedCode || result.Message != seckillClosedMessage {
		t.Fatalf("result = %+v, want activity closed code/message", result)
	}
	if risk.markCalls != 1 {
		t.Fatalf("mark calls = %d, want 1", risk.markCalls)
	}
	if risk.markUserID != 7 || risk.markActivityNo != "A1" || risk.markIP != "127.0.0.1" {
		t.Fatalf("mark args = user %d activity %s ip %s, want 7/A1/127.0.0.1", risk.markUserID, risk.markActivityNo, risk.markIP)
	}
}

func TestPartInDoesNotMarkSuspiciousWhenRiskDisabled(t *testing.T) {
	risk := &fakeRiskGateway{}
	app := NewSeckillApp(
		config.GatewayConfig{Risk: config.RiskConfig{Enabled: false}},
		&fakeActivityGateway{detail: &ActivityDetail{ActivityNo: "A1", ActivityOpen: false}},
		nil,
		risk,
		nil,
		nil,
		nil,
		&NopMachineChecker{},
		nil, // worker pool
		slog.Default(),
	)

	result, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if result.Code != seckillClosedCode {
		t.Fatalf("result = %+v, want activity closed code", result)
	}
	if risk.markCalls != 0 {
		t.Fatalf("mark calls = %d, want 0", risk.markCalls)
	}
}

func TestPartInBlocksWhenUserRateLimited(t *testing.T) {
	queue := &fakeQueuePublisher{}
	limiter := &fakeUserRateLimiter{allowed: false}
	app := NewSeckillApp(
		config.GatewayConfig{RateLimit: config.RateLimitConfig{UserEnabled: true, UserRate: 10, UserInterval: 10 * time.Second, UserExpire: 5 * time.Minute}},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		&fakeRiskGateway{},
		nil,
		queue,
		nil,
		&NopMachineChecker{},
		nil,
		slog.Default(),
		WithUserRateLimiter(limiter),
	)

	result, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if result.Code != seckillFailCode || result.Message != seckillFailMessage {
		t.Fatalf("result = %+v, want Java fail code/message", result)
	}
	if limiter.userID != 7 || limiter.rate != 10 || limiter.interval != 10*time.Second || limiter.ttl != 5*time.Minute {
		t.Fatalf("limiter args = user %d rate %d interval %s ttl %s", limiter.userID, limiter.rate, limiter.interval, limiter.ttl)
	}
	if len(queue.events) != 0 {
		t.Fatalf("published events = %d, want 0", len(queue.events))
	}
}

func TestPartInReturnsJavaRiskUserResult(t *testing.T) {
	queue := &fakeQueuePublisher{}
	app := NewSeckillApp(
		config.GatewayConfig{Risk: config.RiskConfig{Enabled: true}},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		&fakeRiskGateway{riskUser: true},
		nil,
		queue,
		nil,
		&NopMachineChecker{},
		nil,
		slog.Default(),
	)

	result, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if result.Code != seckillRiskUserCode || result.Message != seckillRiskUserMessage {
		t.Fatalf("result = %+v, want risk user code/message", result)
	}
	if len(queue.events) != 0 {
		t.Fatalf("published events = %d, want 0", len(queue.events))
	}
}

func TestPartInReturnsJavaStockEmptyResult(t *testing.T) {
	queue := &fakeQueuePublisher{}
	app := NewSeckillApp(
		config.GatewayConfig{},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 0},
		&fakeRiskGateway{},
		nil,
		queue,
		nil,
		&NopMachineChecker{},
		nil,
		slog.Default(),
	)

	result, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if result.Code != seckillStockEmptyCode || result.Message != seckillStockEmptyMessage {
		t.Fatalf("result = %+v, want stock empty code/message", result)
	}
	if len(queue.events) != 0 {
		t.Fatalf("published events = %d, want 0", len(queue.events))
	}
}

func TestPartInStoresQueuedTraceAfterSuccessfulPublish(t *testing.T) {
	queueState := &fakeQueueStateStore{}
	traceID := "11111111111111111111111111111111"
	app := NewSeckillApp(
		config.GatewayConfig{},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		&fakeRiskGateway{},
		nil,
		&fakeQueuePublisher{},
		nil,
		&NopMachineChecker{},
		nil,
		slog.Default(),
		WithQueueStateStore(queueState),
	)

	_, err := app.PartIn(tracing.WithTraceID(context.Background(), traceID), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if queueState.setUserID != 7 || queueState.setActivityNo != "A1" || queueState.setTraceID != traceID {
		t.Fatalf("queued state = user %d activity %s trace %s, want 7/A1/%s", queueState.setUserID, queueState.setActivityNo, queueState.setTraceID, traceID)
	}
	if queueState.setTTL != queueStateTTL {
		t.Fatalf("queued ttl = %s, want %s", queueState.setTTL, queueStateTTL)
	}
}

func TestCheckQueueByActivityUsesQueuedTraceState(t *testing.T) {
	traceID := "11111111111111111111111111111111"
	app := NewSeckillApp(
		config.GatewayConfig{},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeResultStore{result: &SeckillResult{Status: "success", OrderNo: "O1"}},
		&NopMachineChecker{},
		nil,
		slog.Default(),
		WithQueueStateStore(&fakeQueueStateStore{queuedTraceID: traceID}),
	)

	orderNo, err := app.CheckQueueByActivity(context.Background(), 7, "A1")
	if err != nil {
		t.Fatalf("CheckQueueByActivity returned error: %v", err)
	}
	if orderNo != "O1" {
		t.Fatalf("orderNo = %q, want O1", orderNo)
	}
}

type fakeActivityGateway struct {
	detail *ActivityDetail
}

func (g *fakeActivityGateway) ListActivities(context.Context) (ActivityList, error) {
	return ActivityList{}, nil
}

func (g *fakeActivityGateway) GetActivity(context.Context, string) (*ActivityDetail, error) {
	return g.detail, nil
}

func (g *fakeActivityGateway) CreateActivity(context.Context, CreateActivityRequest) (*ActivityDetail, error) {
	return g.detail, nil
}

func (g *fakeActivityGateway) UpdateActivity(context.Context, UpdateActivityRequest) error {
	return nil
}

func (g *fakeActivityGateway) EndActivity(context.Context, string) error {
	return nil
}

func (g *fakeActivityGateway) AddProduct(context.Context, AddProductRequest) error {
	return nil
}

func (g *fakeActivityGateway) RemoveProduct(context.Context, string, string) error {
	return nil
}

type fakeStockGateway struct {
	stock int64
	err   error
}

func (g *fakeStockGateway) Peek(context.Context, string, string) (int64, error) {
	return g.stock, g.err
}

func (g *fakeStockGateway) Deduct(context.Context, DeductRequest) (bool, error) {
	return true, nil
}

func (g *fakeStockGateway) Release(context.Context, string, string, string, int) error {
	return nil
}

type fakeQueuePublisher struct {
	events      []PartInEvent
	traceIDSeen string
}

type fakeUserRateLimiter struct {
	allowed  bool
	userID   int64
	rate     int
	interval time.Duration
	ttl      time.Duration
	err      error
}

func (l *fakeUserRateLimiter) TryAcquire(_ context.Context, userID int64, rate int, interval time.Duration, ttl time.Duration) (bool, error) {
	l.userID = userID
	l.rate = rate
	l.interval = interval
	l.ttl = ttl
	return l.allowed, l.err
}

type fakeQueueStateStore struct {
	setUserID     int64
	setActivityNo string
	setTraceID    string
	setTTL        time.Duration
	setErr        error
	queuedTraceID string
	getErr        error
}

func (s *fakeQueueStateStore) SetQueued(_ context.Context, userID int64, activityNo string, traceID string, ttl time.Duration) error {
	s.setUserID = userID
	s.setActivityNo = activityNo
	s.setTraceID = traceID
	s.setTTL = ttl
	return s.setErr
}

func (s *fakeQueueStateStore) GetQueuedTrace(_ context.Context, _ int64, _ string) (string, error) {
	return s.queuedTraceID, s.getErr
}

type fakeResultStore struct {
	result *SeckillResult
}

func (s *fakeResultStore) SetProcessing(context.Context, string) error      { return nil }
func (s *fakeResultStore) SetSuccess(context.Context, string, string) error { return nil }
func (s *fakeResultStore) SetFailed(context.Context, string, string) error  { return nil }
func (s *fakeResultStore) Get(context.Context, string) (*SeckillResult, error) {
	return s.result, nil
}

func (p *fakeQueuePublisher) Publish(ctx context.Context, event PartInEvent) error {
	p.traceIDSeen = tracing.TraceID(ctx)
	p.events = append(p.events, event)
	return nil
}

type fakeRiskGateway struct {
	markCalls      int
	markUserID     int64
	markActivityNo string
	markIP         string
	records        []RiskRecord
	riskUser       bool
}

func (g *fakeRiskGateway) Evaluate(context.Context, int64, string) (*RiskEvaluation, error) {
	return &RiskEvaluation{}, nil
}

func (g *fakeRiskGateway) IsRiskUser(context.Context, int64) (bool, error) {
	return g.riskUser, nil
}

func (g *fakeRiskGateway) MarkSuspicious(_ context.Context, activity *ActivityDetail, userID int64, requestIP string) error {
	g.markCalls++
	g.markUserID = userID
	if activity != nil {
		g.markActivityNo = activity.ActivityNo
	}
	g.markIP = requestIP
	return nil
}

func (g *fakeRiskGateway) RecordAction(_ context.Context, record RiskRecord) error {
	g.records = append(g.records, record)
	return nil
}

func openActivityDetail() *ActivityDetail {
	return &ActivityDetail{
		ActivityNo:   "A1",
		ActivityName: "activity",
		ActivityOpen: true,
		Products: []ProductDetail{
			{
				SKUNo:        "S1",
				ProductName:  "sku",
				SeckillPrice: 99,
			},
		},
	}
}

// --- Pipeline Acquirer tests (Task 3: Redis Pipeline 合并) ---

type fakePipelineAcquirer struct {
	allowed    bool
	err        error
	called     bool
	calledUser int64
	calledKey  string
}

func (f *fakePipelineAcquirer) TryAcquireAndSetProcessing(_ context.Context, userID int64, rate int, interval, ttl time.Duration, resultKey string) (bool, error) {
	f.called = true
	f.calledUser = userID
	f.calledKey = resultKey
	return f.allowed, f.err
}

// trackingResultStore 记录 SetProcessing 调用
type trackingResultStore struct {
	fakeResultStore
	processingCalls int
	lastTraceID     string
}

func (s *trackingResultStore) SetProcessing(_ context.Context, traceID string) error {
	s.processingCalls++
	s.lastTraceID = traceID
	return nil
}

func (s *trackingResultStore) SetFailed(_ context.Context, traceID, _ string) error {
	return nil
}

func TestFastPathUsesPipelineAcquirerWhenAvailable(t *testing.T) {
	acquirer := &fakePipelineAcquirer{allowed: true}
	pool := NewWorkerPool(
		WorkerPoolConfig{QueueSize: 100, WorkerCount: 1},
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

	app := NewSeckillApp(
		config.GatewayConfig{
			RateLimit: config.RateLimitConfig{
				UserEnabled:  true,
				UserRate:     10,
				UserInterval: 10 * time.Second,
				UserExpire:   5 * time.Minute,
			},
		},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		&fakeRiskGateway{},
		nil,
		&fakeQueuePublisher{},
		nil,
		&NopMachineChecker{},
		pool,
		slog.Default(),
		WithPipelineAcquirer(acquirer),
	)

	traceID := tracing.NewTraceID()
	partInCtx := tracing.WithTraceID(context.Background(), traceID)
	result, err := app.PartIn(partInCtx, 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if !result.Queued {
		t.Fatalf("result.Queued = false, want true")
	}

	// 验证 PipelineAcquirer 被调用
	if !acquirer.called {
		t.Fatal("PipelineAcquirer.TryAcquireAndSetProcessing was not called")
	}
	if acquirer.calledUser != 7 {
		t.Fatalf("acquirer userID = %d, want 7", acquirer.calledUser)
	}
		if !strings.HasPrefix(acquirer.calledKey, "seckill:order:result:") {
			t.Fatalf("acquirer resultKey = %q, want prefix seckill:order:result:", acquirer.calledKey)
		}
}

func TestFastPathPipelineRejectsWhenRateLimited(t *testing.T) {
	acquirer := &fakePipelineAcquirer{allowed: false}
	pool := NewWorkerPool(
		WorkerPoolConfig{QueueSize: 100, WorkerCount: 1},
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

	app := NewSeckillApp(
		config.GatewayConfig{
			RateLimit: config.RateLimitConfig{
				UserEnabled:  true,
				UserRate:     10,
				UserInterval: 10 * time.Second,
				UserExpire:   5 * time.Minute,
			},
		},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		&fakeRiskGateway{},
		nil,
		&fakeQueuePublisher{},
		nil,
		&NopMachineChecker{},
		pool,
		slog.Default(),
		WithPipelineAcquirer(acquirer),
	)

	result, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if result.Code != seckillFailCode {
		t.Fatalf("result.Code = %q, want %q (rate limited)", result.Code, seckillFailCode)
	}
	if result.Queued {
		t.Fatal("result.Queued = true, want false (should be rejected)")
	}
}

func TestFastPathPipelineFallsBackOnErr(t *testing.T) {
	// Pipeline 失败时应回退到分开调用
	acquirer := &fakePipelineAcquirer{allowed: true, err: fmt.Errorf("pipeline broken")}
	limiter := &fakeUserRateLimiter{allowed: true}
	resultStore := &trackingResultStore{}
	pool := NewWorkerPool(
		WorkerPoolConfig{QueueSize: 100, WorkerCount: 1},
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

	app := NewSeckillApp(
		config.GatewayConfig{
			RateLimit: config.RateLimitConfig{
				UserEnabled:  true,
				UserRate:     10,
				UserInterval: 10 * time.Second,
				UserExpire:   5 * time.Minute,
			},
		},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		&fakeRiskGateway{},
		nil,
		&fakeQueuePublisher{},
		resultStore,
		&NopMachineChecker{},
		pool,
		slog.Default(),
		WithPipelineAcquirer(acquirer),
		WithUserRateLimiter(limiter),
	)

	result, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if !result.Queued {
		t.Fatal("result.Queued = false, want true (fallback should succeed)")
	}
	// 验证回退后 UserRateLimiter 被调用
	if limiter.userID != 7 {
		t.Fatalf("fallback limiter userID = %d, want 7", limiter.userID)
	}
	// 验证回退后 SetProcessing 被调用
	if resultStore.processingCalls != 1 {
		t.Fatalf("SetProcessing calls = %d, want 1 (fallback)", resultStore.processingCalls)
	}
}

func TestFastPathWithoutPipelineAcquirerUsesSeparateCalls(t *testing.T) {
	limiter := &fakeUserRateLimiter{allowed: true}
	resultStore := &trackingResultStore{}
	pool := NewWorkerPool(
		WorkerPoolConfig{QueueSize: 100, WorkerCount: 1},
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

	app := NewSeckillApp(
		config.GatewayConfig{
			RateLimit: config.RateLimitConfig{
				UserEnabled:  true,
				UserRate:     10,
				UserInterval: 10 * time.Second,
				UserExpire:   5 * time.Minute,
			},
		},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		&fakeRiskGateway{},
		nil,
		&fakeQueuePublisher{},
		resultStore,
		&NopMachineChecker{},
		pool,
		slog.Default(),
		WithUserRateLimiter(limiter),
		// 注意：不提供 PipelineAcquirer
	)

	result, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn returned error: %v", err)
	}
	if !result.Queued {
		t.Fatal("result.Queued = false, want true")
	}
	// 验证旧路径：UserRateLimiter 和 SetProcessing 分别调用
	if limiter.userID != 7 {
		t.Fatalf("limiter userID = %d, want 7", limiter.userID)
	}
	if resultStore.processingCalls != 1 {
		t.Fatalf("SetProcessing calls = %d, want 1", resultStore.processingCalls)
	}
}

// mockDynamicConfig implements DynamicGatewayConfig for testing
type mockDynamicConfig struct {
	riskEnabled         bool
	machineCheckEnabled bool
}

func (m *mockDynamicConfig) RiskEnabled() bool         { return m.riskEnabled }
func (m *mockDynamicConfig) MachineCheckEnabled() bool { return m.machineCheckEnabled }

type rejectMachineChecker struct{}

func (r *rejectMachineChecker) Challenge(context.Context, int64) (MachineChallenge, error) {
	return MachineChallenge{}, nil
}
func (r *rejectMachineChecker) Check(context.Context, int64, string) bool { return false }

func TestSeckillAppDynamicRiskEnabled(t *testing.T) {
	dynCfg := &mockDynamicConfig{riskEnabled: true}
	risk := &fakeRiskGateway{riskUser: true}
	app := NewSeckillApp(
		config.GatewayConfig{Risk: config.RiskConfig{Enabled: false}},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		risk,
		nil,
		&fakeQueuePublisher{},
		nil,
		&NopMachineChecker{},
		nil,
		slog.Default(),
		WithDynamicConfig(dynCfg),
	)

	result, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn error: %v", err)
	}
	if result.Code != seckillRiskUserCode {
		t.Fatalf("result.Code = %s, want %s (risk blocked)", result.Code, seckillRiskUserCode)
	}

	dynCfg.riskEnabled = false
	result, err = app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn error: %v", err)
	}
	if result.Code != seckillSuccessCode {
		t.Fatalf("result.Code = %s, want %s (should pass)", result.Code, seckillSuccessCode)
	}
}

func TestSeckillAppDynamicMachineCheckEnabled(t *testing.T) {
	dynCfg := &mockDynamicConfig{machineCheckEnabled: true}
	app := NewSeckillApp(
		config.GatewayConfig{MachineCheck: config.MachineCheckConfig{Enabled: false}},
		&fakeActivityGateway{detail: openActivityDetail()},
		&fakeStockGateway{stock: 10},
		&fakeRiskGateway{},
		nil,
		&fakeQueuePublisher{},
		nil,
		&rejectMachineChecker{},
		nil,
		slog.Default(),
		WithDynamicConfig(dynCfg),
	)

	result, err := app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn error: %v", err)
	}
	if result.Code != seckillFailCode {
		t.Fatalf("result.Code = %s, want %s (machine check fail)", result.Code, seckillFailCode)
	}

	dynCfg.machineCheckEnabled = false
	result, err = app.PartIn(context.Background(), 7, "A1", "S1", "127.0.0.1", "", 1, "")
	if err != nil {
		t.Fatalf("PartIn error: %v", err)
	}
	if result.Code != seckillSuccessCode {
		t.Fatalf("result.Code = %s, want %s (should pass)", result.Code, seckillSuccessCode)
	}
}
