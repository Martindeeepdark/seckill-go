// Package persistence 持久化层的错误定义
package persistence

import (
	commonerrors "seckill-common/errors"
	"seckill-order-service/internal/domain/repository"
)

var (
	// ErrNotFound 资源未找到错误
	ErrNotFound = repository.ErrNotFound
	// ErrDuplicate 资源重复错误
	ErrDuplicate = repository.ErrDuplicate
	// ErrInvalidState 无效状态错误
	ErrInvalidState = commonerrors.ErrInvalidState
)
