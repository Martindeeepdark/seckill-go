// Package rpc 提供gRPC服务的实现
package rpc

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"seckill-stock-service/internal/application"
	"seckill-stock-service/internal/infrastructure/persistence"
)

// toStatusError 将领域错误转换为gRPC状态错误
func toStatusError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, application.ErrCommandValidation), errors.Is(err, application.ErrStockInsufficient):
		// 业务错误不需要转换为 gRPC status，直接返回原始错误
		return err
	case errors.Is(err, persistence.ErrNotFound):
		return fmt.Errorf("to gRPC status: %w", status.Error(codes.NotFound, err.Error()))
	case errors.Is(err, persistence.ErrStockNotReady):
		return fmt.Errorf("to gRPC status: %w", status.Error(codes.FailedPrecondition, err.Error()))
	default:
		return err
	}
}
