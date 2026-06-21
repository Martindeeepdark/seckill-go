package gateway

import (
	"errors"
	"testing"
)

func TestCircuitBreakerDisabledAlwaysExecutesFunction(t *testing.T) {
	breaker := NewCircuitBreaker("order-service", CircuitBreakerConfig{Enabled: false}, nil)
	calls := 0
	wantErr := errors.New("downstream failed")

	for i := 0; i < 20; i++ {
		err := breaker.Execute(func() error {
			calls++
			return wantErr
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("Execute() error = %v, want downstream error", err)
		}
	}

	if calls != 20 {
		t.Fatalf("calls = %d, want 20", calls)
	}
}
