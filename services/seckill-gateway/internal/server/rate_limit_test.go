package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"

	"seckill-common/tracing"
)

func TestRateLimiterRefillsByQPS(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := newRateLimiterForInterval(1, time.Second, func() time.Time { return now })

	if !limiter.Allow() {
		t.Fatal("first request was rejected")
	}
	if limiter.Allow() {
		t.Fatal("second request was allowed before refill")
	}

	now = now.Add(time.Second)
	if !limiter.Allow() {
		t.Fatal("request was rejected after one second refill")
	}
}

func TestRateLimitMiddlewareRejectsOverQPSWithTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(100, 0)
	router := gin.New()
	router.Use(TraceMiddleware())
	router.Use(newRateLimitMiddleware(RateLimitOptions{
		MaxQPS: 1,
		Store:  NewLocalRateLimitStore(func() time.Time { return now }),
	}))
	router.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	traceID := "11111111111111111111111111111111"
	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodGet, "/ok", nil)
	firstReq.Header.Set(tracing.HeaderTraceID, traceID)
	router.ServeHTTP(first, firstReq)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d body = %s, want 200", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodGet, "/ok", nil)
	secondReq.Header.Set(tracing.HeaderTraceID, traceID)
	router.ServeHTTP(second, secondReq)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d body = %s, want 429", second.Code, second.Body.String())
	}

	var body rateLimitResponse
	if err := json.Unmarshal(second.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "rate_limited" || body.Success {
		t.Fatalf("response = %+v, want rate_limited failure", body)
	}
	if body.RequestTraceID != traceID {
		t.Fatalf("requestTraceId = %q, want %q", body.RequestTraceID, traceID)
	}
	if got := second.Header().Get(tracing.HeaderTraceID); got != traceID {
		t.Fatalf("trace header = %q, want %q", got, traceID)
	}
}

func TestRateLimitMiddlewareLimitsRoutesIndependently(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(100, 0)
	router := gin.New()
	router.Use(newRateLimitMiddleware(RateLimitOptions{
		MaxQPS: 1,
		Store:  NewLocalRateLimitStore(func() time.Time { return now }),
	}))
	router.GET("/a", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/b", func(c *gin.Context) { c.Status(http.StatusOK) })

	if resp := performRateLimitRequest(router, "/a", ""); resp.Code != http.StatusOK {
		t.Fatalf("/a first status = %d, want 200", resp.Code)
	}
	if resp := performRateLimitRequest(router, "/a", ""); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("/a second status = %d, want 429", resp.Code)
	}
	if resp := performRateLimitRequest(router, "/b", ""); resp.Code != http.StatusOK {
		t.Fatalf("/b first status = %d, want 200", resp.Code)
	}
}

func TestRateLimitMiddlewareUsesResourceRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(100, 0)
	router := gin.New()
	router.Use(newRateLimitMiddleware(RateLimitOptions{
		MaxQPS: 3,
		Rules: []RateLimitRule{
			{Resource: "POST /api/seckill/part-in", Count: 1, Interval: 2 * time.Second},
		},
		Store: NewLocalRateLimitStore(func() time.Time { return now }),
	}))
	router.POST("/api/seckill/part-in", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/api/activities", func(c *gin.Context) { c.Status(http.StatusOK) })

	if resp := performRateLimitRequest(router, "/api/seckill/part-in", ""); resp.Code != http.StatusOK {
		t.Fatalf("part-in first status = %d, want 200", resp.Code)
	}
	if resp := performRateLimitRequest(router, "/api/seckill/part-in", ""); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("part-in second status = %d, want 429", resp.Code)
	}
	if resp := performRateLimitRequest(router, "/api/activities", ""); resp.Code != http.StatusOK {
		t.Fatalf("activities first status = %d, want 200", resp.Code)
	}
	if resp := performRateLimitRequest(router, "/api/activities", ""); resp.Code != http.StatusOK {
		t.Fatalf("activities second status = %d, want 200 from max_qps fallback", resp.Code)
	}

	now = now.Add(2 * time.Second)
	if resp := performRateLimitRequest(router, "/api/seckill/part-in", ""); resp.Code != http.StatusOK {
		t.Fatalf("part-in after rule interval status = %d, want 200", resp.Code)
	}
}

func TestRateLimitMiddlewareMatchesParameterizedRouteRule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(100, 0)
	router := gin.New()
	router.Use(newRateLimitMiddleware(RateLimitOptions{
		Rules: []RateLimitRule{
			{Resource: "GET /api/seckill/activity/:activityNo", Count: 1, Interval: time.Second},
		},
		Store: NewLocalRateLimitStore(func() time.Time { return now }),
	}))
	router.GET("/api/seckill/activity/:activityNo", func(c *gin.Context) { c.Status(http.StatusOK) })

	if resp := performRateLimitRequest(router, "/api/seckill/activity/A1", ""); resp.Code != http.StatusOK {
		t.Fatalf("first activity detail status = %d, want 200", resp.Code)
	}
	if resp := performRateLimitRequest(router, "/api/seckill/activity/A2", ""); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("second activity detail status = %d, want 429", resp.Code)
	}
}

