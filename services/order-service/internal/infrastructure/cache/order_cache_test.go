package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/domain/repository"
)

type fakeStore struct {
	mu              sync.Mutex
	orders          map[string]entity.Order
	getCalls        int32
	createCalls     int32
	markPaidCalls   int32
	closeCalls      int32
	listActCalls    int32
	listActsCalls   int32
	listUserCalls   int32
	getErr          error
	createErr       error
	markPaidErr     error
	closeErr        error
}

func newFakeStore() *fakeStore {
	return &fakeStore{orders: map[string]entity.Order{}}
}

func newTestCache(t *testing.T, source repository.OrderStore) *OrderCache {
	t.Helper()
	c, err := NewOrderCache(source, DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewOrderCache: %v", err)
	}
	return c
}

func (s *fakeStore) CreateOrder(_ context.Context, order entity.Order) error {
	atomic.AddInt32(&s.createCalls, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return s.createErr
	}
	if _, ok := s.orders[order.OrderNo]; ok {
		return errors.New("duplicate")
	}
	s.orders[order.OrderNo] = order
	return nil
}

func (s *fakeStore) GetOrder(_ context.Context, orderNo string) (entity.Order, error) {
	atomic.AddInt32(&s.getCalls, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getErr != nil {
		return entity.Order{}, s.getErr
	}
	o, ok := s.orders[orderNo]
	if !ok {
		return entity.Order{}, errors.New("not found")
	}
	return o, nil
}

// GetByUserAndTrace 匹配 OrderStore 接口；测试中暂不验证该路径
func (s *fakeStore) GetByUserAndTrace(_ context.Context, _ int64, _ string) (entity.Order, error) {
	return entity.Order{}, errors.New("not found")
}

func (s *fakeStore) ListOrdersByActivity(_ context.Context, activityNo string) ([]entity.Order, error) {
	atomic.AddInt32(&s.listActCalls, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]entity.Order, 0)
	for _, o := range s.orders {
		if o.ActivityNo == activityNo {
			out = append(out, o)
		}
	}
	return out, nil
}

func (s *fakeStore) ListOrdersByActivities(_ context.Context, activityNos []string) (map[string][]entity.Order, error) {
	atomic.AddInt32(&s.listActsCalls, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]entity.Order)
	for _, o := range s.orders {
		for _, a := range activityNos {
			if o.ActivityNo == a {
				out[a] = append(out[a], o)
			}
		}
	}
	return out, nil
}

