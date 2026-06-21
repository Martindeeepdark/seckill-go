package domain

import "errors"

var (
	// ErrCircuitOpen 熔断器打开时返回的错误
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrOrderNotFound 订单不存在
	ErrOrderNotFound = errors.New("order not found")

	// ErrOrderNotPayable 订单不可支付
	ErrOrderNotPayable = errors.New("order not payable")

	// ErrForbidden 支付被禁止（非订单所有者）
	ErrForbidden = errors.New("payment forbidden")
)
