// Package server 提供 HTTP 服务器和中间件
package server

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"

	"seckill-common/tracing"
)

const defaultUserRateInterval = 10 * time.Second

// rateLimitResponse 限流响应
type rateLimitResponse struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Success        bool   `json:"success"`
	RequestTraceID string `json:"requestTraceId,omitempty"`
}

// RateLimitOptions 限流选项
type RateLimitOptions struct {
	MaxQPS       int
	Rules        []RateLimitRule
	UserEnabled  bool
	UserRate     int
	UserInterval time.Duration
	Store        RateLimitStore
	Now          func() time.Time
}

// RateLimitRule 限流规则
type RateLimitRule struct {
	Resource string
	Count    int
	Interval time.Duration
}

// RateLimitStore 限流存储接口
type RateLimitStore interface {
	Allow(ctx context.Context, key string, limit int, interval time.Duration) (bool, error)
}

// RateLimitMiddleware 使用进程本地令牌桶限制 gateway 入站 QPS（按路由）
func RateLimitMiddleware(maxQPS int) gin.HandlerFunc {
	return RateLimitMiddlewareWithOptions(RateLimitOptions{MaxQPS: maxQPS})
}

// RateLimitMiddlewareWithOptions 使用选项创建限流中间件
func RateLimitMiddlewareWithOptions(options RateLimitOptions) gin.HandlerFunc {
	if options.Store == nil {
		options.Store = NewShardedRateLimitStore(options.Now)
	}
	if options.UserInterval <= 0 {
		options.UserInterval = defaultUserRateInterval
	}
	return newRateLimitMiddleware(options)
}

// DynamicRateLimitMiddleware 从 RuntimeConfig 读取限流配置，每次请求动态读取。
func DynamicRateLimitMiddleware(rc *GatewayRuntimeConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		snap := rc.RateLimit()
		if !snap.Enabled {
			c.Next()
			return
		}
		opts := snap.Options
		if opts.Store == nil {
			opts.Store = NewShardedRateLimitStore(opts.Now)
		}
		if opts.UserInterval <= 0 {
			opts.UserInterval = defaultUserRateInterval
		}
		if limited, err := limitRoute(c, opts); err != nil {
			c.Next()
			return
		} else if limited {
			abortRateLimited(c)
			return
		}
		if limited, err := limitUser(c, opts); err != nil {
			c.Next()
			return
		} else if limited {
			abortRateLimited(c)
			return
		}
		c.Next()
	}
}

// newRateLimitMiddleware 创建限流中间件实例
func newRateLimitMiddleware(options RateLimitOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limited, err := limitRoute(c, options); err != nil {
			c.Next()
			return
		} else if limited {
			abortRateLimited(c)
			return
		}

		if limited, err := limitUser(c, options); err != nil {
			c.Next()
			return
		} else if limited {
			abortRateLimited(c)
			return
		}

		c.Next()
	}
}

// limitRoute 限制路由请求速率
func limitRoute(c *gin.Context, options RateLimitOptions) (bool, error) {
	resource := routeResource(c)
	limit, interval, ok := routeLimit(resource, options)
	if !ok {
		return false, nil
	}
	allowed, err := options.Store.Allow(c.Request.Context(), "route:"+resource, limit, interval)
	if err != nil {
		return false, fmt.Errorf("route rate limit: %w", err)
	}
	return !allowed, nil
}

// routeLimit 获取路由限流配置
func routeLimit(resource string, options RateLimitOptions) (int, time.Duration, bool) {
	for _, rule := range options.Rules {
		if strings.TrimSpace(rule.Resource) != resource {
			continue
		}
		if rule.Count <= 0 {
			return 0, 0, false
		}
		interval := rule.Interval
		if interval <= 0 {
			interval = time.Second
		}
		return rule.Count, interval, true
	}
	if options.MaxQPS <= 0 {
		return 0, 0, false
	}
	return options.MaxQPS, time.Second, true
}

// limitUser 限制用户请求速率
func limitUser(c *gin.Context, options RateLimitOptions) (bool, error) {
	if !options.UserEnabled || options.UserRate <= 0 {
		return false, nil
	}
	userID := strings.TrimSpace(c.GetHeader("X-User-Id"))
	if userID == "" {
		return false, nil
	}
	if !isPositiveUserID(userID) {
		return false, nil
	}
	allowed, err := options.Store.Allow(c.Request.Context(), "user:"+userID, options.UserRate, options.UserInterval)
	if err != nil {
		return false, fmt.Errorf("user rate limit: %w", err)
	}
	return !allowed, nil
}

// isPositiveUserID 检查是否为有效的正整数用户 ID
func isPositiveUserID(userID string) bool {
	parsed, err := strconv.ParseInt(userID, 10, 64)
	return err == nil && parsed > 0
}

// abortRateLimited 中止请求并返回限流响应
func abortRateLimited(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusTooManyRequests, rateLimitResponse{
		Code:           "rate_limited",
		Message:        "请求过于频繁，请稍后再试",
		Success:        false,
		RequestTraceID: tracing.TraceID(c.Request.Context()),
	})
}

