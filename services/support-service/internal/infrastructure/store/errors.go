// Package store 提供存储层错误定义
package store

import "errors"

var (
	ErrNotFound     = errors.New("not found")     // 资源未找到
	ErrDuplicate    = errors.New("duplicate")     // 资源重复
	ErrInvalidState = errors.New("invalid state") // 无效状态
)
