package server

import (
	"sync"
	"testing"
)

func TestRuntimeConfigRiskEnabled(t *testing.T) {
	rc := NewGatewayRuntimeConfig()

	if rc.RiskEnabled() {
		t.Error("default should be false")
	}

	rc.UpdateRisk(true)
	if !rc.RiskEnabled() {
		t.Error("should be true after update")
	}

	rc.UpdateRisk(false)
	if rc.RiskEnabled() {
		t.Error("should be false after second update")
	}
}

func TestRuntimeConfigMachineCheckEnabled(t *testing.T) {
	rc := NewGatewayRuntimeConfig()

	if rc.MachineCheckEnabled() {
		t.Error("default should be false")
	}

	rc.UpdateMachineCheck(true)
	if !rc.MachineCheckEnabled() {
		t.Error("should be true after update")
	}
}

func TestRuntimeConfigSnapshot(t *testing.T) {
	rc := NewGatewayRuntimeConfig()
	rc.UpdateRisk(true)
	rc.UpdateMachineCheck(false)
	rc.UpdateRateLimit(true, RateLimitOptions{MaxQPS: 500})
	rc.UpdateDegrade(DegradeOptions{Enabled: true, FailureThreshold: 3})

	snap := rc.Snapshot()

	if !snap.RiskEnabled {
		t.Error("snapshot risk should be true")
	}
	if snap.MachineCheckEnabled {
		t.Error("snapshot machineCheck should be false")
	}
	if !snap.RateLimit.Enabled {
		t.Error("snapshot rateLimit should be enabled")
	}
	if snap.RateLimit.Options.MaxQPS != 500 {
		t.Errorf("snapshot maxQPS = %d, want 500", snap.RateLimit.Options.MaxQPS)
	}
	if !snap.Degrade.Enabled {
		t.Error("snapshot degrade should be enabled")
	}
	if snap.Degrade.FailureThreshold != 3 {
		t.Errorf("snapshot degrade threshold = %d, want 3", snap.Degrade.FailureThreshold)
	}
}

func TestRuntimeConfigSnapshotIsCopy(t *testing.T) {
	rc := NewGatewayRuntimeConfig()
	rc.UpdateRisk(true)

	snap := rc.Snapshot()
	rc.UpdateRisk(false)

	if !snap.RiskEnabled {
		t.Error("snapshot should not change after original update")
	}
}

func TestRuntimeConfigConcurrentReadWrite(t *testing.T) {
	rc := NewGatewayRuntimeConfig()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			rc.UpdateRisk(true)
			rc.UpdateMachineCheck(false)
			rc.UpdateRateLimit(true, RateLimitOptions{MaxQPS: 100})
			rc.UpdateDegrade(DegradeOptions{Enabled: true})
		}()
		go func() {
			defer wg.Done()
			_ = rc.RiskEnabled()
			_ = rc.MachineCheckEnabled()
			_ = rc.RateLimit()
			_ = rc.Degrade()
			_ = rc.Snapshot()
		}()
	}
	wg.Wait()
}

func TestRuntimeConfigNilReceiver(t *testing.T) {
	var rc *GatewayRuntimeConfig

	if rc.RiskEnabled() {
		t.Error("nil RiskEnabled should return false")
	}
	if rc.MachineCheckEnabled() {
		t.Error("nil MachineCheckEnabled should return false")
	}
	rc.UpdateRisk(true)     // should not panic
	rc.UpdateMachineCheck(true) // should not panic
}
