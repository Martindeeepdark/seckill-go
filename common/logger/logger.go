// Package logger 提供结构化日志功能
package logger

import (
	"log/slog"
	"os"

	commonlogs "github.com/Martindeeepdark/go-common/logs"
	"go.uber.org/zap/zapcore"

	"seckill-common/config"
)

// Init 初始化全局日志记录器
// 使用 JSON 格式输出到标准输出
func Init(cfg config.Config) error {
	level := toCommonLogLevel(cfg.LogLevel)

	// JSON encoder 配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	return commonlogs.Init(level,
		commonlogs.WithEncoder(zapcore.NewJSONEncoder(encoderConfig)),
		commonlogs.WithOutput(zapcore.AddSync(os.Stdout)))
}

// GetSlogLogger 返回一个 *slog.Logger 用于向后兼容
// 基础设施代码（tracing, discovery, queue）仍然需要 *slog.Logger 参数
func GetSlogLogger() *slog.Logger {
	return commonlogs.GetSlogLogger()
}

// Sync 刷新日志缓冲区
// 应该在程序退出前调用
func Sync() error {
	return commonlogs.Sync()
}

// toCommonLogLevel 将 slog.Level 转换为 commonlogs.Level
func toCommonLogLevel(level any) commonlogs.Level {
	// 兼容 slog.Level 和其他类型
	switch v := level.(type) {
	case int:
		if v <= -4 {
			return commonlogs.LevelDebug
		}
		if v >= 8 {
			return commonlogs.LevelError
		}
		if v >= 4 {
			return commonlogs.LevelWarn
		}
		return commonlogs.LevelInfo
	default:
		return commonlogs.LevelInfo
	}
}