// LocalRateLimitStore 进程本地限流存储（令牌桶算法）
type LocalRateLimitStore struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiter
	now      func() time.Time
}

// NewLocalRateLimitStore 创建本地限流存储
func NewLocalRateLimitStore(now func() time.Time) *LocalRateLimitStore {
	if now == nil {
		now = time.Now
	}
	return &LocalRateLimitStore{
		limiters: make(map[string]*rateLimiter),
		now:      now,
	}
}

// Allow 检查是否允许请求
func (s *LocalRateLimitStore) Allow(_ context.Context, key string, limit int, interval time.Duration) (bool, error) {
	s.mu.Lock()
	limiter, ok := s.limiters[key]
	if !ok || !limiter.sameLimit(limit, interval) {
		limiter = newRateLimiterForInterval(limit, interval, s.now)
		s.limiters[key] = limiter
	}
	s.mu.Unlock()
	return limiter.Allow(), nil
}

// RedisRateLimitStore Redis 限流存储
type RedisRateLimitStore struct {
	client redisRateLimitClient
	prefix string
	now    func() time.Time // 可注入的时钟（用于测试）
}

// redisRateLimitClient Redis 客户端接口
type redisRateLimitClient interface {
	// 原有方法（固定窗口使用）
	Incr(ctx context.Context, key string) *goredis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *goredis.BoolCmd

	// 新增方法（滑动窗口使用）
	ZRemRangeByScore(ctx context.Context, key string, min, max string) *goredis.IntCmd
	ZCard(ctx context.Context, key string) *goredis.IntCmd
	ZAdd(ctx context.Context, key string, members ...goredis.Z) *goredis.IntCmd
	Pipeline() goredis.Pipeliner
}

// NewRedisRateLimitStore 创建 Redis 限流存储
func NewRedisRateLimitStore(client redisRateLimitClient, prefix string) *RedisRateLimitStore {
	if prefix == "" {
		prefix = "seckill:gateway:rate_limit"
	}
	return &RedisRateLimitStore{
		client: client,
		prefix: prefix,
		now:    time.Now,
	}
}

// Allow 检查是否允许请求（滑动窗口算法，两次 Pipeline 实现）
func (s *RedisRateLimitStore) Allow(ctx context.Context, key string, limit int, interval time.Duration) (bool, error) {
	if s == nil || s.client == nil {
		return true, nil
	}
	if limit <= 0 {
		return true, nil
	}
	if interval <= 0 {
		interval = time.Second
	}

	now := s.now().UnixMilli()
	windowStart := now - interval.Milliseconds()
	redisKey := fmt.Sprintf("%s:%s", s.prefix, key)

	// Pipeline 1: 清理过期 + 统计当前窗口请求数
	pipe1 := s.client.Pipeline()
	pipe1.ZRemRangeByScore(ctx, redisKey, "-inf", strconv.FormatInt(windowStart, 10))
	countCmd := pipe1.ZCard(ctx, redisKey)
	_, err := pipe1.Exec(ctx)
	if err != nil {
		// 降级：Redis 不可用时放行请求（fail-open）
		return true, nil
	}

	count := countCmd.Val()
	if count >= int64(limit) {
		return false, nil // 已超限，拒绝
	}

	// Pipeline 2: 写入本次请求 + 设置 TTL
	member := fmt.Sprintf("%d:%d", now, rand.Int63())
	pipe2 := s.client.Pipeline()
	pipe2.ZAdd(ctx, redisKey, goredis.Z{Score: float64(now), Member: member})
	pipe2.Expire(ctx, redisKey, interval+5*time.Second)
	_, err = pipe2.Exec(ctx)
	if err != nil {
		return true, nil // 降级：写入失败时放行请求
	}

	return true, nil
}

// rateLimiter 令牌桶限流器
type rateLimiter struct {
	mu       sync.Mutex
	now      func() time.Time
	limit    int
	interval time.Duration
	rate     float64
	capacity float64
	tokens   float64
	last     time.Time
}

// newRateLimiterForInterval 创建指定间隔的限流器
func newRateLimiterForInterval(limit int, interval time.Duration, now func() time.Time) *rateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if interval <= 0 {
		interval = time.Second
	}
	if now == nil {
		now = time.Now
	}
	capacity := float64(limit)
	return &rateLimiter{
		now:      now,
		limit:    limit,
		interval: interval,
		rate:     capacity / interval.Seconds(),
		capacity: capacity,
		tokens:   capacity,
		last:     now(),
	}
}

// sameLimit 检查限流配置是否相同
func (l *rateLimiter) sameLimit(limit int, interval time.Duration) bool {
	if interval <= 0 {
		interval = time.Second
	}
	return l.limit == limit && l.interval == interval
}

// Allow 检查是否允许请求（令牌桶算法）
func (l *rateLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.After(l.last) {
		l.tokens += now.Sub(l.last).Seconds() * l.rate
		if l.tokens > l.capacity {
			l.tokens = l.capacity
		}
		l.last = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
