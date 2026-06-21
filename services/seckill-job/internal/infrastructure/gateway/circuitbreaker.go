package gateway

import (
	"log/slog"
	"time"

	"github.com/go-kratos/aegis/circuitbreaker"
	"github.com/go-kratos/aegis/circuitbreaker/sre"

	"seckill-common/errors"
)

// CircuitBreaker 封装 aegis SRE 熔断器，提供简洁的 Execute 接口。
// 每个下游服务（activity、order、stock 等）使用独立的熔断器实例。
type CircuitBreaker struct {
	cb     circuitbreaker.CircuitBreaker
	name   string
	logger *slog.Logger
}

// CircuitBreakerConfig 熔断器配置参数。
type CircuitBreakerConfig struct {
	// Enabled 是否启用应用层熔断。
	Enabled bool
	// Success 成功率阈值（0~1），降低此值会使熔断更激进，默认 0.6。
	Success float64
	// Request 触发熔断计算的最小请求数，默认 100。
	Request int64
	// Window 统计窗口时长，默认 10s。
	Window time.Duration
}

// DefaultCircuitBreakerConfig 返回默认熔断器配置。
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Enabled: true,
		Success: 0.6,
		Request: 100,
		Window:  10 * time.Second,
	}
}

// NewCircuitBreaker 根据配置创建熔断器。
// name 用于日志标识（如 "activity"、"order" 等）。
func NewCircuitBreaker(name string, cfg CircuitBreakerConfig, logger *slog.Logger) *CircuitBreaker {
	opts := []sre.Option{
		sre.WithSuccess(cfg.Success),
		sre.WithRequest(cfg.Request),
	}
	if cfg.Window > 0 {
		opts = append(opts, sre.WithWindow(cfg.Window))
	}
	return &CircuitBreaker{
		cb:     sre.NewBreaker(opts...),
		name:   name,
		logger: logger,
	}
}

// Execute 在熔断保护下执行函数 fn。
// 当熔断器判定不应放行时，返回 ErrCircuitOpen，调用方可据此进行降级。
// fn 的执行成功/失败会反馈给熔断器用于统计。
func (b *CircuitBreaker) Execute(fn func() error) error {
	if err := b.cb.Allow(); err != nil {
		// 请求被熔断器拒绝，同时记录失败以进一步升高拒绝率
		b.cb.MarkFailed()
		b.logger.Warn("熔断器拒绝请求，服务降级", "service", b.name)
		return errors.ErrCircuitOpen
	}
	err := fn()
	if err != nil {
		b.cb.MarkFailed()
	} else {
		b.cb.MarkSuccess()
	}
	return err
}