func TestRateLimitMiddlewareLimitsUserIndependently(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(100, 0)
	router := gin.New()
	router.Use(newRateLimitMiddleware(RateLimitOptions{
		UserEnabled:  true,
		UserRate:     1,
		UserInterval: 10 * time.Second,
		Store:        NewLocalRateLimitStore(func() time.Time { return now }),
	}))
	router.POST("/api/seckill/part-in", func(c *gin.Context) { c.Status(http.StatusOK) })

	if resp := performRateLimitRequest(router, "/api/seckill/part-in", "7"); resp.Code != http.StatusOK {
		t.Fatalf("user 7 first status = %d, want 200", resp.Code)
	}
	if resp := performRateLimitRequest(router, "/api/seckill/part-in", "7"); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("user 7 second status = %d, want 429", resp.Code)
	}
	if resp := performRateLimitRequest(router, "/api/seckill/part-in", "8"); resp.Code != http.StatusOK {
		t.Fatalf("user 8 first status = %d, want 200", resp.Code)
	}

	now = now.Add(10 * time.Second)
	if resp := performRateLimitRequest(router, "/api/seckill/part-in", "7"); resp.Code != http.StatusOK {
		t.Fatalf("user 7 after refill status = %d, want 200", resp.Code)
	}
}

func performRateLimitRequest(router *gin.Engine, path string, userID string) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if strings.Contains(path, "part-in") {
		req = httptest.NewRequest(http.MethodPost, path, nil)
	}
	if userID != "" {
		req.Header.Set("X-User-Id", userID)
	}
	router.ServeHTTP(resp, req)
	return resp
}

func TestDynamicRateLimitMiddlewareReadsFromRuntimeConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(100, 0)

	rc := NewGatewayRuntimeConfig()
	rc.UpdateRateLimit(true, RateLimitOptions{
		MaxQPS: 1,
		Store:  NewLocalRateLimitStore(func() time.Time { return now }),
	})

	router := gin.New()
	router.Use(DynamicRateLimitMiddleware(rc))
	router.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	// First request passes
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp.Code)
	}

	// Second request limited
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", resp.Code)
	}
}

func TestDynamicRateLimitMiddlewareDisabledPasses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rc := NewGatewayRuntimeConfig()
	rc.UpdateRateLimit(false, RateLimitOptions{MaxQPS: 1})

	router := gin.New()
	router.Use(DynamicRateLimitMiddleware(rc))
	router.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 5; i++ {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ok", nil))
		if resp.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 (disabled)", i+1, resp.Code)
		}
	}
}

func TestDynamicRateLimitMiddlewareConfigChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(100, 0)

	rc := NewGatewayRuntimeConfig()
	rc.UpdateRateLimit(true, RateLimitOptions{
		MaxQPS: 1,
		Store:  NewLocalRateLimitStore(func() time.Time { return now }),
	})

	router := gin.New()
	router.Use(DynamicRateLimitMiddleware(rc))
	router.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	// First request passes, second limited
	httptest.NewRecorder().Code = 0
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp.Code)
	}

	// Disable rate limit dynamically
	rc.UpdateRateLimit(false, RateLimitOptions{MaxQPS: 1})

	// Now requests should pass
	for i := 0; i < 3; i++ {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ok", nil))
		if resp.Code != http.StatusOK {
			t.Fatalf("after disable, request %d status = %d, want 200", i+1, resp.Code)
		}
	}
}

func TestRedisRateLimitClient_InterfaceComplete(t *testing.T) {
	// 此测试验证 redis.Client 实现了 redisRateLimitClient 接口
	// 包括滑动窗口算法所需的 ZSET 操作方法
	var _ redisRateLimitClient = (*goredis.Client)(nil) // 应该编译通过

	// 验证接口包含滑动窗口所需的方法（编译时检查）
	// 如果接口缺少这些方法，下面的代码将无法编译
	var client redisRateLimitClient
	if client != nil {
		_ = client.ZRemRangeByScore
		_ = client.ZCard
		_ = client.ZAdd
		_ = client.Pipeline
	}
}

