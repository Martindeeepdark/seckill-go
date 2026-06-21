// Package vo 定义活动领域的值对象，封装业务规则和数据验证。
package vo

// Money 使用分作为金额单位，避免浮点数表示价格带来的精度问题。
type Money int64

// Quantity 表示一次购买的正整数数量。
type Quantity int64

// NewMoney 校验金额不能为负数。
func NewMoney(cents int64) (Money, bool) {
	if cents < 0 {
		return 0, false
	}
	return Money(cents), true
}

// Cents 返回以分为单位的基础金额值，供存储和传输层使用。
func (v Money) Cents() int64 {
	return int64(v)
}

// NewQuantity 校验购买数量必须大于零。
func NewQuantity(value int64) (Quantity, bool) {
	if value <= 0 {
		return 0, false
	}
	return Quantity(value), true
}

// Int 返回基础数量值，供存储和传输层使用。
func (v Quantity) Int() int64 {
	return int64(v)
}
