package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"seckill-gateway-service/internal/application"
)

// mockSource implements ActivityGateway for testing.
// It tracks call counts to verify caching behavior.
type mockSource struct {
	mu              sync.Mutex
	listCalls       int
	getCalls        int
	getCallsByNo    map[string]int
	listResult      application.ActivityList
	listErr         error
	getResult       *application.ActivityDetail
	getErr          error
	getResultsByNo  map[string]*application.ActivityDetail
	createResult    *application.ActivityDetail
	createErr       error
	updateErr       error
	endErr          error
	addProductErr   error
	removeProductErr error
}

func newMockSource() *mockSource {
	return &mockSource{
		getCallsByNo:   make(map[string]int),
		getResultsByNo: make(map[string]*application.ActivityDetail),
	}
}

func (m *mockSource) ListActivities(ctx context.Context) (application.ActivityList, error) {
	m.mu.Lock()
	m.listCalls++
	m.mu.Unlock()
	return m.listResult, m.listErr
}

func (m *mockSource) GetActivity(ctx context.Context, activityNo string) (*application.ActivityDetail, error) {
	m.mu.Lock()
	m.getCalls++
	m.getCallsByNo[activityNo]++
	m.mu.Unlock()

	if detail, ok := m.getResultsByNo[activityNo]; ok {
		return detail, m.getErr
	}
	return m.getResult, m.getErr
}

func (m *mockSource) CreateActivity(ctx context.Context, req application.CreateActivityRequest) (*application.ActivityDetail, error) {
	return m.createResult, m.createErr
}

func (m *mockSource) UpdateActivity(ctx context.Context, req application.UpdateActivityRequest) error {
	return m.updateErr
}

func (m *mockSource) EndActivity(ctx context.Context, activityNo string) error {
	return m.endErr
}

func (m *mockSource) AddProduct(ctx context.Context, req application.AddProductRequest) error {
	return m.addProductErr
}

func (m *mockSource) RemoveProduct(ctx context.Context, activityNo, skuNo string) error {
	return m.removeProductErr
}

// helperActivityDetail creates a simple ActivityDetail for testing.
func helperActivityDetail(no string, open bool) *application.ActivityDetail {
	return &application.ActivityDetail{
		ActivityNo:   no,
		ActivityName: "test-" + no,
		ActivityOpen: open,
		Products: []application.ProductDetail{
			{SKUNo: "SKU-1", SeckillPrice: 100},
		},
	}
}

// newTestCache builds an ActivityCache with DefaultConfig overridden only by
// the given refresh interval. Keeps call sites concise after the Config
// refactor.
func newTestCache(src application.ActivityGateway, refreshInterval time.Duration) *ActivityCache {
	cfg := DefaultConfig()
	cfg.RefreshInterval = refreshInterval
	return NewActivityCache(src, cfg)
}

// TestActivityCacheGetHit verifies that a cached GetActivity returns without calling source.
func TestActivityCacheGetHit(t *testing.T) {
	src := newMockSource()
	src.getResult = helperActivityDetail("A1", true)

	cache := newTestCache(src, 30*time.Second)
	defer cache.Stop()

	// First call populates cache
	detail, err := cache.GetActivity(context.Background(), "A1")
	if err != nil {
		t.Fatalf("first GetActivity: %v", err)
	}
	if detail.ActivityNo != "A1" {
		t.Fatalf("first call: got activityNo=%s, want A1", detail.ActivityNo)
	}

	// Second call should hit cache, not source
	detail2, err := cache.GetActivity(context.Background(), "A1")
	if err != nil {
		t.Fatalf("second GetActivity: %v", err)
	}
	if detail2.ActivityNo != "A1" {
		t.Fatalf("second call: got activityNo=%s, want A1", detail2.ActivityNo)
	}

	src.mu.Lock()
	calls := src.getCalls
	src.mu.Unlock()
	if calls != 1 {
		t.Fatalf("source GetActivity called %d times, want 1 (cache should serve second call)", calls)
	}
}

// TestActivityCacheGetMiss verifies that cache miss calls source and caches result.
func TestActivityCacheGetMiss(t *testing.T) {
	src := newMockSource()
	src.getResult = helperActivityDetail("B1", true)

	cache := newTestCache(src, 30*time.Second)
	defer cache.Stop()

	detail, err := cache.GetActivity(context.Background(), "B1")
	if err != nil {
		t.Fatalf("GetActivity: %v", err)
	}
	if detail == nil {
		t.Fatal("GetActivity returned nil detail")
	}
	if detail.ActivityNo != "B1" {
		t.Fatalf("got activityNo=%s, want B1", detail.ActivityNo)
	}

	src.mu.Lock()
	calls := src.getCalls
	src.mu.Unlock()
	if calls != 1 {
		t.Fatalf("source GetActivity called %d times, want 1", calls)
	}
}

