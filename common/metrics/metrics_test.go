package metrics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestClient(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return client, func() { client.Close(); mr.Close() }
}

func TestConstants(t *testing.T) {
	if FieldRateLimit != "rate-limit" {
		t.Errorf("FieldRateLimit = %q, want %q", FieldRateLimit, "rate-limit")
	}
	if FieldRisk != "risk" {
		t.Errorf("FieldRisk = %q, want %q", FieldRisk, "risk")
	}
	if FieldStockEmpty != "stock-empty" {
		t.Errorf("FieldStockEmpty = %q, want %q", FieldStockEmpty, "stock-empty")
	}
	if FieldSuccess != "success" {
		t.Errorf("FieldSuccess = %q, want %q", FieldSuccess, "success")
	}
	if FieldOther != "other" {
		t.Errorf("FieldOther = %q, want %q", FieldOther, "other")
	}
}

func TestIncrRateLimit(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()
	SetClient(client)

	ctx := context.Background()
	runID := "smoke-20260615-120000-12345"

	IncrRateLimit(ctx, runID)

	key := keyPrefix + runID
	val, err := client.HGet(ctx, key, FieldRateLimit).Result()
	if err != nil {
		t.Fatalf("HGet %s %s: %v", key, FieldRateLimit, err)
	}
	if val != "1" {
		t.Errorf("rate-limit = %q, want 1", val)
	}

	// Check TTL is set
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL %s: %v", key, err)
	}
	if ttl <= 0 || ttl > time.Hour {
		t.Errorf("TTL = %v, want positive duration <= 1h", ttl)
	}
}

func TestAllFieldFunctions(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()
	SetClient(client)

	ctx := context.Background()
	runID := "smoke-all-fields"

	IncrRateLimit(ctx, runID)
	IncrRisk(ctx, runID)
	IncrRisk(ctx, runID) // twice
	IncrStockEmpty(ctx, runID)
	IncrSuccess(ctx, runID)
	IncrSuccess(ctx, runID)
	IncrSuccess(ctx, runID) // three times
	IncrOther(ctx, runID)

	key := keyPrefix + runID
	all, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		t.Fatalf("HGetAll %s: %v", key, err)
	}

	tests := []struct {
		field string
		want  string
	}{
		{FieldRateLimit, "1"},
		{FieldRisk, "2"},
		{FieldStockEmpty, "1"},
		{FieldSuccess, "3"},
		{FieldOther, "1"},
	}
	for _, tt := range tests {
		got := all[tt.field]
		if got != tt.want {
			t.Errorf("%s = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestEmptyRunID_NoOp(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()
	SetClient(client)

	ctx := context.Background()

	// None of these should panic or write anything
	IncrRateLimit(ctx, "")
	IncrRisk(ctx, "")
	IncrStockEmpty(ctx, "")
	IncrSuccess(ctx, "")
	IncrOther(ctx, "")

	// Verify no key was created for empty runID
	keys, err := client.Keys(ctx, keyPrefix+"*").Result()
	if err != nil {
		t.Fatalf("KEYS: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys for empty runID, got %d: %v", len(keys), keys)
	}
}

func TestNilClient_NoOp(t *testing.T) {
	// Reset to nil client (no-op, no panic)
	SetClient(nil)

	ctx := context.Background()
	// These should not panic
	IncrRateLimit(ctx, "run-123")
	IncrRisk(ctx, "run-123")
	IncrStockEmpty(ctx, "run-123")
	IncrSuccess(ctx, "run-123")
	IncrOther(ctx, "run-123")
}

func TestConcurrentIncrement(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()
	SetClient(client)

	ctx := context.Background()
	runID := "smoke-concurrent"

	const goroutines = 100
	const callsPerGoroutine = 100
	const expected = goroutines * callsPerGoroutine

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				IncrRateLimit(ctx, runID)
			}
		}()
	}
	wg.Wait()

	key := keyPrefix + runID
	val, err := client.HGet(ctx, key, FieldRateLimit).Result()
	if err != nil {
		t.Fatalf("HGet %s %s: %v", key, FieldRateLimit, err)
	}
	if val != "10000" {
		t.Errorf("concurrent total = %q, want 10000", val)
	}

	// Verify other fields are not set
	riskVal, _ := client.HGet(ctx, key, FieldRisk).Result()
	if riskVal != "" {
		t.Errorf("FieldRisk should be empty, got %q", riskVal)
	}
}

func TestMultipleFieldsConcurrent(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()
	SetClient(client)

	ctx := context.Background()
	runID := "smoke-multi-concurrent"

	const goroutines = 50
	const callsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				switch (idx + j) % 5 {
				case 0:
					IncrRateLimit(ctx, runID)
				case 1:
					IncrRisk(ctx, runID)
				case 2:
					IncrStockEmpty(ctx, runID)
				case 3:
					IncrSuccess(ctx, runID)
				case 4:
					IncrOther(ctx, runID)
				}
			}
		}(i)
	}
	wg.Wait()

	key := keyPrefix + runID
	all, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		t.Fatalf("HGetAll %s: %v", key, err)
	}

	// Sum all values — should equal total calls
	total := 0
	for _, v := range all {
		n := 0
		for _, c := range v {
			n = n*10 + int(c-'0')
		}
		total += n
	}

	expected := goroutines * callsPerGoroutine
	if total != expected {
		t.Errorf("total increments = %d, want %d", total, expected)
	}

	// Each field should be approximately expected/5 (±20% for concurrency distribution)
	avgPerField := expected / 5
	for field, v := range all {
		n := 0
		for _, c := range v {
			n = n*10 + int(c-'0')
		}
		if n < avgPerField*8/10 || n > avgPerField*12/10 {
			t.Errorf("field %s = %d, expected ~%d (out of range)", field, n, avgPerField)
		}
	}
}

func TestPipelineAtomic(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()
	SetClient(client)

	ctx := context.Background()
	runID := "smoke-pipeline-test"

	IncrRateLimit(ctx, runID)

	key := keyPrefix + runID

	// Verify both HINCRBY and EXPIRE were executed
	val, err := client.HGet(ctx, key, FieldRateLimit).Result()
	if err != nil {
		t.Fatalf("HGet failed: %v", err)
	}
	if val != "1" {
		t.Errorf("value = %q, want 1", val)
	}

	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl <= 0 {
		t.Errorf("TTL = %v, want positive (EXPIRE was set)", ttl)
	}
}
