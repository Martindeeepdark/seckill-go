package server

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestShardedRateLimitStoreImplementsInterface 验证 ShardedRateLimitStore 实现 RateLimitStore 接口
func TestShardedRateLimitStoreImplementsInterface(t *testing.T) {
	var _ RateLimitStore = NewShardedRateLimitStore(nil)
}

// TestShardedRateLimitStoreBasicAllow 基本允许/拒绝测试
func TestShardedRateLimitStoreBasicAllow(t *testing.T) {
	now := time.Unix(100, 0)
	store := NewShardedRateLimitStore(func() time.Time { return now })
	ctx := context.Background()

	// 消耗所有令牌（limit=3）
	for i := 0; i < 3; i++ {
		allowed, err := store.Allow(ctx, "test-key", 3, time.Second)
		if err != nil {
			t.Fatalf("Allow(%d) returned error: %v", i, err)
		}
		if !allowed {
			t.Fatalf("Allow(%d) = false, want true", i)
		}
	}

	// 第 4 次应该被拒绝
	allowed, err := store.Allow(ctx, "test-key", 3, time.Second)
	if err != nil {
		t.Fatalf("Allow(overflow) returned error: %v", err)
	}
	if allowed {
		t.Fatal("Allow(overflow) = true, want false")
	}

	// 推进时间，令牌恢复
	now = now.Add(time.Second)
	allowed, err = store.Allow(ctx, "test-key", 3, time.Second)
	if err != nil {
		t.Fatalf("Allow(after refill) returned error: %v", err)
	}
	if !allowed {
		t.Fatal("Allow(after refill) = false, want true")
	}

	// 不同的 key 应该独立
	allowed, err = store.Allow(ctx, "other-key", 3, time.Second)
	if err != nil {
		t.Fatalf("Allow(other-key) returned error: %v", err)
	}
	if !allowed {
		t.Fatal("Allow(other-key) = false, want true")
	}
}

// TestShardedRateLimitStoreConcurrentAccess 并发安全性测试
func TestShardedRateLimitStoreConcurrentAccess(t *testing.T) {
	store := NewShardedRateLimitStore(time.Now)
	ctx := context.Background()

	const goroutines = 100
	const requestsPerGoroutine = 50
	var allowedCount atomic.Int64
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			key := "concurrent-key"
			for j := 0; j < requestsPerGoroutine; j++ {
				allowed, err := store.Allow(ctx, key, goroutines*requestsPerGoroutine, time.Minute)
				if err != nil {
					t.Errorf("goroutine %d request %d: unexpected error: %v", id, j, err)
					return
				}
				if allowed {
					allowedCount.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()

	// 所有请求的令牌桶 limit 足够大，应该全部允许
	totalAllowed := allowedCount.Load()
	if totalAllowed != goroutines*requestsPerGoroutine {
		t.Fatalf("total allowed = %d, want %d", totalAllowed, goroutines*requestsPerGoroutine)
	}
}

// TestShardedRateLimitStoreConcurrentDifferentKeys 多 key 并发测试
func TestShardedRateLimitStoreConcurrentDifferentKeys(t *testing.T) {
	store := NewShardedRateLimitStore(time.Now)
	ctx := context.Background()

	const goroutines = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			key := "key-" + strconv.Itoa(id)
			for j := 0; j < 10; j++ {
				_, err := store.Allow(ctx, key, 10, time.Second)
				if err != nil {
					t.Errorf("goroutine %d: unexpected error: %v", id, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

// TestShardedRateLimitStoreShardDistribution 验证 key 分布到不同 shard
func TestShardedRateLimitStoreShardDistribution(t *testing.T) {
	store := NewShardedRateLimitStore(time.Now)

	// 通过反射或内部方法验证分片分布
	// 使用 getShard 检查不同的 key 映射到不同的 shard
	seenShards := make(map[int]bool)
	for i := 0; i < 256; i++ {
		key := "key-" + strconv.Itoa(i)
		shard := store.getShard(key)
		// 计算 shard 的索引
		for idx := range store.shards {
			if &store.shards[idx] == shard {
				seenShards[idx] = true
				break
			}
		}
	}

	// 256 个 key 应该分布到多个 shard，不能全在一个 shard 里
	if len(seenShards) < 8 {
		t.Fatalf("keys distributed to only %d shards, want at least 8", len(seenShards))
	}
}

// BenchmarkShardedVsLocalRateLimit 对比基准测试
func BenchmarkShardedVsLocalRateLimit(b *testing.B) {
	ctx := context.Background()
	now := time.Now

	b.Run("Local_100", func(b *testing.B) {
		benchmarkLocalRateLimit(ctx, now, b, 100)
	})
	b.Run("Sharded_100", func(b *testing.B) {
		benchmarkShardedRateLimit(ctx, now, b, 100)
	})
	b.Run("Local_500", func(b *testing.B) {
		benchmarkLocalRateLimit(ctx, now, b, 500)
	})
	b.Run("Sharded_500", func(b *testing.B) {
		benchmarkShardedRateLimit(ctx, now, b, 500)
	})
	b.Run("Local_1000", func(b *testing.B) {
		benchmarkLocalRateLimit(ctx, now, b, 1000)
	})
	b.Run("Sharded_1000", func(b *testing.B) {
		benchmarkShardedRateLimit(ctx, now, b, 1000)
	})
}

func benchmarkLocalRateLimit(ctx context.Context, now func() time.Time, b *testing.B, concurrency int) {
	store := NewLocalRateLimitStore(now)
	b.ResetTimer()
	var wg sync.WaitGroup
	wg.Add(concurrency)
	iters := b.N / concurrency
	if iters == 0 {
		iters = 1
	}
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			key := "bench-key-" + strconv.Itoa(id)
			for j := 0; j < iters; j++ {
				_, _ = store.Allow(ctx, key, iters+1, time.Minute)
			}
		}(i)
	}
	wg.Wait()
}

func benchmarkShardedRateLimit(ctx context.Context, now func() time.Time, b *testing.B, concurrency int) {
	store := NewShardedRateLimitStore(now)
	b.ResetTimer()
	var wg sync.WaitGroup
	wg.Add(concurrency)
	iters := b.N / concurrency
	if iters == 0 {
		iters = 1
	}
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			key := "bench-key-" + strconv.Itoa(id)
			for j := 0; j < iters; j++ {
				_, _ = store.Allow(ctx, key, iters+1, time.Minute)
			}
		}(i)
	}
	wg.Wait()
}
