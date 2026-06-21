// Package persistence 提供风险控制的持久化层实现
package persistence

import "errors"

// ErrNotFound 数据未找到错误
var ErrNotFound = errors.New("not found")
