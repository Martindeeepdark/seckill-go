package application

import "errors"

var (
	ErrCommandValidation = errors.New("command validation failed")
)

// ReserveStockCommand 扣减库存命令
type ReserveStockCommand struct {
	ActivityNo    string
	SKUNo         string
	UserID        int64
	Quantity      int64
	PurchaseLimit int64
	OrderNo       string
}

// Validate 验证命令
func (c ReserveStockCommand) Validate() error {
	if c.ActivityNo == "" {
		return errors.New("activity no is required")
	}
	if c.SKUNo == "" {
		return errors.New("sku no is required")
	}
	if c.UserID <= 0 {
		return errors.New("user id must be positive")
	}
	if c.Quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	if c.OrderNo == "" {
		return errors.New("order no is required")
	}
	return nil
}

// ReleaseStockCommand 释放库存命令
type ReleaseStockCommand struct {
	ActivityNo string
	SKUNo      string
	UserID     int64
	Quantity   int64
	OrderNo    string
}

// Validate 验证命令
func (c ReleaseStockCommand) Validate() error {
	if c.ActivityNo == "" {
		return errors.New("activity no is required")
	}
	if c.SKUNo == "" {
		return errors.New("sku no is required")
	}
	if c.Quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	if c.OrderNo == "" {
		return errors.New("order no is required")
	}
	return nil
}