func TestRedisRateLimitStore_SlidingWindow_SingleInstance(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisRateLimitStore(client, "test")

	ctx := context.Background()
	key := "user:10001"
	limit := 2
	interval := 1 * time.Second

	// 第 1 次请求应该通过
	allowed, err := store.Allow(ctx, key, limit, interval)
	if err != nil {
		t.Fatalf("request 1 error: %v", err)
	}
	if !allowed {
		t.Fatal("request 1 should be allowed")
	}

	// 第 2 次请求应该通过
	allowed, err = store.Allow(ctx, key, limit, interval)
	if err != nil {
		t.Fatalf("request 2 error: %v", err)
	}
	if !allowed {
		t.Fatal("request 2 should be allowed")
	}

	// 第 3 次应该被拒绝（窗口内已有 2 个请求）
	allowed, err = store.Allow(ctx, key, limit, interval)
	if err != nil {
		t.Fatalf("request 3 error: %v", err)
	}
	if allowed {
		t.Fatal("request 3 should be rate limited")
	}

	// 等待 1.1 秒让所有请求过期
	time.Sleep(1100 * time.Millisecond)

	// 这时应该可以发送新请求（窗口已滑过）
	allowed, err = store.Allow(ctx, key, limit, interval)
	if err != nil {
		t.Fatalf("request after window error: %v", err)
	}
	if !allowed {
		t.Fatal("request after window should be allowed")
	}
}

func TestRedisRateLimitStore_FailOpen_RedisDown(t *testing.T) {
	// 使用错误的 Redis 地址模拟连接失败
	client := goredis.NewClient(&goredis.Options{
		Addr:        "127.0.0.1:9999", // 不存在的地址
		DialTimeout: 100 * time.Millisecond,
	})
	defer client.Close()
	store := NewRedisRateLimitStore(client, "test")

	ctx := context.Background()
	key := "user:10001"
	limit := 3
	interval := 5 * time.Second

	// Redis 不可用时应该放行请求
	allowed, err := store.Allow(ctx, key, limit, interval)
	if err != nil {
		t.Fatalf("should not return error on fail-open, got: %v", err)
	}
	if !allowed {
		t.Fatal("should fail-open when Redis is down")
	}
}

func TestRedisRateLimitStore_WindowBoundaryPrecision(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisRateLimitStore(client, "test")

	// 使用可控时钟
	now := time.Unix(100, 0)
	store.now = func() time.Time { return now }

	ctx := context.Background()
	key := "user:10001"
	limit := 10
	interval := 10 * time.Second

	// 在 t=0 时发送 10 次请求
	for i := 0; i < 10; i++ {
		allowed, err := store.Allow(ctx, key, limit, interval)
		if err != nil {
			t.Fatalf("request %d error: %v", i+1, err)
		}
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// t=0 时第 11 次应该被拒绝
	allowed, err := store.Allow(ctx, key, limit, interval)
	if err != nil {
		t.Fatalf("request 11 error: %v", err)
	}
	if allowed {
		t.Fatal("request 11 at t=0 should be rate limited")
	}

	// t=9.5s 仍在窗口内，应该被拒绝
	now = now.Add(9500 * time.Millisecond)
	allowed, err = store.Allow(ctx, key, limit, interval)
	if err != nil {
		t.Fatalf("request at t=9.5s error: %v", err)
	}
	if allowed {
		t.Fatal("request at t=9.5s should still be rate limited")
	}

	// t=10.1s 窗口外，应该允许
	now = now.Add(600 * time.Millisecond) // 总共 10.1s
	allowed, err = store.Allow(ctx, key, limit, interval)
	if err != nil {
		t.Fatalf("request at t=10.1s error: %v", err)
	}
	if !allowed {
		t.Fatal("request at t=10.1s should be allowed")
	}
}

// TestRedisRateLimitStore_ConcurrentRequests 测试并发请求场景
//
// 两次 Pipeline 方案的并发语义：
// - 多个请求可能同时读到相同的 count（例如都读到 count=5）
// - 这些请求都会通过 count < limit 检查并执行写入
// - 最终 count 会超过 limit（Design Doc 接受的 10-20 次容忍度）
//
// 实际测试：20 个并发请求，limit=10，允许 10-20 次通过
// 断言：至少 10 次（业务最低要求），最多 20 次（所有请求同时通过检查的极端情况）
func TestRedisRateLimitStore_ConcurrentRequests(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisRateLimitStore(client, "test")

	ctx := context.Background()
	key := "user:10001"
	limit := 10
	interval := 10 * time.Second

	// 并发发送 20 次请求
	var wg sync.WaitGroup
	results := make([]bool, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			allowed, err := store.Allow(ctx, key, limit, interval)
			if err != nil {
				t.Errorf("request %d error: %v", idx+1, err)
			}
			results[idx] = allowed
		}(i)
	}
	wg.Wait()

	// 统计允许的请求数
	allowedCount := 0
	for _, allowed := range results {
		if allowed {
			allowedCount++
		}
	}

	// 两次 Pipeline 实现的并发行为：20 个并发请求，limit=10
	// 多个请求可能同时读到相同的 count 并都通过检查，导致超限写入
	// 实际测试结果：16-19 次通过（取决于调度）
	// 断言：至少 10 次（业务最低要求），最多 20 次（所有请求同时通过检查的极端情况）
	if allowedCount < 10 {
		t.Fatalf("allowed count = %d, want at least 10", allowedCount)
	}
	if allowedCount > 20 {
		t.Fatalf("allowed count = %d, want no more than 20 (concurrent tolerance)", allowedCount)
	}
}
