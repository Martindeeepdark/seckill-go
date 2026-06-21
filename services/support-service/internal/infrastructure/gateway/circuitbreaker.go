package gateway

import (
	"log/slog"
	"sync"
	"time"

	"seckill-support-service/internal/domain"
)

// cbState 熔断器状态
type cbState int

const (
	cbClosed   cbState = iota // 关闭（正常放行）
	cbOpen                    // 打开（熔断中，拒绝请求）
	cbHalfOpen                // 半开（试探性放行，验证下游是否恢复）
)

// ErrCircuitOpen 熔断器打开时返回的错误（定义在 domain 包中避免循环依赖）
var ErrCircuitOpen = domain.ErrCircuitOpen

// CircuitBreaker 应用层熔断器
// 三态模型：Closed -> Open -> HalfOpen -> Closed
// 不依赖任何第三方库，仅使用标准库实现。
type CircuitBreaker struct {
	mu           sync.Mutex
	name         string        // 熔断器名称，用于日志标识
	state        cbState       // 当前状态
	failures     int           // 连续失败计数
	maxFailures  int           // 触发熔断的最大连续失败次数
	resetTimeout time.Duration // 熔断打开后等待多久进入半开状态
	lastFailure  time.Time     // 上一次失败的时间
	logger       *slog.Logger
}

// NewCircuitBreaker 创建应用层熔断器
//   - name: 熔断器名称，用于日志和监控
//   - maxFailures: 连续失败达到此数值后触发熔断
//   - resetTimeout: 熔断打开后，等待此时间进入半开状态进行试探
//   - logger: 日志记录器
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration, logger *slog.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		state:        cbClosed,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		logger:       logger,
	}
}

// Execute 在熔断保护下执行函数
// 如果熔断器处于打开状态，直接返回 ErrCircuitOpen。
// 函数执行成功则重置计数器；失败则记录失败，达到阈值时触发熔断。
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allow() {
		return ErrCircuitOpen
	}
	err := fn()
	if err != nil {
		cb.recordFailure()
		return err
	}
	cb.recordSuccess()
	return nil
}

// allow 判断当前是否允许请求通过
func (cb *CircuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case cbClosed:
		return true
	case cbOpen:
		// 超过冷却时间，进入半开状态试探
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = cbHalfOpen
			cb.logger.Info("熔断器进入半开状态", "service", cb.name)
			return true
		}
		return false
	case cbHalfOpen:
		return true
	}
	return true
}

// recordFailure 记录一次失败
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.state == cbHalfOpen {
		// 半开状态下再次失败，立即回到打开状态
		cb.state = cbOpen
		cb.logger.Warn("熔断器在半开状态下再次失败，重新打开", "service", cb.name)
		return
	}
	if cb.failures >= cb.maxFailures {
		cb.state = cbOpen
		cb.logger.Warn("熔断器打开，停止向下游服务发起请求",
			"service", cb.name,
			"failures", cb.failures,
			"maxFailures", cb.maxFailures,
			"resetTimeout", cb.resetTimeout,
		)
	}
}

// recordSuccess 记录一次成功
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == cbHalfOpen {
		cb.state = cbClosed
		cb.logger.Info("熔断器恢复关闭状态，下游服务已恢复正常", "service", cb.name)
	}
	cb.failures = 0
}
