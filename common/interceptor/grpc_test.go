package interceptor

import (
	"context"
	"testing"

	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"seckill-common/tracing"
)

func TestTraceUnaryClientInterceptorPropagatesTraceMetadata(t *testing.T) {
	traceID := "11111111111111111111111111111111"
	ctx := tracing.WithTraceID(context.Background(), traceID)

	var captured metadata.MD
	err := TraceUnaryClientInterceptor()(ctx, "/test.Service/Method", nil, nil, nil, func(ctx context.Context, _ string, _ any, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			t.Fatal("missing outgoing metadata")
		}
		captured = md
		return nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if got := firstMetadataValue(captured, tracing.MetadataTraceID); got != traceID {
		t.Fatalf("trace metadata = %q, want %q", got, traceID)
	}
	if got := firstMetadataValue(captured, tracing.MetadataRequestID); got != traceID {
		t.Fatalf("request metadata = %q, want %q", got, traceID)
	}
	if got := firstMetadataValue(captured, tracing.HeaderTraceParent); got == "" {
		t.Fatal("traceparent metadata is empty")
	}
}

func TestTraceUnaryServerInterceptorRestoresTraceFromTraceParent(t *testing.T) {
	traceID := "22222222222222222222222222222222"
	traceparent := "00-" + traceID + "-3333333333333333-01"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(tracing.HeaderTraceParent, traceparent))

	_, err := TraceUnaryServerInterceptor()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, func(ctx context.Context, _ any) (any, error) {
		if got := tracing.TraceID(ctx); got != traceID {
			t.Fatalf("trace ID = %q, want %q", got, traceID)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}

func TestContextFromIncomingMetadataRestoresTraceParentSpanContext(t *testing.T) {
	traceID := "44444444444444444444444444444444"
	spanID := "5555555555555555"
	traceparent := "00-" + traceID + "-" + spanID + "-01"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(tracing.HeaderTraceParent, traceparent))

	ctx, got := contextFromIncomingMetadata(ctx)
	if got != traceID {
		t.Fatalf("trace ID = %q, want %q", got, traceID)
	}
	spanContext := oteltrace.SpanContextFromContext(ctx)
	if !spanContext.IsRemote() {
		t.Fatal("span context should be remote")
	}
	if spanContext.SpanID().String() != spanID {
		t.Fatalf("parent spanID = %q, want %q", spanContext.SpanID().String(), spanID)
	}
}

func firstMetadataValue(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
