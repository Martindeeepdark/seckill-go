package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"seckill-processor-service/internal/domain/model"
)

type fakeActivitySource struct {
	activityHits atomic.Int64
	skuHits      atomic.Int64
	activity     model.ActivityInfo
	sku          model.SKUInfo
	activityErr  error
	skuErr       error
	mu           sync.Mutex
	delay        bool
}

func (f *fakeActivitySource) GetActivity(ctx context.Context, activityNo string) (model.ActivityInfo, error) {
	f.activityHits.Add(1)
	if f.delay {
		select {
		case <-ctx.Done():
			return model.ActivityInfo{}, ctx.Err()
		default:
		}
	}
	if f.activityErr != nil {
		return model.ActivityInfo{}, f.activityErr
	}
	return f.activity, nil
}

func (f *fakeActivitySource) GetSKU(ctx context.Context, activityNo, skuNo string) (model.SKUInfo, error) {
	f.skuHits.Add(1)
	if f.skuErr != nil {
		return model.SKUInfo{}, f.skuErr
	}
	return f.sku, nil
}

func newCache(t *testing.T, source *fakeActivitySource) *ActivityCache {
	t.Helper()
	c, err := NewActivityCache(source, nil, DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewActivityCache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestActivityCache_GetActivity_MissThenHit(t *testing.T) {
	source := &fakeActivitySource{
		activity: model.ActivityInfo{ActivityNo: "A1", Name: "flash", PurchaseLimit: 2},
	}
	c := newCache(t, source)

	got, err := c.GetActivity(context.Background(), "A1")
	if err != nil {
		t.Fatalf("first GetActivity: %v", err)
	}
	if got.ActivityNo != "A1" || got.Name != "flash" || got.PurchaseLimit != 2 {
		t.Fatalf("unexpected activity: %+v", got)
	}
	if source.activityHits.Load() != 1 {
		t.Fatalf("expected 1 source call, got %d", source.activityHits.Load())
	}

	got2, err := c.GetActivity(context.Background(), "A1")
	if err != nil {
		t.Fatalf("second GetActivity: %v", err)
	}
	if got2.ActivityNo != "A1" {
		t.Fatalf("cached activity mismatch: %+v", got2)
	}
	if source.activityHits.Load() != 1 {
		t.Fatalf("expected cache hit (1 source call), got %d", source.activityHits.Load())
	}
}

func TestActivityCache_GetActivity_SourceError(t *testing.T) {
	srcErr := errors.New("rpc unavailable")
	source := &fakeActivitySource{activityErr: srcErr}
	c := newCache(t, source)

	if _, err := c.GetActivity(context.Background(), "A1"); !errors.Is(err, srcErr) {
		t.Fatalf("expected source error, got %v", err)
	}
	if source.activityHits.Load() != 1 {
		t.Fatalf("expected 1 source call on error path, got %d", source.activityHits.Load())
	}
}

func TestActivityCache_GetSKU_MissThenHit(t *testing.T) {
	source := &fakeActivitySource{
		sku: model.SKUInfo{ActivityNo: "A1", SKUNo: "S1", TotalStock: 100, SeckillPrice: 999},
	}
	c := newCache(t, source)

	got, err := c.GetSKU(context.Background(), "A1", "S1")
	if err != nil {
		t.Fatalf("first GetSKU: %v", err)
	}
	if got.SKUNo != "S1" || got.TotalStock != 100 || got.SeckillPrice != 999 {
		t.Fatalf("unexpected sku: %+v", got)
	}
	if source.skuHits.Load() != 1 {
		t.Fatalf("expected 1 source call, got %d", source.skuHits.Load())
	}

	if _, err := c.GetSKU(context.Background(), "A1", "S1"); err != nil {
		t.Fatalf("second GetSKU: %v", err)
	}
	if source.skuHits.Load() != 1 {
		t.Fatalf("expected cache hit (1 source call), got %d", source.skuHits.Load())
	}
}

func TestActivityCache_GetSKU_SourceError(t *testing.T) {
	srcErr := errors.New("sku rpc unavailable")
	source := &fakeActivitySource{skuErr: srcErr}
	c := newCache(t, source)

	if _, err := c.GetSKU(context.Background(), "A1", "S1"); !errors.Is(err, srcErr) {
		t.Fatalf("expected source error, got %v", err)
	}
	if source.skuHits.Load() != 1 {
		t.Fatalf("expected 1 source call on error path, got %d", source.skuHits.Load())
	}
}

func TestActivityCache_ConcurrentSameKey(t *testing.T) {
	source := &fakeActivitySource{
		activity: model.ActivityInfo{ActivityNo: "A1", Name: "flash"},
	}
	c := newCache(t, source)

	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make([]error, n)
	results := make([]model.ActivityInfo, n)
	start := make(chan struct{})

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			<-start
			info, err := c.GetActivity(context.Background(), "A1")
			results[idx] = info
			errs[idx] = err
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d error: %v", i, err)
		}
		if results[i].ActivityNo != "A1" {
			t.Fatalf("goroutine %d bad result: %+v", i, results[i])
		}
	}

	calls := source.activityHits.Load()
	if calls == 0 {
		t.Fatalf("source was never called")
	}
	if calls > int64(n) {
		t.Fatalf("source calls %d exceeded goroutines %d", calls, n)
	}
	t.Logf("concurrent access: %d goroutines, %d source calls", n, calls)
}

func TestActivityCache_NilSourceRejected(t *testing.T) {
	if _, err := NewActivityCache(nil, nil, DefaultConfig(), nil); err == nil {
		t.Fatal("expected error for nil source")
	}
}
