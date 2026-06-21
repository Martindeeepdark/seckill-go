// Package vo 定义活动领域的值对象，封装业务规则和数据验证。
package vo

import "strings"

// ActivityName 表示非空的秒杀活动名称。
type ActivityName string

// PurchaseLimit 表示活动维度的用户限购数量。
type PurchaseLimit int64

// NewActivityName 去除首尾空白并校验活动名称不能为空。
func NewActivityName(value string) (ActivityName, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return ActivityName(value), true
}

// String 返回基础活动名称，供存储和传输层使用。
func (v ActivityName) String() string {
	return string(v)
}

// NewPurchaseLimit 校验限购数量必须大于零。
func NewPurchaseLimit(value int64) (PurchaseLimit, bool) {
	if value <= 0 {
		return 0, false
	}
	return PurchaseLimit(value), true
}

// DefaultPurchaseLimit 返回请求未指定限购数量时的领域默认值。
func DefaultPurchaseLimit() PurchaseLimit {
	return PurchaseLimit(1)
}

// Int 返回基础限购数量，供存储和传输层使用。
func (v PurchaseLimit) Int() int64 {
	return int64(v)
}
