// Package tracing 提供分布式链路追踪功能
package tracing

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	commonlogs "github.com/Martindeeepdark/go-common/logs"
	commontrace "github.com/Martindeeepdark/go-common/trace/otel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// HeaderTraceID 自定义追踪 ID 头
	HeaderTraceID = "X-Trace-Id"
	// HeaderRequestID 请求 ID 头
	HeaderRequestID = "X-Request-Id"
	// HeaderTraceParent W3C traceparent 头
	HeaderTraceParent = "traceparent"
	// HeaderB3TraceID B3 追踪 ID 头
	HeaderB3TraceID = "X-B3-TraceId"
	// MetadataTraceID gRPC 元数据追踪 ID 键
	MetadataTraceID = "x-trace-id"
	// MetadataRequestID gRPC 元数据请求 ID 键
	MetadataRequestID = "x-request-id"
	// TraceIDKey 上下文中追踪 ID 的键
	TraceIDKey = "traceId"
)

type traceIDKey struct{}

// NewTraceID 生成新的追踪 ID
// 使用加密随机数生成 W3C 标准的 16 字节追踪 ID
func NewTraceID() string {
	var b [16]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		// 随机数生成失败时，使用时间戳回退方案
		sum := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		copy(b[:], sum[:16])
	}
	traceID := trace.TraceID(b)
	if !traceID.IsValid() {
		return NewTraceID()
	}
	return traceID.String()
}

// NormalizeTraceID 规范化追踪 ID
// 转换为小写并验证格式
func NormalizeTraceID(traceID string) string {
	traceID = strings.ToLower(strings.TrimSpace(traceID))
	if traceID == "" {
		return ""
	}
	value, err := trace.TraceIDFromHex(traceID)
	if err != nil || !value.IsValid() {
		return ""
	}
	return value.String()
}

// TraceIDFromCarrier 从载体（如 HTTP 头）中提取追踪 ID
// 支持多种追踪头格式：自定义、W3C traceparent、B3
func TraceIDFromCarrier(get func(string) string) string {
	if get == nil {
		return ""
	}
	// 优先检查自定义头
	for _, key := range []string{
		HeaderTraceID,
		MetadataTraceID,
		TraceIDKey,
		HeaderRequestID,
		MetadataRequestID,
	} {
		if traceID := NormalizeTraceID(get(key)); traceID != "" {
			return traceID
		}
	}
	// 其次检查 W3C traceparent
	if traceID := traceIDFromTraceParent(get(HeaderTraceParent)); traceID != "" {
		return traceID
	}
	// 最后检查 B3
	if traceID := NormalizeTraceID(get(HeaderB3TraceID)); traceID != "" {
		return traceID
	}
	return ""
}

// ContextFromCarrier 从 HTTP header、gRPC metadata 或 MQ header 中恢复 trace 上下文。
// 自定义 traceId 优先；没有自定义 traceId 时尽量保留 W3C traceparent 的远端父 span。
func ContextFromCarrier(ctx context.Context, get func(string) string, fallbackTraceID string) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if get == nil {
		return EnsureTraceID(ctx, fallbackTraceID)
	}
	if traceID := customTraceIDFromCarrier(get); traceID != "" {
		return WithTraceID(ctx, traceID), traceID
	}
	if spanContext := spanContextFromTraceParent(get(HeaderTraceParent)); spanContext.IsValid() {
		traceID := spanContext.TraceID().String()
		ctx = trace.ContextWithRemoteSpanContext(ctx, spanContext)
		ctx = context.WithValue(ctx, traceIDKey{}, traceID)
		return ctx, traceID
	}
	if traceID := NormalizeTraceID(get(HeaderB3TraceID)); traceID != "" {
		return WithTraceID(ctx, traceID), traceID
	}
	return EnsureTraceID(ctx, fallbackTraceID)
}

// WithTraceID 将追踪 ID 添加到上下文中
// 同时设置 OpenTelemetry 的 SpanContext 和标准 logs key
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID = NormalizeTraceID(traceID)
	if traceID == "" {
		return ctx
	}

	// 1. 设置私有 key（用于当前包的 TraceID() 函数）
	ctx = context.WithValue(ctx, traceIDKey{}, traceID)

	// 2. 设置标准 logs key（供 commonlogs 使用）
	ctx = commonlogs.WithTraceID(ctx, traceID)

	// 3. 设置 OpenTelemetry SpanContext
	if traceIDValue, err := trace.TraceIDFromHex(traceID); err == nil {
		spanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceIDValue,
			SpanID:     randomSpanID(),
			TraceFlags: trace.FlagsSampled,
			Remote:     true,
		})
		ctx = trace.ContextWithRemoteSpanContext(ctx, spanContext)
	}

	return ctx
}

