// Package rpc 提供 gRPC 错误转换工具
package rpc

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"seckill-order-service/internal/infrastructure/persistence"
)

// toStatusError 将领域错误转换为 gRPC 状态错误
// err: 领域错误
// 返回 gRPC 状态错误
func toStatusError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, persistence.ErrNotFound):
		// 未找到错误转换为 NotFound 状态码
		return fmt.Errorf("to gRPC status: %w", status.Error(codes.NotFound, err.Error()))
	case errors.Is(err, persistence.ErrDuplicate):
		// 重复错误转换为 AlreadyExists 状态码
		return fmt.Errorf("to gRPC status: %w", status.Error(codes.AlreadyExists, err.Error()))
	case errors.Is(err, persistence.ErrInvalidState):
		// 无效状态错误转换为 FailedPrecondition 状态码
		return fmt.Errorf("to gRPC status: %w", status.Error(codes.FailedPrecondition, err.Error()))
	default:
		// 其他错误直接返回
		return err
	}
}