func (s *fakeStore) ListOrdersByUser(_ context.Context, userID int64) ([]entity.Order, error) {
	atomic.AddInt32(&s.listUserCalls, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]entity.Order, 0)
	for _, o := range s.orders {
		if o.UserID == userID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (s *fakeStore) MarkOrderPaid(_ context.Context, orderNo string, _ string, _ time.Time) error {
	atomic.AddInt32(&s.markPaidCalls, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.markPaidErr != nil {
		return s.markPaidErr
	}
	o, ok := s.orders[orderNo]
	if !ok {
		return errors.New("not found")
	}
	o.Status = entity.OrderPaid
	s.orders[orderNo] = o
	return nil
}

func (s *fakeStore) CloseOrder(_ context.Context, orderNo string) error {
	atomic.AddInt32(&s.closeCalls, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closeErr != nil {
		return s.closeErr
	}
	o, ok := s.orders[orderNo]
	if !ok {
		return errors.New("not found")
	}
	o.Status = entity.OrderClosed
	s.orders[orderNo] = o
	return nil
}

func sampleOrder(orderNo string) entity.Order {
	return entity.Order{
		OrderNo:    orderNo,
		UserID:     1001,
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		Quantity:   1,
		PayAmount:  9900,
		Status:     entity.OrderPending,
		TraceID:    "trace-1",
		CreatedAt:  time.Now(),
	}
}

func TestOrderCache_GetOrder_MissThenHit(t *testing.T) {
	store := newFakeStore()
	store.orders["TEST001"] = sampleOrder("TEST001")
	c := newTestCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	first, err := c.GetOrder(ctx, "TEST001")
	if err != nil {
		t.Fatalf("first GetOrder: %v", err)
	}
	if first.OrderNo != "TEST001" {
		t.Fatalf("unexpected order: %+v", first)
	}
	if got := atomic.LoadInt32(&store.getCalls); got != 1 {
		t.Fatalf("after first get, source calls = %d, want 1", got)
	}

	second, err := c.GetOrder(ctx, "TEST001")
	if err != nil {
		t.Fatalf("second GetOrder: %v", err)
	}
	if second.OrderNo != "TEST001" {
		t.Fatalf("unexpected cached order: %+v", second)
	}
	if got := atomic.LoadInt32(&store.getCalls); got != 1 {
		t.Fatalf("after second get, source calls = %d, want 1 (should hit cache)", got)
	}
}

func TestOrderCache_GetOrder_SourceError(t *testing.T) {
	store := newFakeStore()
	srcErr := errors.New("db down")
	store.getErr = srcErr
	c := newTestCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	if _, err := c.GetOrder(ctx, "TEST001"); !errors.Is(err, srcErr) {
		t.Fatalf("expected source error, got %v", err)
	}
	if got := atomic.LoadInt32(&store.getCalls); got != 1 {
		t.Fatalf("source calls = %d, want 1", got)
	}

	store.getErr = nil
	store.orders["TEST001"] = sampleOrder("TEST001")
	if _, err := c.GetOrder(ctx, "TEST001"); err != nil {
		t.Fatalf("after recovery, GetOrder should succeed, got %v", err)
	}
	if got := atomic.LoadInt32(&store.getCalls); got != 2 {
		t.Fatalf("source calls = %d, want 2 (error must not be cached)", got)
	}
}

func TestOrderCache_CreateOrder_Invalidates(t *testing.T) {
	store := newFakeStore()
	c := newTestCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	store.orders["TEST001"] = sampleOrder("TEST001")
	if _, err := c.GetOrder(ctx, "TEST001"); err != nil {
		t.Fatalf("prime cache: %v", err)
	}
	if got := atomic.LoadInt32(&store.getCalls); got != 1 {
		t.Fatalf("primer calls = %d, want 1", got)
	}

	// CreateOrder for the same key must succeed (fakeStore allows re-insert of
	// same shape since we delete first) and invalidate the cached entry.
	delete(store.orders, "TEST001")
	if err := c.CreateOrder(ctx, sampleOrder("TEST001")); err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	// Re-read: cache should have been evicted, so source must be hit again.
	if _, err := c.GetOrder(ctx, "TEST001"); err != nil {
		t.Fatalf("GetOrder after CreateOrder: %v", err)
	}
	if got := atomic.LoadInt32(&store.getCalls); got != 2 {
		t.Fatalf("after CreateOrder-invalidate, source calls = %d, want 2", got)
	}
}

func TestOrderCache_MarkOrderPaid_Invalidates(t *testing.T) {
	store := newFakeStore()
	store.orders["TEST001"] = sampleOrder("TEST001")
	c := newTestCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	if _, err := c.GetOrder(ctx, "TEST001"); err != nil {
		t.Fatalf("prime cache: %v", err)
	}
	if got := atomic.LoadInt32(&store.getCalls); got != 1 {
		t.Fatalf("primer calls = %d, want 1", got)
	}

	if err := c.MarkOrderPaid(ctx, "TEST001", "TX001", time.Now()); err != nil {
		t.Fatalf("MarkOrderPaid: %v", err)
	}
	if got := atomic.LoadInt32(&store.getCalls); got != 1 {
		t.Fatalf("MarkOrderPaid should not call GetOrder, calls = %d", got)
	}

	got, err := c.GetOrder(ctx, "TEST001")
	if err != nil {
		t.Fatalf("GetOrder after MarkOrderPaid: %v", err)
	}
	if got.Status != entity.OrderPaid {
		t.Fatalf("cached order status not updated, got %s", got.Status)
	}
	if calls := atomic.LoadInt32(&store.getCalls); calls != 2 {
		t.Fatalf("after MarkOrderPaid, source calls = %d, want 2 (cache must be evicted)", calls)
	}
}

func TestOrderCache_CloseOrder_Invalidates(t *testing.T) {
	store := newFakeStore()
	store.orders["TEST001"] = sampleOrder("TEST001")
	c := newTestCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	if _, err := c.GetOrder(ctx, "TEST001"); err != nil {
		t.Fatalf("prime cache: %v", err)
	}

	if err := c.CloseOrder(ctx, "TEST001"); err != nil {
		t.Fatalf("CloseOrder: %v", err)
	}

	got, err := c.GetOrder(ctx, "TEST001")
	if err != nil {
		t.Fatalf("GetOrder after CloseOrder: %v", err)
	}
	if got.Status != entity.OrderClosed {
		t.Fatalf("cached order status not updated, got %s", got.Status)
	}
	if calls := atomic.LoadInt32(&store.getCalls); calls != 2 {
		t.Fatalf("after CloseOrder, source calls = %d, want 2 (cache must be evicted)", calls)
	}
}

func TestOrderCache_PassthroughMethods(t *testing.T) {
	store := newFakeStore()
	store.orders["TEST001"] = sampleOrder("TEST001")
	c := newTestCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	if _, err := c.ListOrdersByActivity(ctx, "ACT001"); err != nil {
		t.Fatalf("ListOrdersByActivity: %v", err)
	}
	if got := atomic.LoadInt32(&store.listActCalls); got != 1 {
		t.Fatalf("ListOrdersByActivity source calls = %d, want 1", got)
	}
	if _, err := c.ListOrdersByActivities(ctx, []string{"ACT001"}); err != nil {
		t.Fatalf("ListOrdersByActivities: %v", err)
	}
	if got := atomic.LoadInt32(&store.listActsCalls); got != 1 {
		t.Fatalf("ListOrdersByActivities source calls = %d, want 1", got)
	}
	if _, err := c.ListOrdersByUser(ctx, 1001); err != nil {
		t.Fatalf("ListOrdersByUser: %v", err)
	}
	if got := atomic.LoadInt32(&store.listUserCalls); got != 1 {
		t.Fatalf("ListOrdersByUser source calls = %d, want 1", got)
	}
}

func TestOrderCache_Concurrent(t *testing.T) {
	store := newFakeStore()
	store.orders["TEST001"] = sampleOrder("TEST001")
	c := newTestCache(t, store)
	defer func() { _ = c.Close() }()

	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make(chan error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			<-start
			o, err := c.GetOrder(context.Background(), "TEST001")
			if err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", idx, err)
				return
			}
			if o.OrderNo != "TEST001" {
				errs <- fmt.Errorf("goroutine %d: wrong order %s", idx, o.OrderNo)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}

	calls := atomic.LoadInt32(&store.getCalls)
	if calls < 1 || calls > n {
		t.Fatalf("concurrent source calls = %d, want in [1, %d]", calls, n)
	}
	t.Logf("concurrent GetOrder: %d source calls across %d goroutines", calls, n)
}

func TestNewOrderCache_NilSource(t *testing.T) {
	if _, err := NewOrderCache(nil, DefaultConfig(), nil); err == nil {
		t.Fatal("expected error for nil source")
	}
}
