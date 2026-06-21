// Package errors 提供通用错误定义和错误处理工具
package errors

import "errors"

var (
	// ErrNotFound 资源未找到错误
	ErrNotFound = errors.New("not found")
	// ErrDuplicate 重复错误
	ErrDuplicate = errors.New("duplicate")
	// ErrStockNotReady 库存未就绪错误
	ErrStockNotReady = errors.New("stock not ready")
	// ErrInvalidState 无效状态错误
	ErrInvalidState = errors.New("invalid state")
	// ErrCircuitOpen 熔断器打开错误，调用方应据此进行降级处理
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// TraceProcessing 追踪结果的处理中状态
const TraceProcessing = "PROCESSING"
