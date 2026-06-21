package rpcclient

import (
	"testing"
	"time"
)

// --- Connection Pool Tuning Tests ---

func TestConnectionPoolDefaults(t *testing.T) {
	cfg := ConnectionPoolConfig{}
	cfg = cfg.ApplyDefaults()

	if cfg.InitialWindowSize != 262144 {
		t.Errorf("InitialWindowSize = %d, want 262144", cfg.InitialWindowSize)
	}
	if cfg.InitialConnWindowSize != 1048576 {
		t.Errorf("InitialConnWindowSize = %d, want 1048576", cfg.InitialConnWindowSize)
	}
	if cfg.MaxRecvMsgSize != 4194304 {
		t.Errorf("MaxRecvMsgSize = %d, want 4194304", cfg.MaxRecvMsgSize)
	}
	if cfg.KeepaliveTime != 30*time.Second {
		t.Errorf("KeepaliveTime = %v, want 30s", cfg.KeepaliveTime)
	}
	if cfg.KeepaliveTimeout != 10*time.Second {
		t.Errorf("KeepaliveTimeout = %v, want 10s", cfg.KeepaliveTimeout)
	}
}

func TestConnectionPoolDefaultsPreserveSetValues(t *testing.T) {
	cfg := ConnectionPoolConfig{
		InitialWindowSize:     65536,
		InitialConnWindowSize: 524288,
		MaxRecvMsgSize:        2097152,
		KeepaliveTime:         15 * time.Second,
		KeepaliveTimeout:      5 * time.Second,
	}
	applied := cfg.ApplyDefaults()

	if applied.InitialWindowSize != 65536 {
		t.Errorf("InitialWindowSize = %d, want 65536 (should preserve user value)", applied.InitialWindowSize)
	}
	if applied.InitialConnWindowSize != 524288 {
		t.Errorf("InitialConnWindowSize = %d, want 524288 (should preserve user value)", applied.InitialConnWindowSize)
	}
	if applied.MaxRecvMsgSize != 2097152 {
		t.Errorf("MaxRecvMsgSize = %d, want 2097152 (should preserve user value)", applied.MaxRecvMsgSize)
	}
	if applied.KeepaliveTime != 15*time.Second {
		t.Errorf("KeepaliveTime = %v, want 15s (should preserve user value)", applied.KeepaliveTime)
	}
	if applied.KeepaliveTimeout != 5*time.Second {
		t.Errorf("KeepaliveTimeout = %v, want 5s (should preserve user value)", applied.KeepaliveTimeout)
	}
}

func TestConnectionPoolConfigFromMap(t *testing.T) {
	raw := map[string]interface{}{
		"rpc": map[string]interface{}{
			"initial_window_size":      131072,
			"initial_conn_window_size": 524288,
			"max_recv_msg_size":        8388608,
			"keepalive_time":           "60s",
			"keepalive_timeout":        "20s",
		},
	}

	got := ConnectionPoolConfigFromMap(raw, "rpc")

	if got.InitialWindowSize != 131072 {
		t.Errorf("InitialWindowSize = %d, want 131072", got.InitialWindowSize)
	}
	if got.InitialConnWindowSize != 524288 {
		t.Errorf("InitialConnWindowSize = %d, want 524288", got.InitialConnWindowSize)
	}
	if got.MaxRecvMsgSize != 8388608 {
		t.Errorf("MaxRecvMsgSize = %d, want 8388608", got.MaxRecvMsgSize)
	}
	if got.KeepaliveTime != 60*time.Second {
		t.Errorf("KeepaliveTime = %v, want 60s", got.KeepaliveTime)
	}
	if got.KeepaliveTimeout != 20*time.Second {
		t.Errorf("KeepaliveTimeout = %v, want 20s", got.KeepaliveTimeout)
	}
}

func TestConnectionPoolConfigFromMapDefaultsWhenMissing(t *testing.T) {
	raw := map[string]interface{}{
		"rpc": map[string]interface{}{
			"timeout": "3s",
		},
	}

	got := ConnectionPoolConfigFromMap(raw, "rpc")
	applied := got.ApplyDefaults()

	if applied.InitialWindowSize != 262144 {
		t.Errorf("InitialWindowSize = %d, want default 262144", applied.InitialWindowSize)
	}
	if applied.MaxRecvMsgSize != 4194304 {
		t.Errorf("MaxRecvMsgSize = %d, want default 4194304", applied.MaxRecvMsgSize)
	}
}

func TestBuildGRPCDialOptions(t *testing.T) {
	cfg := ConnectionPoolConfig{
		InitialWindowSize:     262144,
		InitialConnWindowSize: 1048576,
		MaxRecvMsgSize:        4194304,
		KeepaliveTime:         30 * time.Second,
		KeepaliveTimeout:      10 * time.Second,
	}

	opts := cfg.GRPCDialOptions()
	if len(opts) != 4 {
		t.Fatalf("GRPCDialOptions() returned %d options, want 4", len(opts))
	}
}

func TestEndpointPrefersConfiguredValue(t *testing.T) {
	got := Endpoint("activity-service", "127.0.0.1:9001", "fallback:9001")
	if got != "127.0.0.1:9001" {
		t.Fatalf("Endpoint() = %q, want configured endpoint", got)
	}
}

func TestEndpointDefaultsToDiscoveryService(t *testing.T) {
	got := Endpoint("activity-service", "", "127.0.0.1:9001")
	if got != "discovery:///activity-service" {
		t.Fatalf("Endpoint() = %q, want discovery endpoint", got)
	}
}

func TestEndpointFallsBackWithoutServiceName(t *testing.T) {
	got := Endpoint("", "", "127.0.0.1:9001")
	if got != "127.0.0.1:9001" {
		t.Fatalf("Endpoint() = %q, want fallback endpoint", got)
	}
}

func TestCircuitBreakerPolicyFromMap(t *testing.T) {
	raw := map[string]interface{}{
		"rpc": map[string]interface{}{
			"circuit_breaker": map[string]interface{}{
				"success": "0.7",
				"request": 42,
				"window":  "5s",
				"bucket":  5,
			},
		},
	}

	got := CircuitBreakerPolicyFromMap(raw, "rpc.circuit_breaker")
	if got.Success != 0.7 {
		t.Fatalf("Success = %v, want 0.7", got.Success)
	}
	if got.Request != 42 {
		t.Fatalf("Request = %d, want 42", got.Request)
	}
	if got.Window != 5*time.Second {
		t.Fatalf("Window = %s, want 5s", got.Window)
	}
	if got.Bucket != 5 {
		t.Fatalf("Bucket = %d, want 5", got.Bucket)
	}
}
