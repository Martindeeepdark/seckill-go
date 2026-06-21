// Package interceptor 提供 gRPC 拦截器
package interceptor

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"seckill-common/tracing"
)

// TraceUnaryServerInterceptor 服务端链路追踪拦截器
// 从请求元数据中提取 traceID，创建 span 并传播到下游
func TraceUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, traceID := contextFromIncomingMetadata(ctx)
		ctx, span, _ := tracing.StartSpan(ctx, "gRPC "+info.FullMethod, traceID)
		resp, err := handler(ctx, req)
		tracing.EndSpan(span, err)
		return resp, err
	}
}

// TraceUnaryClientInterceptor 客户端链路追踪拦截器
// 创建 span 并将 traceID 和 traceparent 添加到请求元数据中
func TraceUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, span, traceID := tracing.StartSpan(ctx, "gRPC "+method, "")
		ctx = metadata.AppendToOutgoingContext(
			ctx,
			tracing.MetadataTraceID, traceID,
			tracing.MetadataRequestID, traceID,
		)
		if traceParent := tracing.TraceParent(ctx); traceParent != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, tracing.HeaderTraceParent, traceParent)
		}
		err := invoker(ctx, method, req, reply, cc, opts...)
		tracing.EndSpan(span, err)
		return err
	}
}

// contextFromIncomingMetadata 从入站请求元数据中恢复 trace 上下文。
func contextFromIncomingMetadata(ctx context.Context) (context.Context, string) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return tracing.EnsureTraceID(ctx, "")
	}
	return tracing.ContextFromCarrier(ctx, func(key string) string {
		if values := md.Get(key); len(values) > 0 {
			return values[0]
		}
		return ""
	}, "")
}
