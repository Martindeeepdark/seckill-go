package server

import (
	"testing"
	"time"

	"seckill-gateway-service/internal/config"
)

func TestApplyGatewayConfigAllFields(t *testing.T) {
	rt := NewGatewayRuntimeConfig()
	cfg := config.GatewayConfig{
		RateLimit: config.RateLimitConfig{
			Enabled:     true,
			MaxQPS:      500,
			UserEnabled: true,
			UserRate:    10,
			UserInterval: 5 * time.Second,
			Rules: []config.RateLimitRule{
				{Resource: "/api/seckill", Count: 100, Interval: time.Second},
			},
		},
		Degrade: config.DegradeConfig{
			Enabled:          true,
			FailureThreshold: 7,
			Timeout:          30 * time.Second,
		},
		Risk: config.RiskConfig{Enabled: true},
		MachineCheck: config.MachineCheckConfig{Enabled: false},
	}

	ApplyGatewayConfig(rt, cfg)

	snap := rt.Snapshot()

	if !snap.RateLimit.Enabled {
		t.Error("rateLimit enabled not applied")
	}
	if snap.RateLimit.Options.MaxQPS != 500 {
		t.Errorf("maxQPS = %d, want 500", snap.RateLimit.Options.MaxQPS)
	}
	if !snap.RateLimit.Options.UserEnabled {
		t.Error("userEnabled not applied")
	}
	if snap.RateLimit.Options.UserRate != 10 {
		t.Errorf("userRate = %d, want 10", snap.RateLimit.Options.UserRate)
	}
	if len(snap.RateLimit.Options.Rules) != 1 {
		t.Errorf("rules len = %d, want 1", len(snap.RateLimit.Options.Rules))
	} else if snap.RateLimit.Options.Rules[0].Resource != "/api/seckill" {
		t.Errorf("rule resource = %q, want /api/seckill", snap.RateLimit.Options.Rules[0].Resource)
	}

	if !snap.Degrade.Enabled {
		t.Error("degrade enabled not applied")
	}
	if snap.Degrade.FailureThreshold != 7 {
		t.Errorf("degrade threshold = %d, want 7", snap.Degrade.FailureThreshold)
	}
	if snap.Degrade.Timeout != 30*time.Second {
		t.Errorf("degrade timeout = %v, want 30s", snap.Degrade.Timeout)
	}

	if !snap.RiskEnabled {
		t.Error("risk enabled not applied")
	}
	if snap.MachineCheckEnabled {
		t.Error("machineCheck should be false")
	}
}

func TestApplyGatewayConfigNilReceiver(t *testing.T) {
	var rt *GatewayRuntimeConfig
	cfg := config.GatewayConfig{Risk: config.RiskConfig{Enabled: true}}

	ApplyGatewayConfig(rt, cfg)
}

func TestApplyGatewayConfigOverwritesPrevious(t *testing.T) {
	rt := NewGatewayRuntimeConfig()
	rt.UpdateRisk(true)
	rt.UpdateRateLimit(true, RateLimitOptions{MaxQPS: 999})

	ApplyGatewayConfig(rt, config.GatewayConfig{
		RateLimit: config.RateLimitConfig{Enabled: false, MaxQPS: 100},
		Risk:      config.RiskConfig{Enabled: false},
	})

	snap := rt.Snapshot()
	if snap.RateLimit.Enabled {
		t.Error("rateLimit should be overwritten to false")
	}
	if snap.RateLimit.Options.MaxQPS != 100 {
		t.Errorf("maxQPS = %d, want 100", snap.RateLimit.Options.MaxQPS)
	}
	if snap.RiskEnabled {
		t.Error("risk should be overwritten to false")
	}
}

// TestApplyGatewaySegmentPreservesBaseFields 验证 partial etcd PUT 不会丢失 base 字段。
// 场景：base 含完整 gateway.rate_limit（含 rules/user_enabled），etcd 段只覆盖 rate_limit.max_qps。
// 期望：runtime 中 max_qps=500（被覆盖），rules/user_enabled/enabled 保留 base 值。
// 此契约之前在 main.go 的 applyEtcdConfig 中被违反——直接部分覆盖会清零未提及的字段。
func TestApplyGatewaySegmentPreservesBaseFields(t *testing.T) {
	base := map[string]any{
		"gateway": map[string]any{
			"rate_limit": map[string]any{
				"enabled":      true,
				"max_qps":      100,
				"user_enabled": true,
				"user_rate":    10,
				"rules": []any{
					map[string]any{"resource": "POST /api/seckill", "count": 1000, "interval": "1s"},
				},
			},
			"degrade": map[string]any{
				"enabled":           true,
				"failure_threshold": 5,
				"timeout":           "10s",
			},
			"risk": map[string]any{"enabled": true},
		},
	}
	etcdSegment := map[string]any{
		"rate_limit": map[string]any{
			"max_qps": 500,
		},
	}

	rt := NewGatewayRuntimeConfig()
	ApplyGatewaySegment(rt, base, etcdSegment)

	snap := rt.Snapshot()
	if snap.RateLimit.Options.MaxQPS != 500 {
		t.Errorf("maxQPS = %d, want 500 (overlay)", snap.RateLimit.Options.MaxQPS)
	}
	if !snap.RateLimit.Enabled {
		t.Error("enabled should be preserved from base")
	}
	if !snap.RateLimit.Options.UserEnabled {
		t.Error("user_enabled should be preserved from base")
	}
	if snap.RateLimit.Options.UserRate != 10 {
		t.Errorf("user_rate = %d, want 10 (base)", snap.RateLimit.Options.UserRate)
	}
	if len(snap.RateLimit.Options.Rules) != 1 {
		t.Errorf("rules len = %d, want 1 (preserved from base)", len(snap.RateLimit.Options.Rules))
	}
	if !snap.Degrade.Enabled {
		t.Error("degrade.enabled should be preserved from base (not in overlay)")
	}
	if !snap.RiskEnabled {
		t.Error("risk.enabled should be preserved from base (not in overlay)")
	}
}

func TestApplyGatewaySegmentNilOverlay(t *testing.T) {
	base := map[string]any{
		"gateway": map[string]any{
			"risk": map[string]any{"enabled": true},
		},
	}

	rt := NewGatewayRuntimeConfig()
	ApplyGatewaySegment(rt, base, nil)

	if !rt.RiskEnabled() {
		t.Error("nil overlay should apply base as-is")
	}
}

func TestApplyGatewaySegmentNilReceiver(t *testing.T) {
	var rt *GatewayRuntimeConfig
	ApplyGatewaySegment(rt, map[string]any{}, map[string]any{})
}

