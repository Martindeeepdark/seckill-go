package queue

import "testing"

func TestResultKeysUseJavaPrefixWithLegacyFallback(t *testing.T) {
	traceID := "TRACE1"
	if got, want := resultKey(traceID), "seckill:order:result:TRACE1"; got != want {
		t.Fatalf("resultKey() = %q, want %q", got, want)
	}
	if got, want := legacyResultKey(traceID), "seckill:trace:TRACE1"; got != want {
		t.Fatalf("legacyResultKey() = %q, want %q", got, want)
	}
}
