package queue

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"

	"seckill-common/tracing"
)

func TestContextFromNATSMessageUsesHeaderTraceID(t *testing.T) {
	headerTraceID := tracing.NewTraceID()
	bodyTraceID := tracing.NewTraceID()
	msg := &nats.Msg{Header: nats.Header{}}
	msg.Header.Set(tracing.HeaderTraceID, headerTraceID)

	ctx, got := contextFromNATSMessage(context.Background(), msg, bodyTraceID)
	if got != headerTraceID {
		t.Fatalf("traceID = %q, want header trace %q", got, headerTraceID)
	}
	if ctxTraceID := tracing.TraceID(ctx); ctxTraceID != headerTraceID {
		t.Fatalf("context traceID = %q, want %q", ctxTraceID, headerTraceID)
	}
}

func TestContextFromNATSMessageFallsBackToBodyTraceID(t *testing.T) {
	bodyTraceID := tracing.NewTraceID()
	msg := &nats.Msg{Header: nats.Header{}}

	ctx, got := contextFromNATSMessage(context.Background(), msg, bodyTraceID)
	if got != bodyTraceID {
		t.Fatalf("traceID = %q, want body trace %q", got, bodyTraceID)
	}
	if ctxTraceID := tracing.TraceID(ctx); ctxTraceID != bodyTraceID {
		t.Fatalf("context traceID = %q, want %q", ctxTraceID, bodyTraceID)
	}
}

func TestTraceHeadersUseContextTraceID(t *testing.T) {
	ctxTraceID := tracing.NewTraceID()
	fallbackTraceID := tracing.NewTraceID()
	headers := traceHeaders(tracing.WithTraceID(context.Background(), ctxTraceID), fallbackTraceID)

	if got := headers.Get(tracing.HeaderTraceID); got != ctxTraceID {
		t.Fatalf("header traceID = %q, want context trace %q", got, ctxTraceID)
	}
	if got := headers.Get(tracing.HeaderRequestID); got != ctxTraceID {
		t.Fatalf("header requestID = %q, want context trace %q", got, ctxTraceID)
	}
}

func TestTraceHeadersUseFallbackTraceID(t *testing.T) {
	fallbackTraceID := tracing.NewTraceID()
	headers := traceHeaders(context.Background(), fallbackTraceID)

	if got := headers.Get(tracing.HeaderTraceID); got != fallbackTraceID {
		t.Fatalf("header traceID = %q, want fallback trace %q", got, fallbackTraceID)
	}
}
