package application

import (
	"errors"
	"time"
)

// StartActivityCommand 开始活动命令
type StartActivityCommand struct {
	ActivityNo string
	StartedAt  time.Time
}

// Validate 校验开始活动命令参数，要求活动编号非空。
func (c StartActivityCommand) Validate() error {
	if c.ActivityNo == "" {
		return errors.New("activity no is required")
	}
	return nil
}

// EndActivityCommand 结束活动命令
type EndActivityCommand struct {
	ActivityNo string
	Reason     string
	EndedAt    time.Time
}

// Validate 校验结束活动命令参数，要求活动编号和结束原因非空。
func (c EndActivityCommand) Validate() error {
	if c.ActivityNo == "" {
		return errors.New("activity no is required")
	}
	if c.Reason == "" {
		return errors.New("reason is required")
	}
	return nil
}

// AddSKUCommand 添加商品命令
type AddSKUCommand struct {
	ActivityNo string
	SKUNo      string
	Stock      int64
	Price      int64
}

// Validate 校验添加 SKU 命令参数，要求编号、库存和价格合法。
func (c AddSKUCommand) Validate() error {
	if c.ActivityNo == "" {
		return errors.New("activity no is required")
	}
	if c.SKUNo == "" {
		return errors.New("sku no is required")
	}
	if c.Stock <= 0 {
		return errors.New("stock must be positive")
	}
	if c.Price <= 0 {
		return errors.New("price must be positive")
	}
	return nil
}

// RemoveSKUCommand 移除商品命令
type RemoveSKUCommand struct {
	ActivityNo string
	SKUNo      string
	Reason     string
}

// Validate 校验移除 SKU 命令参数，要求活动编号、SKU 编号和原因非空。
func (c RemoveSKUCommand) Validate() error {
	if c.ActivityNo == "" {
		return errors.New("activity no is required")
	}
	if c.SKUNo == "" {
		return errors.New("sku no is required")
	}
	if c.Reason == "" {
		return errors.New("reason is required")
	}
	return nil
}
