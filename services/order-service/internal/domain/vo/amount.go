// Package vo 提供金额和数量的值对象
package vo

// Money 金额值对象，单位为分
type Money int64

// Quantity 数量值对象
type Quantity int64

// NewMoney 创建金额值对象
// cents: 金额（分）
// 返回金额对象和是否创建成功
func NewMoney(cents int64) (Money, bool) {
	if cents < 0 {
		return 0, false
	}
	return Money(cents), true
}

// Cents 返回金额的分值
func (v Money) Cents() int64 { return int64(v) }

// NewQuantity 创建数量值对象
// value: 数量值
// 返回数量对象和是否创建成功
func NewQuantity(value int64) (Quantity, bool) {
	if value <= 0 {
		return 0, false
	}
	return Quantity(value), true
}

// Int 返回数量的整数值
func (v Quantity) Int() int64 { return int64(v) }
