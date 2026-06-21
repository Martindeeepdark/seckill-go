package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaultsRPCDiscoveryAndCircuitBreaker(t *testing.T) {
	path := writeConfig(t, "log:\n  level: info\n")

	_, _, rpc, appCB, err := Load(path)
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
	if !appCB.Enabled {
		t.Fatal("AppCB.Enabled = false, want true")
	}
}

func TestLoadRPCConfiguredEndpointWins(t *testing.T) {
	path := writeConfig(t, `
rpc:
  order: 127.0.0.1:19004
  circuit_breaker:
    enabled: false
`)

	_, _, rpc, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if rpc.CircuitBreaker {
		t.Fatal("CircuitBreaker = true, want false")
	}
	if rpc.OrderAddr != "127.0.0.1:19004" {
		t.Fatalf("OrderAddr = %q, want configured endpoint", rpc.OrderAddr)
	}
	if rpc.StockAddr != "discovery:///stock-service" {
		t.Fatalf("StockAddr = %q, want discovery endpoint", rpc.StockAddr)
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
