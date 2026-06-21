package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRawReturnsMergedMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
gateway:
  rate_limit:
    enabled: true
    max_qps: 250
  risk:
    enabled: false
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	raw, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("LoadRaw returned error: %v", err)
	}

	gw, ok := raw["gateway"].(map[string]any)
	if !ok {
		t.Fatal("raw[\"gateway\"] should be a map")
	}
	rl, ok := gw["rate_limit"].(map[string]any)
	if !ok {
		t.Fatal("gateway.rate_limit should be a map")
	}
	if rl["enabled"] != true {
		t.Error("rate_limit.enabled should be true from file")
	}
	maxQPS, ok := rl["max_qps"].(int)
	if !ok || maxQPS != 250 {
		t.Errorf("max_qps = %v, want 250", rl["max_qps"])
	}
}

func TestLoadRawContainsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	raw, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("LoadRaw returned error: %v", err)
	}

	gw, ok := raw["gateway"].(map[string]any)
	if !ok {
		t.Fatal("raw[\"gateway\"] should exist from defaults")
	}
	rl, ok := gw["rate_limit"].(map[string]any)
	if !ok {
		t.Fatal("gateway.rate_limit should exist from defaults")
	}
	if rl["enabled"] != false {
		t.Error("default rate_limit.enabled should be false")
	}
}

func TestLoadRateLimitUserConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
gateway:
  rate_limit:
    enabled: true
    max_qps: 300
    rules:
      - resource: POST /api/seckill/part-in
        count: 1000
        interval: 1s
      - resource: GET /api/admin/activities
        count: 200
        interval: 5s
      - resource: ""
        count: 0
    user_enabled: true
    user_rate: 12
    user_interval: 15s
    user_expire: 6m
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if !cfg.Gateway.RateLimit.Enabled {
		t.Fatal("rate limit should be enabled")
	}
	if cfg.Gateway.RateLimit.MaxQPS != 300 {
		t.Fatalf("max qps = %d, want 300", cfg.Gateway.RateLimit.MaxQPS)
	}
	if len(cfg.Gateway.RateLimit.Rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(cfg.Gateway.RateLimit.Rules))
	}
	firstRule := cfg.Gateway.RateLimit.Rules[0]
	if firstRule.Resource != "POST /api/seckill/part-in" || firstRule.Count != 1000 || firstRule.Interval != time.Second {
		t.Fatalf("first rule = %+v, want part-in 1000/1s", firstRule)
	}
	secondRule := cfg.Gateway.RateLimit.Rules[1]
	if secondRule.Resource != "GET /api/admin/activities" || secondRule.Count != 200 || secondRule.Interval != 5*time.Second {
		t.Fatalf("second rule = %+v, want admin 200/5s", secondRule)
	}
	if !cfg.Gateway.RateLimit.UserEnabled {
		t.Fatal("user rate limit should be enabled")
	}
	if cfg.Gateway.RateLimit.UserRate != 12 {
		t.Fatalf("user rate = %d, want 12", cfg.Gateway.RateLimit.UserRate)
	}
	if cfg.Gateway.RateLimit.UserInterval != 15*time.Second {
		t.Fatalf("user interval = %s, want 15s", cfg.Gateway.RateLimit.UserInterval)
	}
	if cfg.Gateway.RateLimit.UserExpire != 6*time.Minute {
		t.Fatalf("user expire = %s, want 6m", cfg.Gateway.RateLimit.UserExpire)
	}
}
