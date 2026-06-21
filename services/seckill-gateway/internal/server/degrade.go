// Package server 提供 HTTP 服务器和中间件
package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"seckill-common/tracing"
)

// degradeResponse 降级响应
type degradeResponse struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Success        bool   `json:"success"`
	RequestTraceID string `json:"requestTraceId,omitempty"`
}

// DegradeMiddleware 创建降级中间件
func DegradeMiddleware(failureThreshold int, timeout time.Duration) gin.HandlerFunc {
	return newDegradeMiddleware(newDegradeController(failureThreshold, timeout, time.Now))
}

// DynamicDegradeMiddleware 从 RuntimeConfig 读取降级配置，每次请求动态读取。
// 每次调用创建独立的 degradeController，适合不需要跨请求共享断路器状态的场景。
func DynamicDegradeMiddleware(rc *GatewayRuntimeConfig) gin.HandlerFunc {
	return DynamicDegradeMiddlewareWithController(rc, nil)
}

// DynamicDegradeMiddlewareWithController 从 RuntimeConfig 读取降级开关，
// 使用共享 controller 维护断路器状态。
func DynamicDegradeMiddlewareWithController(rc *GatewayRuntimeConfig, controller *degradeController) gin.HandlerFunc {
	return func(c *gin.Context) {
		degrade := rc.Degrade()
		if !degrade.Enabled {
			c.Next()
			return
		}
		threshold := degrade.FailureThreshold
		if threshold <= 0 {
			threshold = 5
		}
		timeout := degrade.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}

		// 如果没有共享 controller，按需创建（threshold/timeout 动态传入）
		ctrl := controller
		if ctrl == nil {
			ctrl = newDegradeController(threshold, timeout, time.Now)
		}

		resource := routeResource(c)
		if !ctrl.Allow(resource) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, degradeResponse{
				Code:           "service_degraded",
				Message:        "服务繁忙，请稍后再试",
				Success:        false,
				RequestTraceID: tracing.TraceID(c.Request.Context()),
			})
			return
		}

		c.Next()

		ctrl.Observe(resource, requestFailed(c))
	}
}

// newDegradeMiddleware 创建降级中间件实例
func newDegradeMiddleware(controller *degradeController) gin.HandlerFunc {
	return func(c *gin.Context) {
		resource := routeResource(c)
		if controller != nil && !controller.Allow(resource) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, degradeResponse{
				Code:           "service_degraded",
				Message:        "服务繁忙，请稍后再试",
				Success:        false,
				RequestTraceID: tracing.TraceID(c.Request.Context()),
			})
			return
		}

		c.Next()

		if controller != nil {
			controller.Observe(resource, requestFailed(c))
		}
	}
}

// routeResource 获取路由资源标识
func routeResource(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return "unknown"
	}
	if fullPath := c.FullPath(); fullPath != "" {
		return c.Request.Method + " " + fullPath
	}
	return c.Request.Method + " " + c.Request.URL.Path
}

// requestFailed 检查请求是否失败
func requestFailed(c *gin.Context) bool {
	if c == nil {
		return false
	}
	status := c.Writer.Status()
	return status >= http.StatusInternalServerError || len(c.Errors) > 0
}

// degradeController 降级控制器
type degradeController struct {
	mu               sync.Mutex
	breakers         map[string]*routeBreaker
	failureThreshold int
	timeout          time.Duration
	now              func() time.Time
}

// newDegradeController 创建降级控制器
func newDegradeController(failureThreshold int, timeout time.Duration, now func() time.Time) *degradeController {
	if failureThreshold <= 0 {
		failureThreshold = 1
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if now == nil {
		now = time.Now
	}
	return &degradeController{
		breakers:         make(map[string]*routeBreaker),
		failureThreshold: failureThreshold,
		timeout:          timeout,
		now:              now,
	}
}

// Allow 检查是否允许请求
func (c *degradeController) Allow(resource string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.breaker(resource).allow(c.now(), c.timeout)
}

// Observe 观察请求结果
func (c *degradeController) Observe(resource string, failed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.breaker(resource).observe(failed, c.now(), c.failureThreshold)
}

// breaker 获取或创建路由断路器
func (c *degradeController) breaker(resource string) *routeBreaker {
	if resource == "" {
		resource = "unknown"
	}
	breaker, ok := c.breakers[resource]
	if !ok {
		breaker = &routeBreaker{state: breakerClosed}
		c.breakers[resource] = breaker
	}
	return breaker
}

// breakerState 断路器状态
type breakerState int

const (
	breakerClosed   breakerState = iota // 关闭
	breakerOpen                         // 开启
	breakerHalfOpen                     // 半开
)

// routeBreaker 路由断路器
type routeBreaker struct {
	state            breakerState
	failures         int
	openedAt         time.Time
	halfOpenInFlight bool
}

// allow 检查是否允许请求通过
func (b *routeBreaker) allow(now time.Time, timeout time.Duration) bool {
	switch b.state {
	case breakerOpen:
		if now.Sub(b.openedAt) < timeout {
			return false
		}
		b.state = breakerHalfOpen
		b.halfOpenInFlight = true
		return true
	case breakerHalfOpen:
		if b.halfOpenInFlight {
			return false
		}
		b.halfOpenInFlight = true
		return true
	default:
		return true
	}
}

// observe 观察请求结果并更新断路器状态
func (b *routeBreaker) observe(failed bool, now time.Time, threshold int) {
	switch b.state {
	case breakerHalfOpen:
		b.halfOpenInFlight = false
		if failed {
			b.open(now)
			return
		}
		b.close()
	case breakerOpen:
		return
	default:
		if failed {
			b.failures++
			if b.failures >= threshold {
				b.open(now)
			}
			return
		}
		b.failures = 0
	}
}

// open 打开断路器
func (b *routeBreaker) open(now time.Time) {
	b.state = breakerOpen
	b.openedAt = now
	b.failures = 0
	b.halfOpenInFlight = false
}

// close 关闭断路器
func (b *routeBreaker) close() {
	b.state = breakerClosed
	b.failures = 0
	b.halfOpenInFlight = false
}
