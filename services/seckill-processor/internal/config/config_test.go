package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaultsRPCDiscoveryAndCircuitBreaker(t *testing.T) {
	path := writeConfig(t, "log:\n  level: info\n")

	_, _, rpc, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !rpc.CircuitBreaker {
		t.Fatal("CircuitBreaker = false, want true")
	}
	if rpc.Timeout != 3*time.Second {
		t.Fatalf("Timeout = %s, want 3s", rpc.Timeout)
	}
	if rpc.ActivityAddr != "discovery:///activity-service" {
		t.Fatalf("ActivityAddr = %q, want discovery endpoint", rpc.ActivityAddr)
	}
	if rpc.SupportAddr != "discovery:///support-service" {
		t.Fatalf("SupportAddr = %q, want discovery endpoint", rpc.SupportAddr)
	}
}

func TestLoadRPCConfiguredEndpointWins(t *testing.T) {
	path := writeConfig(t, `
rpc:
  activity: 127.0.0.1:19001
  circuit_breaker:
    enabled: false
    success: 0.7
    request: 42
    window: 5s
    bucket: 5
`)

	_, _, rpc, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if rpc.CircuitBreaker {
		t.Fatal("CircuitBreaker = true, want false")
	}
	if rpc.ActivityAddr != "127.0.0.1:19001" {
		t.Fatalf("ActivityAddr = %q, want configured endpoint", rpc.ActivityAddr)
	}
	if rpc.StockAddr != "discovery:///stock-service" {
		t.Fatalf("StockAddr = %q, want discovery endpoint", rpc.StockAddr)
	}
	if rpc.CircuitBreakerPolicy.Success != 0.7 {
		t.Fatalf("CircuitBreakerPolicy.Success = %v, want 0.7", rpc.CircuitBreakerPolicy.Success)
	}
	if rpc.CircuitBreakerPolicy.Request != 42 {
		t.Fatalf("CircuitBreakerPolicy.Request = %d, want 42", rpc.CircuitBreakerPolicy.Request)
	}
	if rpc.CircuitBreakerPolicy.Window != 5*time.Second {
		t.Fatalf("CircuitBreakerPolicy.Window = %s, want 5s", rpc.CircuitBreakerPolicy.Window)
	}
	if rpc.CircuitBreakerPolicy.Bucket != 5 {
		t.Fatalf("CircuitBreakerPolicy.Bucket = %d, want 5", rpc.CircuitBreakerPolicy.Bucket)
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
