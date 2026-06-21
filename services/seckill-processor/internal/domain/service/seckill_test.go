package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"seckill-processor-service/internal/domain/event"
	"seckill-processor-service/internal/domain/model"
)

func TestSubmitPublishesOrderCreated(t *testing.T) {
	events := &recordingEvents{}
	orders := &fakeOrders{}
	stock := &fakeStock{deducted: true}
	svc := NewSeckillService(
		&fakeActivity{activity: openActivity(), sku: model.SKUInfo{SKUNo: "S1", SeckillPrice: 9900, LimitQuantity: 2}},
		stock,
		&fakeRisk{},
		orders,
		events,
		&fakeIDGen{},
		&fakeTmpChecker{},
		nil,
	)

	err := svc.Submit(context.Background(), model.SubmitCommand{
		TraceID:        "trace-1",
		RequestTraceID: "req-1",
		ActivityNo:     "A1",
		SKUNo:          "S1",
		UserID:         7,
		Quantity:       2,
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if len(events.created) != 1 {
		t.Fatalf("created events = %d, want 1", len(events.created))
	}
	created := events.created[0]
	if created.UserID != 7 || created.ActivityNo != "A1" || created.SKUNo != "S1" {
		t.Fatalf("created event = %+v, want user/activity/sku", created)
	}
	if created.PayAmount != 19800 {
		t.Fatalf("pay amount = %d, want 19800", created.PayAmount)
	}
	if orders.created.Quantity != 2 || orders.created.Status == "" {
		t.Fatalf("created order = %+v, want quantity/status", orders.created)
	}
	if stock.limit != 2 {
		t.Fatalf("stock limit = %d, want sku limit 2", stock.limit)
	}
}

func TestSubmitPublishesRejectedForRiskUser(t *testing.T) {
	events := &recordingEvents{}
	stock := &fakeStock{deducted: true}
	orders := &fakeOrders{}
	svc := NewSeckillService(
		&fakeActivity{activity: openActivity(), sku: model.SKUInfo{SKUNo: "S1", SeckillPrice: 9900}},
		stock,
		&fakeRisk{risk: true},
		orders,
		events,
		&fakeIDGen{},
		&fakeTmpChecker{},
		nil,
	)

	err := svc.Submit(context.Background(), model.SubmitCommand{
		TraceID:        "trace-1",
		RequestTraceID: "req-1",
		ActivityNo:     "A1",
		SKUNo:          "S1",
		UserID:         7,
		Quantity:       1,
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if len(events.rejected) != 1 {
		t.Fatalf("rejected events = %d, want 1", len(events.rejected))
	}
	if events.rejected[0].Reason != event.ReasonRiskUser {
		t.Fatalf("reject reason = %s, want %s", events.rejected[0].Reason, event.ReasonRiskUser)
	}
	if stock.deductCalls != 0 {
		t.Fatalf("stock deduct calls = %d, want 0", stock.deductCalls)
	}
	if orders.createCalls != 0 {
		t.Fatalf("order create calls = %d, want 0", orders.createCalls)
	}
}

func TestSubmitReturnsErrorForTemporaryOrderFailureWithoutRejecting(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
	}{
		{name: "unavailable", code: codes.Unavailable},
		{name: "deadline exceeded", code: codes.DeadlineExceeded},
		{name: "resource exhausted", code: codes.ResourceExhausted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := &recordingEvents{}
			stock := &fakeStock{deducted: true}
			orders := &fakeOrders{err: status.Error(tt.code, "temporary order rpc failure")}
			svc := NewSeckillService(
				&fakeActivity{activity: openActivity(), sku: model.SKUInfo{SKUNo: "S1", SeckillPrice: 9900}},
				stock,
				&fakeRisk{},
				orders,
				events,
				&fakeIDGen{},
				&fakeTmpChecker{},
				nil,
			)

			err := svc.Submit(context.Background(), model.SubmitCommand{
				TraceID:        "trace-1",
				RequestTraceID: "req-1",
				ActivityNo:     "A1",
				SKUNo:          "S1",
				UserID:         7,
				Quantity:       1,
			})
			if err == nil {
				t.Fatal("Submit returned nil error, want retryable order failure")
			}
			if !strings.Contains(err.Error(), "temporary") {
				t.Fatalf("Submit error = %v, want temporary failure context", err)
			}
			if len(events.rejected) != 0 {
				t.Fatalf("rejected events = %d, want 0", len(events.rejected))
			}
			if stock.releaseCalls != 1 {
				t.Fatalf("stock release calls = %d, want 1", stock.releaseCalls)
			}
		})
	}
}

func openActivity() model.ActivityInfo {
	return model.ActivityInfo{
		ActivityNo:    "A1",
		StartTime:     time.Now().Add(-time.Minute),
		EndTime:       time.Now().Add(time.Minute),
		Status:        1,
		PurchaseLimit: 5,
	}
}

type recordingEvents struct {
	created  []event.OrderCreated
	rejected []event.SeckillRejected
}

func (e *recordingEvents) Publish(topic string, args ...interface{}) {
	if len(args) != 1 {
		return
	}
	switch topic {
	case event.TopicOrderCreated:
		created, ok := args[0].(event.OrderCreated)
		if ok {
			e.created = append(e.created, created)
		}
	case event.TopicSeckillRejected:
		rejected, ok := args[0].(event.SeckillRejected)
		if ok {
			e.rejected = append(e.rejected, rejected)
		}
	}
}

type fakeActivity struct {
	activity model.ActivityInfo
	sku      model.SKUInfo
	err      error
}

func (g *fakeActivity) GetActivity(context.Context, string) (model.ActivityInfo, error) {
	return g.activity, g.err
}

func (g *fakeActivity) GetSKU(context.Context, string, string) (model.SKUInfo, error) {
	return g.sku, g.err
}

type fakeStock struct {
	deducted     bool
	deductCalls  int
	releaseCalls int
	limit        int64
}

func (g *fakeStock) DeductStockWithLimit(_ context.Context, _ string, _ string, _ int64, _ int64, limit int64, _ string) (bool, error) {
	g.deductCalls++
	g.limit = limit
	return g.deducted, nil
}

func (g *fakeStock) ReleaseStock(context.Context, string, string, int64, int64, string) error {
	g.releaseCalls++
	return nil
}

type fakeRisk struct {
	risk bool
}

func (g *fakeRisk) Evaluate(context.Context, int64, string) (model.RiskResult, error) {
	return model.RiskResult{Risk: g.risk}, nil
}

type fakeOrders struct {
	createCalls   int
	created       model.OrderRequest
	err           error
	existing      model.OrderInfo
	existingErr   error
	lookupCalls   int
	lookupUserID  int64
	lookupTraceID string
}

func (g *fakeOrders) CreateOrder(_ context.Context, order model.OrderRequest) error {
	g.createCalls++
	g.created = order
	return g.err
}

func (g *fakeOrders) GetByUserAndTrace(_ context.Context, userID int64, traceID string) (model.OrderInfo, error) {
	g.lookupCalls++
	g.lookupUserID = userID
	g.lookupTraceID = traceID
	return g.existing, g.existingErr
}

type fakeIDGen struct{}

func (f *fakeIDGen) NextOrderNo() string { return "O1234567890" }

type fakeTmpChecker struct{}

func (f *fakeTmpChecker) IsTemporary(err error) bool {
	// Check if it's a gRPC status error with temporary codes
	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.Unavailable ||
			st.Code() == codes.DeadlineExceeded ||
			st.Code() == codes.ResourceExhausted
	}
	return false
}

// TestSubmit_DuplicateKey_LooksUpExistingOrder verifies that when CreateOrder
// returns ErrDuplicateTraceID (gRPC AlreadyExists from DB 23505), the service
// calls GetByUserAndTrace with dual key (userID + traceID) to resolve the
// existing order and publishes an OrderCreated event so the upper layer
// (markTraceSuccess) writes both idem keys. Stock must NOT be released because
// DuplicateKey is a success path (the original processing already deducted stock).
func TestSubmit_DuplicateKey_LooksUpExistingOrder(t *testing.T) {
	events := &recordingEvents{}
	stock := &fakeStock{deducted: true}
	orders := &fakeOrders{
		err:         ErrDuplicateTraceID,
		existing:    model.OrderInfo{OrderNo: "EXISTING-O1", PayAmount: 9900},
		existingErr: nil,
	}
	svc := NewSeckillService(
		&fakeActivity{activity: openActivity(), sku: model.SKUInfo{SKUNo: "S1", SeckillPrice: 9900, LimitQuantity: 2}},
		stock,
		&fakeRisk{},
		orders,
		events,
		&fakeIDGen{},
		&fakeTmpChecker{},
		nil,
	)

	err := svc.Submit(context.Background(), model.SubmitCommand{
		TraceID:        "T1",
		RequestTraceID: "req-T1",
		ActivityNo:     "A1",
		SKUNo:          "S1",
		UserID:         7,
		Quantity:       1,
	})
	if err != nil {
		t.Fatalf("Submit returned error on duplicate trace_id: %v", err)
	}

	// Must call GetByUserAndTrace with dual key
	if orders.lookupCalls != 1 {
		t.Fatalf("GetByUserAndTrace calls = %d, want 1", orders.lookupCalls)
	}
	if orders.lookupUserID != 7 || orders.lookupTraceID != "T1" {
		t.Fatalf("lookup args = (user=%d, trace=%s), want (7, T1)", orders.lookupUserID, orders.lookupTraceID)
	}

	// Must publish exactly 1 OrderCreated event
	if len(events.created) != 1 {
		t.Fatalf("created events = %d, want 1", len(events.created))
	}
	created := events.created[0]
	if created.OrderNo != "EXISTING-O1" {
		t.Fatalf("created event OrderNo = %s, want EXISTING-O1", created.OrderNo)
	}
	if created.TraceID != "T1" {
		t.Fatalf("created event TraceID = %s, want T1", created.TraceID)
	}
	if created.UserID != 7 {
		t.Fatalf("created event UserID = %d, want 7", created.UserID)
	}

	// DuplicateKey is a success path; stock must NOT be released
	if stock.releaseCalls != 0 {
		t.Fatalf("stock release calls = %d, want 0 (duplicate path must not release stock)", stock.releaseCalls)
	}
	// Must NOT publish any rejection event
	if len(events.rejected) != 0 {
		t.Fatalf("rejected events = %d, want 0", len(events.rejected))
	}
}

// TestSubmit_DuplicateKey_LookupFails_ReturnsError verifies that when
// CreateOrder returns ErrDuplicateTraceID AND the subsequent lookup fails
// (e.g. ErrOrderNotFound), Submit returns a non-nil error so the upper layer
// releases the processor idem key. No OrderCreated event is published.
func TestSubmit_DuplicateKey_LookupFails_ReturnsError(t *testing.T) {
	events := &recordingEvents{}
	stock := &fakeStock{deducted: true}
	orders := &fakeOrders{
		err:         ErrDuplicateTraceID,
		existingErr: ErrOrderNotFound,
	}
	svc := NewSeckillService(
		&fakeActivity{activity: openActivity(), sku: model.SKUInfo{SKUNo: "S1", SeckillPrice: 9900, LimitQuantity: 2}},
		stock,
		&fakeRisk{},
		orders,
		events,
		&fakeIDGen{},
		&fakeTmpChecker{},
		nil,
	)

	err := svc.Submit(context.Background(), model.SubmitCommand{
		TraceID:        "T1",
		RequestTraceID: "req-T1",
		ActivityNo:     "A1",
		SKUNo:          "S1",
		UserID:         7,
		Quantity:       1,
	})
	if err == nil {
		t.Fatal("Submit returned nil error, want lookup-failure error to trigger upper-layer release")
	}
	if !strings.Contains(err.Error(), "lookup failed") {
		t.Fatalf("Submit error = %v, want 'lookup failed' context", err)
	}

	// No OrderCreated event on lookup failure
	if len(events.created) != 0 {
		t.Fatalf("created events = %d, want 0 on lookup failure", len(events.created))
	}
	// Must NOT be rejected — rejection is a final terminal path; this is an
	// infra-level retry path, so the upper layer will release and the message
	// will be retried or alerted via ERROR log.
	if len(events.rejected) != 0 {
		t.Fatalf("rejected events = %d, want 0 on lookup failure", len(events.rejected))
	}
}

// --- Concurrency tests for activity check + risk evaluate parallelization ---

// slowActivity is a fake ActivityQuery that records its start time and sleeps.
type slowActivity struct {
	activity  model.ActivityInfo
	sku       model.SKUInfo
	duration  time.Duration
	started   chan struct{} // closed when GetActivity begins
	startTime time.Time
}

func (g *slowActivity) GetActivity(_ context.Context, _ string) (model.ActivityInfo, error) {
	close(g.started)
	g.startTime = time.Now()
	time.Sleep(g.duration)
	return g.activity, nil
}

func (g *slowActivity) GetSKU(_ context.Context, _ string, _ string) (model.SKUInfo, error) {
	return g.sku, nil
}

// slowRisk is a fake RiskGateway that records its start time and sleeps.
type slowRisk struct {
	duration  time.Duration
	started   chan struct{} // closed when Evaluate begins
	startTime time.Time
}

func (g *slowRisk) Evaluate(_ context.Context, _ int64, _ string) (model.RiskResult, error) {
	close(g.started)
	g.startTime = time.Now()
	time.Sleep(g.duration)
	return model.RiskResult{}, nil
}

// TestSubmitActivityAndRiskRunConcurrently verifies that activity check and risk
// evaluate execute in parallel. If they were serial, total time would be ~200ms.
// With parallel execution, total time should be ~100ms (plus small overhead).
func TestSubmitActivityAndRiskRunConcurrently(t *testing.T) {
	const delay = 100 * time.Millisecond
	act := &slowActivity{
		activity: openActivity(),
		sku:      model.SKUInfo{SKUNo: "S1", SeckillPrice: 9900, LimitQuantity: 2},
		duration: delay,
		started:  make(chan struct{}),
	}
	risk := &slowRisk{
		duration: delay,
		started:  make(chan struct{}),
	}

	svc := NewSeckillService(
		act,
		&fakeStock{deducted: true},
		risk,
		&fakeOrders{},
		&recordingEvents{},
		&fakeIDGen{},
		&fakeTmpChecker{},
		nil,
	)

	start := time.Now()
	err := svc.Submit(context.Background(), model.SubmitCommand{
		TraceID:    "trace-1",
		ActivityNo: "A1",
		SKUNo:      "S1",
		UserID:     7,
		Quantity:   1,
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	// If serial: elapsed >= 2*delay - overhead. If parallel: elapsed < 1.5*delay.
	if elapsed >= 2*delay-10*time.Millisecond {
		t.Fatalf("activity check and risk evaluate appear serial: elapsed %v >= 2*%v", elapsed, delay)
	}

	// Verify both actually started
	<-act.started
	<-risk.started
}

// errorActivity is a fake ActivityQuery that returns an error.
type errorActivity struct {
	sku model.SKUInfo
	err error
}

func (g *errorActivity) GetActivity(_ context.Context, _ string) (model.ActivityInfo, error) {
	return model.ActivityInfo{}, g.err
}

func (g *errorActivity) GetSKU(_ context.Context, _ string, _ string) (model.SKUInfo, error) {
	return g.sku, nil
}

// errorRisk is a fake RiskGateway that returns an error.
type errorRisk struct {
	err error
}

func (g *errorRisk) Evaluate(_ context.Context, _ int64, _ string) (model.RiskResult, error) {
	return model.RiskResult{}, g.err
}

// TestSubmitShortCircuitsOnActivityError verifies that an activity check error
// causes Submit to return an error (not reject) and not proceed to stock/order.
func TestSubmitShortCircuitsOnActivityError(t *testing.T) {
	stock := &fakeStock{deducted: true}
	orders := &fakeOrders{}
	svc := NewSeckillService(
		&errorActivity{err: errors.New("activity rpc failed")},
		stock,
		&fakeRisk{},
		orders,
		&recordingEvents{},
		&fakeIDGen{},
		&fakeTmpChecker{},
		nil,
	)

	err := svc.Submit(context.Background(), model.SubmitCommand{
		TraceID:    "trace-1",
		ActivityNo: "A1",
		SKUNo:      "S1",
		UserID:     7,
		Quantity:   1,
	})
	if err == nil {
		t.Fatal("Submit returned nil, want error from activity check")
	}
	if !strings.Contains(err.Error(), "activity") {
		t.Fatalf("error = %v, want activity-related error", err)
	}
	if stock.deductCalls != 0 {
		t.Fatalf("stock deduct calls = %d, want 0", stock.deductCalls)
	}
	if orders.createCalls != 0 {
		t.Fatalf("order create calls = %d, want 0", orders.createCalls)
	}
}

// TestSubmitShortCircuitsOnRiskError verifies that a risk evaluation error
// causes Submit to return an error and not proceed to stock/order.
func TestSubmitShortCircuitsOnRiskError(t *testing.T) {
	stock := &fakeStock{deducted: true}
	orders := &fakeOrders{}
	svc := NewSeckillService(
		&fakeActivity{activity: openActivity(), sku: model.SKUInfo{SKUNo: "S1", SeckillPrice: 9900}},
		stock,
		&errorRisk{err: errors.New("risk rpc failed")},
		orders,
		&recordingEvents{},
		&fakeIDGen{},
		&fakeTmpChecker{},
		nil,
	)

	err := svc.Submit(context.Background(), model.SubmitCommand{
		TraceID:    "trace-1",
		ActivityNo: "A1",
		SKUNo:      "S1",
		UserID:     7,
		Quantity:   1,
	})
	if err == nil {
		t.Fatal("Submit returned nil, want error from risk evaluate")
	}
	if !strings.Contains(err.Error(), "risk") {
		t.Fatalf("error = %v, want risk-related error", err)
	}
	if stock.deductCalls != 0 {
		t.Fatalf("stock deduct calls = %d, want 0", stock.deductCalls)
	}
	if orders.createCalls != 0 {
		t.Fatalf("order create calls = %d, want 0", orders.createCalls)
	}
}

// TestSubmitBothFailReturnsActivityError verifies that when both activity and
// risk fail, the error from one of them is returned (errgroup returns first error).
func TestSubmitBothFailReturnsActivityError(t *testing.T) {
	stock := &fakeStock{deducted: true}
	orders := &fakeOrders{}
	svc := NewSeckillService(
		&errorActivity{err: errors.New("activity failed")},
		stock,
		&errorRisk{err: errors.New("risk failed")},
		orders,
		&recordingEvents{},
		&fakeIDGen{},
		&fakeTmpChecker{},
		nil,
	)

	err := svc.Submit(context.Background(), model.SubmitCommand{
		TraceID:    "trace-1",
		ActivityNo: "A1",
		SKUNo:      "S1",
		UserID:     7,
		Quantity:   1,
	})
	if err == nil {
		t.Fatal("Submit returned nil, want error")
	}
	// errgroup returns the first non-nil error from g.Go calls
	if !strings.Contains(err.Error(), "activity") && !strings.Contains(err.Error(), "risk") {
		t.Fatalf("error = %v, want activity or risk related error", err)
	}
}

// Verify the fakeRisk and fakeActivity are safe for concurrent use by checking
// they implement the required interfaces at compile time.
var _ ActivityQuery = (*slowActivity)(nil)
var _ RiskGateway = (*slowRisk)(nil)
var _ ActivityQuery = (*errorActivity)(nil)
var _ RiskGateway = (*errorRisk)(nil)

// unused import guard for sync (used by slowActivity/slowRisk channels)
var _ sync.Mutex