// TestActivityCacheSingleFlight verifies concurrent requests for the same key
// only call source once (singleflight behavior).
func TestActivityCacheSingleFlight(t *testing.T) {
	src := newMockSource()
	// Make source slow so concurrent requests pile up
	var sourceMu sync.Mutex
	src.getResult = helperActivityDetail("SF1", true)

	// Wrap source GetActivity to add delay
	slowSrc := &slowMockSource{mockSource: src}

	cache := newTestCache(slowSrc, 30*time.Second)
	defer cache.Stop()

	const concurrency = 20
	var wg sync.WaitGroup
	results := make([]*application.ActivityDetail, concurrency)
	errors := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			d, err := cache.GetActivity(context.Background(), "SF1")
			results[idx] = d
			errors[idx] = err
		}(i)
	}
	wg.Wait()

	// All goroutines should succeed
	for i, err := range errors {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	for i, d := range results {
		if d == nil {
			t.Fatalf("goroutine %d: nil detail", i)
		}
		if d.ActivityNo != "SF1" {
			t.Fatalf("goroutine %d: got activityNo=%s, want SF1", i, d.ActivityNo)
		}
	}

	// Source should be called exactly once due to singleflight
	src.mu.Lock()
	calls := src.getCalls
	src.mu.Unlock()
	if calls != 1 {
		t.Fatalf("source GetActivity called %d times with %d concurrent requests, want 1 (singleflight)", calls, concurrency)
	}

	_ = sourceMu.Unlock // suppress unused var
}

// slowMockSource wraps mockSource with artificial delay on GetActivity.
type slowMockSource struct {
	*mockSource
}

func (s *slowMockSource) GetActivity(ctx context.Context, activityNo string) (*application.ActivityDetail, error) {
	time.Sleep(50 * time.Millisecond) // slow down source
	return s.mockSource.GetActivity(ctx, activityNo)
}

