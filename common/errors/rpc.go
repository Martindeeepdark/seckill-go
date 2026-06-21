package errors

import (
	"context"
	stderrors "errors"
	"net/http"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsTemporaryRPCError 判断是否为临时性 RPC 错误
// 返回 true 表示可以重试，false 表示应该作为最终业务失败处理
func IsTemporaryRPCError(err error) bool {
	if err == nil {
		return false
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	switch kratoserrors.Code(err) {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	switch status.Code(err) {
	case codes.ResourceExhausted, codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

// IsRPCNotFoundError 判断是否为 RPC 层面的未找到错误
// 调用者应该结合业务层的哨兵错误一起使用
func IsRPCNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if kratoserrors.Code(err) == http.StatusNotFound {
		return true
	}
	return status.Code(err) == codes.NotFound
}

// IsRPCFailedPreconditionError 判断是否为前置条件失败的错误
// 通常表示无效状态
func IsRPCFailedPreconditionError(err error) bool {
	if err == nil {
		return false
	}
	if kratoserrors.Code(err) == http.StatusPreconditionFailed {
		return true
	}
	return status.Code(err) == codes.FailedPrecondition
}

// TerminalError 包装错误以表示不应重试
// 队列消费者应该 ACK 终止错误而不是 NAK
type TerminalError struct {
	Err error
}

func (e *TerminalError) Error() string { return e.Err.Error() }
func (e *TerminalError) Unwrap() error { return e.Err }

// WrapTerminal 标记错误为终止错误（不可重试）
func WrapTerminal(err error) error {
	if err == nil {
		return nil
	}
	return &TerminalError{Err: err}
}

// IsTerminal 判断错误是否为终止错误
func IsTerminal(err error) bool {
	return stderrors.As(err, new(*TerminalError))
}
