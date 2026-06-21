// Package rpc 提供领域错误到 gRPC 状态码的转换。
package rpc

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"seckill-activity-service/internal/infrastructure/persistence"
)

// toStatusError 将领域错误转换为 gRPC 状态错误。
func toStatusError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, persistence.ErrNotFound):
		return fmt.Errorf("to gRPC status: %w", status.Error(codes.NotFound, err.Error()))
	case errors.Is(err, persistence.ErrDuplicate):
		return fmt.Errorf("to gRPC status: %w", status.Error(codes.AlreadyExists, err.Error()))
	case errors.Is(err, persistence.ErrStockNotReady):
		return fmt.Errorf("to gRPC status: %w", status.Error(codes.FailedPrecondition, err.Error()))
	case errors.Is(err, persistence.ErrInvalidState):
		return fmt.Errorf("to gRPC status: %w", status.Error(codes.FailedPrecondition, err.Error()))
	default:
		return err
	}
}