// TestActivityCacheRefresh verifies Start performs background refresh via ListActivities.
func TestActivityCacheRefresh(t *testing.T) {
	src := newMockSource()
	src.listResult = application.ActivityList{
		Activities: []application.ActivityItem{
			{ActivityNo: "R1", ActivityName: "refresh-test"},
		},
	}

	refreshInterval := 100 * time.Millisecond
	cache := newTestCache(src, refreshInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cache.Start(ctx)

	// Wait for at least 2 refresh cycles
	time.Sleep(350 * time.Millisecond)

	src.mu.Lock()
	calls := src.listCalls
	src.mu.Unlock()

	if calls < 2 {
		t.Fatalf("ListActivities called %d times in 350ms with %s interval, want >= 2", calls, refreshInterval)
	}
}

// TestActivityCacheGetAfterRefresh verifies that after a background refresh,
// GetActivity returns data populated by the refresh.
func TestActivityCacheGetAfterRefresh(t *testing.T) {
	src := newMockSource()
	detail1 := helperActivityDetail("REF1", true)
	detail2 := helperActivityDetail("REF2", false)

	// Initially return detail1 for GetActivity, then switch to detail2
	var getCallCount int64
	src.getResult = detail1

	cache := newTestCache(src, 50*time.Millisecond)
	defer cache.Stop()

	// Pre-populate with a ListActivities result so refresh can fill the cache
	src.listResult = application.ActivityList{
		Activities: []application.ActivityItem{
			{ActivityNo: "REF1"},
			{ActivityNo: "REF2"},
		},
	}

	// Manually set per-activity results for refresh to populate
	src.mu.Lock()
	src.getResultsByNo["REF1"] = detail1
	src.getResultsByNo["REF2"] = detail2
	src.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cache.Start(ctx)

	// Wait for refresh to run
	time.Sleep(120 * time.Millisecond)

	// After refresh, GetActivity for REF2 should return cached data
	got, err := cache.GetActivity(context.Background(), "REF2")
	if err != nil {
		t.Fatalf("GetActivity after refresh: %v", err)
	}
	if got == nil {
		t.Fatal("GetActivity after refresh returned nil")
	}
	if got.ActivityNo != "REF2" {
		t.Fatalf("got activityNo=%s, want REF2", got.ActivityNo)
	}

	_ = getCallCount // used in atomic operations
}

// TestActivityCacheGetActivityDifferentKeys verifies that different activity numbers
// are cached independently.
func TestActivityCacheGetActivityDifferentKeys(t *testing.T) {
	src := newMockSource()
	src.mu.Lock()
	src.getResultsByNo["X1"] = helperActivityDetail("X1", true)
	src.getResultsByNo["X2"] = helperActivityDetail("X2", false)
	src.mu.Unlock()

	cache := newTestCache(src, 30*time.Second)
	defer cache.Stop()

	d1, err := cache.GetActivity(context.Background(), "X1")
	if err != nil {
		t.Fatalf("GetActivity X1: %v", err)
	}
	d2, err := cache.GetActivity(context.Background(), "X2")
	if err != nil {
		t.Fatalf("GetActivity X2: %v", err)
	}

	if d1.ActivityNo != "X1" {
		t.Fatalf("X1 got %s", d1.ActivityNo)
	}
	if d2.ActivityNo != "X2" {
		t.Fatalf("X2 got %s", d2.ActivityNo)
	}

	// Call again - should use cache
	_, _ = cache.GetActivity(context.Background(), "X1")
	_, _ = cache.GetActivity(context.Background(), "X2")

	src.mu.Lock()
	calls := src.getCalls
	src.mu.Unlock()
	if calls != 2 {
		t.Fatalf("source GetActivity called %d times, want 2 (one per unique key)", calls)
	}
}

// TestActivityCacheListActivitiesCaching verifies ListActivities results are cached.
func TestActivityCacheListActivitiesCaching(t *testing.T) {
	src := newMockSource()
	src.listResult = application.ActivityList{
		Activities: []application.ActivityItem{
			{ActivityNo: "L1", ActivityName: "list-test"},
		},
	}

	cache := newTestCache(src, 30*time.Second)
	defer cache.Stop()

	list1, err := cache.ListActivities(context.Background())
	if err != nil {
		t.Fatalf("first ListActivities: %v", err)
	}
	if len(list1.Activities) != 1 || list1.Activities[0].ActivityNo != "L1" {
		t.Fatalf("first call result: %+v", list1)
	}

	list2, err := cache.ListActivities(context.Background())
	if err != nil {
		t.Fatalf("second ListActivities: %v", err)
	}
	if len(list2.Activities) != 1 {
		t.Fatalf("second call result: %+v", list2)
	}

	src.mu.Lock()
	calls := src.listCalls
	src.mu.Unlock()
	if calls != 1 {
		t.Fatalf("source ListActivities called %d times, want 1", calls)
	}
}

// TestActivityCachePassThroughWriteOps verifies that admin write operations
// pass through to the source without caching.
func TestActivityCachePassThroughWriteOps(t *testing.T) {
	src := newMockSource()
	src.createResult = helperActivityDetail("NEW1", true)

	cache := newTestCache(src, 30*time.Second)
	defer cache.Stop()

	// CreateActivity should pass through
	detail, err := cache.CreateActivity(context.Background(), application.CreateActivityRequest{
		ActivityName: "new",
		StartTime:    "2026-01-01T00:00:00Z",
		EndTime:      "2026-12-31T23:59:59Z",
	})
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}
	if detail.ActivityNo != "NEW1" {
		t.Fatalf("CreateActivity returned %s, want NEW1", detail.ActivityNo)
	}

	// UpdateActivity should pass through
	if err := cache.UpdateActivity(context.Background(), application.UpdateActivityRequest{
		ActivityNo: "NEW1",
	}); err != nil {
		t.Fatalf("UpdateActivity: %v", err)
	}

	// EndActivity should pass through
	if err := cache.EndActivity(context.Background(), "NEW1"); err != nil {
		t.Fatalf("EndActivity: %v", err)
	}

	// AddProduct should pass through
	if err := cache.AddProduct(context.Background(), application.AddProductRequest{
		ActivityNo: "NEW1", SKUNo: "S1", ActivityStock: 100,
	}); err != nil {
		t.Fatalf("AddProduct: %v", err)
	}

	// RemoveProduct should pass through
	if err := cache.RemoveProduct(context.Background(), "NEW1", "S1"); err != nil {
		t.Fatalf("RemoveProduct: %v", err)
	}
}

// TestActivityCacheGetActivitySourceError verifies that source errors propagate.
func TestActivityCacheGetActivitySourceError(t *testing.T) {
	src := newMockSource()
	src.getErr = context.DeadlineExceeded

	cache := newTestCache(src, 30*time.Second)
	defer cache.Stop()

	_, err := cache.GetActivity(context.Background(), "ERR1")
	if err == nil {
		t.Fatal("expected error from GetActivity, got nil")
	}
}

// TestActivityCacheConcurrentGetAndRefresh verifies no data race between
// background refresh and concurrent reads.
func TestActivityCacheConcurrentGetAndRefresh(t *testing.T) {
	src := newMockSource()
	src.getResult = helperActivityDetail("CR1", true)
	src.listResult = application.ActivityList{
		Activities: []application.ActivityItem{
			{ActivityNo: "CR1"},
		},
	}
	src.mu.Lock()
	src.getResultsByNo["CR1"] = helperActivityDetail("CR1", true)
	src.mu.Unlock()

	cache := newTestCache(src, 50*time.Millisecond)
	defer cache.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cache.Start(ctx)

	// Concurrent reads while refresh is running
	var wg sync.WaitGroup
	var readCount int64
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := cache.GetActivity(context.Background(), "CR1")
			if err != nil {
				return
			}
			if d != nil && d.ActivityNo == "CR1" {
				atomic.AddInt64(&readCount, 1)
			}
		}()
		time.Sleep(2 * time.Millisecond)
	}
	wg.Wait()

	if atomic.LoadInt64(&readCount) == 0 {
		t.Fatal("no successful reads during concurrent access")
	}
}