// TraceID 从上下文中获取追踪 ID
// 优先从自定义键获取，其次从 OpenTelemetry 获取
func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, ok := ctx.Value(traceIDKey{}).(string); ok {
		if traceID = NormalizeTraceID(traceID); traceID != "" {
			return traceID
		}
	}
	if traceID := NormalizeTraceID(commontrace.TraceID(ctx)); traceID != "" {
		return traceID
	}
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// TraceParent 生成 W3C traceparent 头值
// 格式：00-{traceID}-{spanID}-{flags}
func TraceParent(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	spanContext := trace.SpanFromContext(ctx).SpanContext()
	if !spanContext.HasTraceID() || !spanContext.HasSpanID() {
		return ""
	}
	flags := "00"
	if spanContext.TraceFlags()&trace.FlagsSampled == trace.FlagsSampled {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s", spanContext.TraceID().String(), spanContext.SpanID().String(), flags)
}

// EnsureTraceID 确保上下文中存在追踪 ID
// 如果不存在则生成新的
func EnsureTraceID(ctx context.Context, traceID string) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID = NormalizeTraceID(traceID)
	if traceID == "" {
		if existing := TraceID(ctx); existing != "" {
			if stored, ok := ctx.Value(traceIDKey{}).(string); !ok || NormalizeTraceID(stored) != existing {
				ctx = context.WithValue(ctx, traceIDKey{}, existing)
			}
			return ctx, existing
		}
		traceID = NewTraceID()
	}
	if existing := TraceID(ctx); existing == traceID {
		if stored, ok := ctx.Value(traceIDKey{}).(string); !ok || NormalizeTraceID(stored) != traceID {
			ctx = context.WithValue(ctx, traceIDKey{}, traceID)
		}
		return ctx, traceID
	}
	return WithTraceID(ctx, traceID), traceID
}

// StartSpan 创建新的追踪 span
// 返回带有 span 的上下文、span 实例和追踪 ID
func StartSpan(ctx context.Context, name string, incomingTraceID string) (context.Context, trace.Span, string) {
	ctx, traceID := EnsureTraceID(ctx, incomingTraceID)
	ctx, span := otel.Tracer("seckill").Start(ctx, name)
	if spanTraceID := TraceID(ctx); spanTraceID != "" {
		traceID = spanTraceID
		ctx = context.WithValue(ctx, traceIDKey{}, traceID)
	}
	return ctx, span, traceID
}

// traceIDFromTraceParent 从 W3C traceparent 头中提取追踪 ID
func traceIDFromTraceParent(header string) string {
	spanContext := spanContextFromTraceParent(header)
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.TraceID().String()
}

func customTraceIDFromCarrier(get func(string) string) string {
	for _, key := range []string{
		HeaderTraceID,
		MetadataTraceID,
		TraceIDKey,
		HeaderRequestID,
		MetadataRequestID,
	} {
		if traceID := NormalizeTraceID(get(key)); traceID != "" {
			return traceID
		}
	}
	return ""
}

func spanContextFromTraceParent(header string) trace.SpanContext {
	parts := strings.Split(strings.TrimSpace(header), "-")
	if len(parts) < 4 {
		return trace.SpanContext{}
	}
	traceID, err := trace.TraceIDFromHex(parts[1])
	if err != nil || !traceID.IsValid() {
		return trace.SpanContext{}
	}
	spanID, err := trace.SpanIDFromHex(parts[2])
	if err != nil || !spanID.IsValid() {
		return trace.SpanContext{}
	}
	var flags trace.TraceFlags
	if parsed, err := strconv.ParseUint(parts[3], 16, 8); err == nil && parsed&1 == 1 {
		flags = trace.FlagsSampled
	}
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: flags,
		Remote:     true,
	})
}

// EndSpan 结束 span
// 如果有错误，记录错误并设置状态
func EndSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// randomSpanID 生成随机的 span ID
func randomSpanID() trace.SpanID {
	var b [8]byte
	_, _ = cryptorand.Read(b[:]) // crypto/rand.Read never fails on supported platforms
	spanID, err := trace.SpanIDFromHex(hex.EncodeToString(b[:]))
	if err != nil {
		return trace.SpanID{}
	}
	return spanID
}
