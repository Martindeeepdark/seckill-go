package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadActivityCacheConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
services:
  activity-service:
    grpc:
      addr: 0.0.0.0:9001
discovery:
  mode: redis
  namespace: test-seckill
  static_fallback: true
  ttl: 9s
  refresh_interval: 3s
  services:
    activity-service:
      - 127.0.0.1:9001
  advertise:
    activity-service: activity-service:9001
trace:
  enabled: true
  endpoint: localhost:4317
  insecure: true
cache:
  activity:
    enabled: true
    max_size: 32
    local_ttl: 11s
    refresh_after: 2s
    redis_ttl: 22s
    null_ttl: 3s
    warmup_ahead: 4m
    refresh_enabled: true
    refresh_initial: 5s
    refresh_tick: 6s
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path, "activity-service")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GRPCAddr != "0.0.0.0:9001" {
		t.Fatalf("grpc addr = %q, want 0.0.0.0:9001", cfg.GRPCAddr)
	}
	if cfg.DiscoveryMode != "redis" {
		t.Fatalf("discovery mode = %q, want redis", cfg.DiscoveryMode)
	}
	if !cfg.TraceEnabled {
		t.Fatal("trace should be enabled")
	}
	if !cfg.Cache.Activity.Enabled {
		t.Fatal("activity cache should be enabled")
	}
	if cfg.Cache.Activity.MaxSize != 32 {
		t.Fatalf("activity cache max size = %d, want 32", cfg.Cache.Activity.MaxSize)
	}
	if cfg.Cache.Activity.LocalTTL != 11*time.Second {
		t.Fatalf("activity cache local ttl = %s, want 11s", cfg.Cache.Activity.LocalTTL)
	}
	if cfg.Cache.Activity.RefreshAfter != 2*time.Second {
		t.Fatalf("activity cache refresh after = %s, want 2s", cfg.Cache.Activity.RefreshAfter)
	}
	if cfg.Cache.Activity.RedisTTL != 22*time.Second {
		t.Fatalf("activity cache redis ttl = %s, want 22s", cfg.Cache.Activity.RedisTTL)
	}
	if cfg.Cache.Activity.NullTTL != 3*time.Second {
		t.Fatalf("activity cache null ttl = %s, want 3s", cfg.Cache.Activity.NullTTL)
	}
	if cfg.Cache.Activity.WarmupAhead != 4*time.Minute {
		t.Fatalf("activity cache warmup ahead = %s, want 4m", cfg.Cache.Activity.WarmupAhead)
	}
	if !cfg.Cache.Activity.RefreshEnabled {
		t.Fatal("activity cache refresh should be enabled")
	}
	if cfg.Cache.Activity.RefreshInitial != 5*time.Second {
		t.Fatalf("activity cache refresh initial = %s, want 5s", cfg.Cache.Activity.RefreshInitial)
	}
	if cfg.Cache.Activity.RefreshTick != 6*time.Second {
		t.Fatalf("activity cache refresh tick = %s, want 6s", cfg.Cache.Activity.RefreshTick)
	}
}
