package errors

import "testing"

func TestSentinelErrors(t *testing.T) {
	sentinels := []error{ErrNotFound, ErrDuplicate, ErrStockNotReady, ErrInvalidState}
	for _, err := range sentinels {
		if err == nil {
			t.Fatal("sentinel error should not be nil")
		}
	}
}

func TestTraceProcessing(t *testing.T) {
	if TraceProcessing != "PROCESSING" {
		t.Fatalf("TraceProcessing = %q, want PROCESSING", TraceProcessing)
	}
}
