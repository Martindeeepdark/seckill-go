package tracing

import (
	"context"
	"log/slog"

	commontrace "github.com/Martindeeepdark/go-common/trace/otel"

	"seckill-common/config"
)

// InitTracing 初始化 OpenTelemetry 链路追踪
// 返回清理函数，用于程序退出时关闭追踪
func InitTracing(cfg config.Config, logger *slog.Logger) func(context.Context) error {
	if !cfg.TraceEnabled {
		return func(context.Context) error { return nil }
	}
	opts := []commontrace.Option{
		commontrace.WithServiceName(cfg.ServiceName),
	}
	if cfg.TraceEndpoint != "" {
		opts = append(opts, commontrace.WithEndpoint(cfg.TraceEndpoint))
	}
	if cfg.TraceInsecure {
		opts = append(opts, commontrace.WithInsecure())
	}
	shutdown, err := commontrace.Init(opts...)
	if err != nil {
		if logger != nil {
			logger.Warn("trace init failed", "error", err)
		}
		return func(context.Context) error { return nil }
	}
	if logger != nil {
		logger.Info("trace initialized", "endpoint", cfg.TraceEndpoint, "service", cfg.ServiceName)
	}
	return shutdown
}
