// Package entity 定义库存领域实体
package entity

import (
	"errors"
	"fmt"
)

// 领域错误
var (
	ErrInvalidQuantity   = errors.New("quantity must be positive")
	ErrEmptyActivityNo   = errors.New("activityNo must not be empty")
	ErrEmptySKUNo        = errors.New("skuNo must not be empty")
	ErrStockInsufficient = errors.New("insufficient stock")
)

// Stock 库存实体，封装单个 SKU 的库存状态
type Stock struct {
	activityNo string
	skuNo      string
	total      int64
	available  int64
}

// NewStock 创建库存实体
func NewStock(activityNo, skuNo string, total, available int64) (Stock, error) {
	if activityNo == "" {
		return Stock{}, ErrEmptyActivityNo
	}
	if skuNo == "" {
		return Stock{}, ErrEmptySKUNo
	}
	if total < 0 || available < 0 {
		return Stock{}, fmt.Errorf("stock values must be non-negative: total=%d available=%d", total, available)
	}
	return Stock{
		activityNo: activityNo,
		skuNo:      skuNo,
		total:      total,
		available:  available,
	}, nil
}

// ActivityNo 返回活动编号。
func (s Stock) ActivityNo() string { return s.activityNo }

// SKUNo 返回 SKU 编号。
func (s Stock) SKUNo() string { return s.skuNo }

// Total 返回总库存。
func (s Stock) Total() int64 { return s.total }

// Available 返回可用库存。
func (s Stock) Available() int64 { return s.available }

// CanReserve 检查是否有足够库存可预留
func (s Stock) CanReserve(quantity int64) bool {
	return quantity > 0 && s.available >= quantity
}

// Reserve 预留库存（扣减），返回新的 Stock 实体
func (s Stock) Reserve(quantity int64) (Stock, error) {
	if quantity <= 0 {
		return s, ErrInvalidQuantity
	}
	if s.available < quantity {
		return s, ErrStockInsufficient
	}
	return Stock{
		activityNo: s.activityNo,
		skuNo:      s.skuNo,
		total:      s.total,
		available:  s.available - quantity,
	}, nil
}

// Release 释放库存（归还），返回新的 Stock 实体
func (s Stock) Release(quantity int64) (Stock, error) {
	if quantity <= 0 {
		return s, ErrInvalidQuantity
	}
	return Stock{
		activityNo: s.activityNo,
		skuNo:      s.skuNo,
		total:      s.total,
		available:  s.available + quantity,
	}, nil
}

// StockReservation 库存预留记录，关联订单号实现幂等
type StockReservation struct {
	activityNo string
	skuNo      string
	orderNo    string
	userID     int64
	quantity   int64
}

// NewStockReservation 创建库存预留记录
func NewStockReservation(activityNo, skuNo, orderNo string, userID, quantity int64) (StockReservation, error) {
	if activityNo == "" {
		return StockReservation{}, ErrEmptyActivityNo
	}
	if skuNo == "" {
		return StockReservation{}, ErrEmptySKUNo
	}
	if quantity <= 0 {
		return StockReservation{}, ErrInvalidQuantity
	}
	return StockReservation{
		activityNo: activityNo,
		skuNo:      skuNo,
		orderNo:    orderNo,
		userID:     userID,
		quantity:   quantity,
	}, nil
}

// ActivityNo 返回活动编号。
func (r StockReservation) ActivityNo() string { return r.activityNo }

// SKUNo 返回 SKU 编号。
func (r StockReservation) SKUNo() string { return r.skuNo }

// OrderNo 返回订单号。
func (r StockReservation) OrderNo() string { return r.orderNo }

// UserID 返回用户 ID。
func (r StockReservation) UserID() int64 { return r.userID }

// Quantity 返回预占数量。
func (r StockReservation) Quantity() int64 { return r.quantity }

// ValidateAgainstPurchaseLimit 检查购买数量是否在限购范围内
func (r StockReservation) ValidateAgainstPurchaseLimit(currentPurchased, limit int64) bool {
	if limit <= 0 {
		return true
	}
	return currentPurchased+r.quantity <= limit
}

// NewStockFromState 从 Redis 状态创建 Stock 值对象
func NewStockFromState(activityNo, skuNo string, total, available int64) (Stock, error) {
	return NewStock(activityNo, skuNo, total, available)
}
