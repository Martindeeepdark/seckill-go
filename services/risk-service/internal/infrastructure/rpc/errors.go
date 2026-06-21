// Package rpc 提供 gRPC 服务相关实现
package rpc

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"seckill-risk-service/internal/infrastructure/persistence"
)

// toStatusError 将错误转换为 gRPC 状态错误
// 将领域层错误映射为合适的 gRPC 状态码（如 NotFound）
// 参数：err-原始错误
// 返回：gRPC 状态错误
func toStatusError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, persistence.ErrNotFound):
		return fmt.Errorf("not found: %w", status.Error(codes.NotFound, err.Error()))
	default:
		return err
	}
}
