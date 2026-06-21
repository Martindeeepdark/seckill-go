// Package rpc 提供gRPC服务实现
package rpc

import (
	"errors"

	supportapp "seckill-support-service/internal/application"
	"seckill-support-service/internal/domain"
	"seckill-support-service/internal/infrastructure/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toStatusError 将应用层错误转换为gRPC状态错误
//nolint:wrapcheck // gRPC状态错误必须直接返回以保留状态码
func toStatusError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, supportapp.ErrInvalidRequest) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, domain.ErrForbidden) {
		return status.Error(codes.PermissionDenied, err.Error())
	}
	if errors.Is(err, domain.ErrOrderNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	if errors.Is(err, domain.ErrOrderNotPayable) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if errors.Is(err, store.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	if errors.Is(err, store.ErrInvalidState) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	return err
}
