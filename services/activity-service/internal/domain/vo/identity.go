// Package vo 定义活动领域的值对象，封装业务规则和数据验证。
package vo

import "strings"

// ActivityNo 是秒杀活动在领域内的唯一标识。
type ActivityNo string

// SKUNo 是活动商品在领域内的唯一标识。
type SKUNo string

// TraceID 用于关联异步秒杀请求和最终处理结果。
type TraceID string

// NewActivityNo 去除首尾空白并校验活动编号不能为空。
func NewActivityNo(value string) (ActivityNo, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return ActivityNo(value), true
}

// String 返回用于存储、RPC 和 JSON DTO 的基础字符串值。
func (v ActivityNo) String() string {
	return string(v)
}

// NewSKUNo 去除首尾空白并校验 SKU 编号不能为空。
func NewSKUNo(value string) (SKUNo, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return SKUNo(value), true
}

// String 返回用于存储、RPC 和 JSON DTO 的基础字符串值。
func (v SKUNo) String() string {
	return string(v)
}

// NewTraceID 去除首尾空白并校验链路编号不能为空。
func NewTraceID(value string) (TraceID, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return TraceID(value), true
}

// String 返回用于存储、RPC 和 JSON DTO 的基础字符串值。
func (v TraceID) String() string {
	return string(v)
}
