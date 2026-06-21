package application

import (
	"errors"
	"time"
)

var (
	ErrInvalidCommand = errors.New("invalid command")
)

// CreateOrderCommand 创建订单命令，携带订单号、用户、活动、SKU、数量、支付金额和 trace 信息。
type CreateOrderCommand struct {
	OrderNo        string
	UserID         int64
	ActivityNo     string
	SKUNo          string
	Quantity       int64
	PayAmount      int64
	TraceID        string
	RequestTraceID string
}

// Validate 校验创建订单命令参数，要求编号、用户、活动、SKU、数量和金额均合法。
func (c CreateOrderCommand) Validate() error {
	if c.OrderNo == "" {
		return errors.New("order no is required")
	}
	if c.UserID <= 0 {
		return errors.New("user id must be positive")
	}
	if c.ActivityNo == "" {
		return errors.New("activity no is required")
	}
	if c.SKUNo == "" {
		return errors.New("sku no is required")
	}
	if c.Quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	if c.PayAmount <= 0 {
		return errors.New("pay amount must be positive")
	}
	return nil
}

// PayOrderCommand 支付订单命令，携带订单号、交易流水号、金额和支付时间。
type PayOrderCommand struct {
	OrderNo       string
	TransactionNo string
	Amount        int64
	PaidAt        time.Time
}

// Validate 校验支付订单命令参数，要求订单号、流水号非空且金额为正。
func (c PayOrderCommand) Validate() error {
	if c.OrderNo == "" {
		return errors.New("order no is required")
	}
	if c.TransactionNo == "" {
		return errors.New("transaction no is required")
	}
	if c.Amount <= 0 {
		return errors.New("amount must be positive")
	}
	return nil
}

// CloseOrderCommand 关闭订单命令，携带订单号、原因和关闭时间。
type CloseOrderCommand struct {
	OrderNo  string
	Reason   string
	ClosedAt time.Time
}

// Validate 校验关闭订单命令参数，要求订单号和原因非空。
func (c CloseOrderCommand) Validate() error {
	if c.OrderNo == "" {
		return errors.New("order no is required")
	}
	if c.Reason == "" {
		return errors.New("reason is required")
	}
	return nil
}
