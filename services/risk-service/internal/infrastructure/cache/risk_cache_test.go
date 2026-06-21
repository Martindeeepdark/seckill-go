package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"seckill-risk-service/internal/domain/entity"
	"seckill-risk-service/internal/domain/repository"
)

type fakeRiskStore struct {
	mu               sync.Mutex
	riskUsers        map[int64]bool
	isRiskCalls      int32
	markRiskCalls    int32
	recordCalls      int32
	countCalls       int32
	highRiskCalls    int32
	listCalls        int32
	cleanupCalls     int32
	isRiskErr        error
	markRiskErr      error
}

func newFakeRiskStore() *fakeRiskStore {
	return &fakeRiskStore{riskUsers: map[int64]bool{}}
}

func newTestRiskCache(t *testing.T, source repository.RiskRepository) *RiskCache {
	t.Helper()
	c, err := NewRiskCache(source, DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewRiskCache: %v", err)
	}
	return c
}

func (s *fakeRiskStore) MarkRiskUser(_ context.Context, userID int64, _ time.Duration) error {
	atomic.AddInt32(&s.markRiskCalls, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.markRiskErr != nil {
		return s.markRiskErr
	}
	s.riskUsers[userID] = true
	return nil
}

func (s *fakeRiskStore) IsRiskUser(_ context.Context, userID int64) (bool, error) {
	atomic.AddInt32(&s.isRiskCalls, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isRiskErr != nil {
		return false, s.isRiskErr
	}
	return s.riskUsers[userID], nil
}

func (s *fakeRiskStore) RecordRiskAction(_ context.Context, _ entity.RiskRecord) error {
	atomic.AddInt32(&s.recordCalls, 1)
	return nil
}

func (s *fakeRiskStore) CountRecentRiskActions(_ context.Context, _ int64, _ string, _ time.Time) (int, error) {
	atomic.AddInt32(&s.countCalls, 1)
	return 0, nil
}

func (s *fakeRiskStore) HasHighRiskRecord(_ context.Context, _ int64, _ time.Time) (bool, error) {
	atomic.AddInt32(&s.highRiskCalls, 1)
	return false, nil
}

func (s *fakeRiskStore) ListRiskRecords(_ context.Context, _ int64) ([]entity.RiskRecord, error) {
	atomic.AddInt32(&s.listCalls, 1)
	return nil, nil
}

func (s *fakeRiskStore) CleanupExpiredRiskUsers(_ context.Context) (int, error) {
	atomic.AddInt32(&s.cleanupCalls, 1)
	return 0, nil
}

func TestRiskCache_IsRiskUser_Miss(t *testing.T) {
	store := newFakeRiskStore()
	c := newTestRiskCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	got, err := c.IsRiskUser(ctx, 1001)
	if err != nil {
		t.Fatalf("IsRiskUser: %v", err)
	}
	if got {
		t.Fatalf("expected false, got true")
	}
	if calls := atomic.LoadInt32(&store.isRiskCalls); calls != 1 {
		t.Fatalf("source calls = %d, want 1", calls)
	}
}

func TestRiskCache_IsRiskUser_Hit(t *testing.T) {
	store := newFakeRiskStore()
	store.riskUsers[1001] = true
	c := newTestRiskCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	first, err := c.IsRiskUser(ctx, 1001)
	if err != nil {
		t.Fatalf("first IsRiskUser: %v", err)
	}
	if !first {
		t.Fatalf("expected true, got false")
	}
	if calls := atomic.LoadInt32(&store.isRiskCalls); calls != 1 {
		t.Fatalf("after first call, source calls = %d, want 1", calls)
	}

	second, err := c.IsRiskUser(ctx, 1001)
	if err != nil {
		t.Fatalf("second IsRiskUser: %v", err)
	}
	if !second {
		t.Fatalf("expected cached true, got false")
	}
	if calls := atomic.LoadInt32(&store.isRiskCalls); calls != 1 {
		t.Fatalf("after second call, source calls = %d, want 1 (should hit cache)", calls)
	}
}

func TestRiskCache_IsRiskUser_SourceError(t *testing.T) {
	store := newFakeRiskStore()
	srcErr := errors.New("db down")
	store.isRiskErr = srcErr
	c := newTestRiskCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	if _, err := c.IsRiskUser(ctx, 1001); !errors.Is(err, srcErr) {
		t.Fatalf("expected source error, got %v", err)
	}
	if calls := atomic.LoadInt32(&store.isRiskCalls); calls != 1 {
		t.Fatalf("source calls = %d, want 1", calls)
	}

	store.isRiskErr = nil
	store.riskUsers[1001] = true
	got, err := c.IsRiskUser(ctx, 1001)
	if err != nil {
		t.Fatalf("after recovery, IsRiskUser: %v", err)
	}
	if !got {
		t.Fatalf("expected true after recovery")
	}
	if calls := atomic.LoadInt32(&store.isRiskCalls); calls != 2 {
		t.Fatalf("source calls = %d, want 2 (error must not be cached)", calls)
	}
}

func TestRiskCache_MarkRiskUser_Invalidates(t *testing.T) {
	store := newFakeRiskStore()
	c := newTestRiskCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	got, err := c.IsRiskUser(ctx, 1001)
	if err != nil {
		t.Fatalf("prime IsRiskUser: %v", err)
	}
	if got {
		t.Fatalf("expected false before marking")
	}
	if calls := atomic.LoadInt32(&store.isRiskCalls); calls != 1 {
		t.Fatalf("primer calls = %d, want 1", calls)
	}

	if err := c.MarkRiskUser(ctx, 1001, time.Hour); err != nil {
		t.Fatalf("MarkRiskUser: %v", err)
	}

	got, err = c.IsRiskUser(ctx, 1001)
	if err != nil {
		t.Fatalf("IsRiskUser after MarkRiskUser: %v", err)
	}
	if !got {
		t.Fatalf("expected true after MarkRiskUser")
	}
	if calls := atomic.LoadInt32(&store.isRiskCalls); calls != 2 {
		t.Fatalf("after MarkRiskUser-invalidate, source calls = %d, want 2 (cache must be evicted)", calls)
	}
}

func TestRiskCache_RecordRiskAction_PassThrough(t *testing.T) {
	store := newFakeRiskStore()
	c := newTestRiskCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	if _, err := c.IsRiskUser(ctx, 1001); err != nil {
		t.Fatalf("prime IsRiskUser: %v", err)
	}
	if calls := atomic.LoadInt32(&store.isRiskCalls); calls != 1 {
		t.Fatalf("primer calls = %d, want 1", calls)
	}

	rec := entity.RiskRecord{UserID: 1001, ActionType: entity.RiskActionSeckill, RiskLevel: entity.RiskLevelHigh}
	if err := c.RecordRiskAction(ctx, rec); err != nil {
		t.Fatalf("RecordRiskAction: %v", err)
	}
	if calls := atomic.LoadInt32(&store.recordCalls); calls != 1 {
		t.Fatalf("RecordRiskAction source calls = %d, want 1", calls)
	}

	if _, err := c.IsRiskUser(ctx, 1001); err != nil {
		t.Fatalf("IsRiskUser after RecordRiskAction: %v", err)
	}
	if calls := atomic.LoadInt32(&store.isRiskCalls); calls != 1 {
		t.Fatalf("after RecordRiskAction, source calls = %d, want 1 (cache must NOT be evicted)", calls)
	}
}

func TestRiskCache_PassThroughMethods(t *testing.T) {
	store := newFakeRiskStore()
	c := newTestRiskCache(t, store)
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	now := time.Now()

	if _, err := c.CountRecentRiskActions(ctx, 1001, entity.RiskActionSeckill, now); err != nil {
		t.Fatalf("CountRecentRiskActions: %v", err)
	}
	if got := atomic.LoadInt32(&store.countCalls); got != 1 {
		t.Fatalf("CountRecentRiskActions source calls = %d, want 1", got)
	}

	if _, err := c.HasHighRiskRecord(ctx, 1001, now); err != nil {
		t.Fatalf("HasHighRiskRecord: %v", err)
	}
	if got := atomic.LoadInt32(&store.highRiskCalls); got != 1 {
		t.Fatalf("HasHighRiskRecord source calls = %d, want 1", got)
	}

	if _, err := c.ListRiskRecords(ctx, 1001); err != nil {
		t.Fatalf("ListRiskRecords: %v", err)
	}
	if got := atomic.LoadInt32(&store.listCalls); got != 1 {
		t.Fatalf("ListRiskRecords source calls = %d, want 1", got)
	}

	if _, err := c.CleanupExpiredRiskUsers(ctx); err != nil {
		t.Fatalf("CleanupExpiredRiskUsers: %v", err)
	}
	if got := atomic.LoadInt32(&store.cleanupCalls); got != 1 {
		t.Fatalf("CleanupExpiredRiskUsers source calls = %d, want 1", got)
	}
}

func TestRiskCache_Concurrent(t *testing.T) {
	store := newFakeRiskStore()
	store.riskUsers[1001] = true
	c := newTestRiskCache(t, store)
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
			got, err := c.IsRiskUser(context.Background(), 1001)
			if err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", idx, err)
				return
			}
			if !got {
				errs <- fmt.Errorf("goroutine %d: expected true", idx)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}

	calls := atomic.LoadInt32(&store.isRiskCalls)
	if calls < 1 || calls > n {
		t.Fatalf("concurrent source calls = %d, want in [1, %d]", calls, n)
	}
	t.Logf("concurrent IsRiskUser: %d source calls across %d goroutines", calls, n)
}

func TestNewRiskCache_NilSourceRejected(t *testing.T) {
	if _, err := NewRiskCache(nil, DefaultConfig(), nil); err == nil {
		t.Fatal("expected error for nil source")
	}
}
