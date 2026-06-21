package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware/circuitbreaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsTemporaryRPCError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context deadline", err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded), want: true},
		{name: "grpc resource exhausted", err: fmt.Errorf("wrapped: %w", status.Error(codes.ResourceExhausted, "busy")), want: true},
		{name: "grpc unavailable", err: fmt.Errorf("wrapped: %w", status.Error(codes.Unavailable, "down")), want: true},
		{name: "grpc deadline exceeded", err: fmt.Errorf("wrapped: %w", status.Error(codes.DeadlineExceeded, "slow")), want: true},
		{name: "kratos circuit breaker", err: circuitbreaker.ErrNotAllowed, want: true},
		{name: "kratos too many requests", err: kratoserrors.New(429, "RATE_LIMITED", "busy"), want: true},
		{name: "grpc not found", err: status.Error(codes.NotFound, "missing"), want: false},
		{name: "grpc already exists", err: status.Error(codes.AlreadyExists, "duplicate"), want: false},
		{name: "plain error", err: stderrors.New("plain"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTemporaryRPCError(tt.err); got != tt.want {
				t.Fatalf("IsTemporaryRPCError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
