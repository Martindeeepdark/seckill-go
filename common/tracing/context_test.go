package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestContextFromCarrierRestoresTraceParentSpanContext(t *testing.T) {
	traceID := "11111111111111111111111111111111"
	spanID := "2222222222222222"
	ctx, got := ContextFromCarrier(context.Background(), func(key string) string {
		if key == HeaderTraceParent {
			return "00-" + traceID + "-" + spanID + "-01"
		}
		return ""
	}, "")

	if got != traceID {
		t.Fatalf("traceID = %q, want %q", got, traceID)
	}
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsRemote() {
		t.Fatal("span context should be remote")
	}
	if spanContext.TraceID().String() != traceID {
		t.Fatalf("span traceID = %q, want %q", spanContext.TraceID().String(), traceID)
	}
	if spanContext.SpanID().String() != spanID {
		t.Fatalf("parent spanID = %q, want %q", spanContext.SpanID().String(), spanID)
	}
	if spanContext.TraceFlags()&trace.FlagsSampled != trace.FlagsSampled {
		t.Fatalf("trace flags = %v, want sampled", spanContext.TraceFlags())
	}
}

func TestContextFromCarrierPrefersCustomTraceIDWhenTraceParentConflicts(t *testing.T) {
	customTraceID := "33333333333333333333333333333333"
	parentTraceID := "11111111111111111111111111111111"
	ctx, got := ContextFromCarrier(context.Background(), func(key string) string {
		switch key {
		case HeaderTraceID:
			return customTraceID
		case HeaderTraceParent:
			return "00-" + parentTraceID + "-2222222222222222-01"
		default:
			return ""
		}
	}, "")

	if got != customTraceID {
		t.Fatalf("traceID = %q, want custom trace %q", got, customTraceID)
	}
	if ctxTraceID := TraceID(ctx); ctxTraceID != customTraceID {
		t.Fatalf("context traceID = %q, want %q", ctxTraceID, customTraceID)
	}
}

func TestEnsureTraceIDPreservesRemoteParentSpanContext(t *testing.T) {
	traceID := "11111111111111111111111111111111"
	spanID := "2222222222222222"
	ctx, _ := ContextFromCarrier(context.Background(), func(key string) string {
		if key == HeaderTraceParent {
			return "00-" + traceID + "-" + spanID + "-01"
		}
		return ""
	}, "")

	ctx, got := EnsureTraceID(ctx, "")
	if got != traceID {
		t.Fatalf("traceID = %q, want %q", got, traceID)
	}
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.SpanID().String() != spanID {
		t.Fatalf("parent spanID = %q, want preserved %q", spanContext.SpanID().String(), spanID)
	}
}
