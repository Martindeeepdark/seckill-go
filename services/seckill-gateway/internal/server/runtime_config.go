package server

import (
	"sync"
	"time"
)

// RateLimitRuntimeOptions 保存运行时限流开关和规则。
type RateLimitRuntimeOptions struct {
	Enabled bool
	Options RateLimitOptions
}

// DegradeOptions 保存运行时降级配置。
type DegradeOptions struct {
	Enabled          bool
	FailureThreshold int
	Timeout          time.Duration
}

// RuntimeConfigSnapshot 是所有动态配置的值拷贝，供一次性读取使用。
type RuntimeConfigSnapshot struct {
	RiskEnabled         bool
	MachineCheckEnabled bool
	RateLimit           RateLimitRuntimeOptions
	Degrade             DegradeOptions
}

// GatewayRuntimeConfig 保存网关运行期可热更新的治理配置。
type GatewayRuntimeConfig struct {
	mu                 sync.RWMutex
	rateLimit          RateLimitRuntimeOptions
	degrade            DegradeOptions
	riskEnabled        bool
	machineCheckEnabled bool
}

// NewGatewayRuntimeConfig 创建运行时配置容器。
func NewGatewayRuntimeConfig() *GatewayRuntimeConfig {
	return &GatewayRuntimeConfig{}
}

// UpdateRateLimit 覆盖运行时限流配置。
func (c *GatewayRuntimeConfig) UpdateRateLimit(enabled bool, options RateLimitOptions) {
	if c == nil {
		return
	}
	options.Rules = copyRateLimitRules(options.Rules)
	c.mu.Lock()
	c.rateLimit = RateLimitRuntimeOptions{Enabled: enabled, Options: options}
	c.mu.Unlock()
}

// RateLimit 读取当前限流配置快照。
func (c *GatewayRuntimeConfig) RateLimit() RateLimitRuntimeOptions {
	if c == nil {
		return RateLimitRuntimeOptions{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return RateLimitRuntimeOptions{
		Enabled: c.rateLimit.Enabled,
		Options: copyRateLimitOptions(c.rateLimit.Options),
	}
}

// UpdateDegrade 覆盖运行时降级配置。
func (c *GatewayRuntimeConfig) UpdateDegrade(options DegradeOptions) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.degrade = options
	c.mu.Unlock()
}

// Degrade 读取当前降级配置快照。
func (c *GatewayRuntimeConfig) Degrade() DegradeOptions {
	if c == nil {
		return DegradeOptions{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.degrade
}

// UpdateRisk 覆盖运行时风控开关。
func (c *GatewayRuntimeConfig) UpdateRisk(enabled bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.riskEnabled = enabled
	c.mu.Unlock()
}

// RiskEnabled 读取当前风控开关。
func (c *GatewayRuntimeConfig) RiskEnabled() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.riskEnabled
}

// UpdateMachineCheck 覆盖运行时机审开关。
func (c *GatewayRuntimeConfig) UpdateMachineCheck(enabled bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.machineCheckEnabled = enabled
	c.mu.Unlock()
}

// MachineCheckEnabled 读取当前机审开关。
func (c *GatewayRuntimeConfig) MachineCheckEnabled() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.machineCheckEnabled
}

// Snapshot 返回所有动态配置的值拷贝。
func (c *GatewayRuntimeConfig) Snapshot() RuntimeConfigSnapshot {
	if c == nil {
		return RuntimeConfigSnapshot{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return RuntimeConfigSnapshot{
		RiskEnabled:         c.riskEnabled,
		MachineCheckEnabled: c.machineCheckEnabled,
		RateLimit: RateLimitRuntimeOptions{
			Enabled: c.rateLimit.Enabled,
			Options: copyRateLimitOptions(c.rateLimit.Options),
		},
		Degrade: c.degrade,
	}
}

func copyRateLimitOptions(options RateLimitOptions) RateLimitOptions {
	options.Rules = copyRateLimitRules(options.Rules)
	return options
}

func copyRateLimitRules(rules []RateLimitRule) []RateLimitRule {
	if len(rules) == 0 {
		return nil
	}
	copied := make([]RateLimitRule, len(rules))
	copy(copied, rules)
	return copied
}
