// Package vo 提供身份标识的值对象
package vo

import "strings"

// ActivityNo 活动编号值对象
type ActivityNo string

// OrderNo 订单号值对象
type OrderNo string

// SKUNo SKU编号值对象
type SKUNo string

// TraceID 追踪ID值对象
type TraceID string

// RequestTraceID 请求追踪ID值对象
type RequestTraceID string

// NewActivityNo 创建活动编号值对象
// value: 原始字符串
// 返回活动编号对象和是否创建成功
func NewActivityNo(value string) (ActivityNo, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return ActivityNo(value), true
}

// String 返回活动编号的字符串表示
func (v ActivityNo) String() string { return string(v) }

// NewOrderNo 创建订单号值对象
// value: 原始字符串
// 返回订单号对象和是否创建成功
func NewOrderNo(value string) (OrderNo, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return OrderNo(value), true
}

// String 返回订单号的字符串表示
func (v OrderNo) String() string { return string(v) }

// NewSKUNo 创建SKU编号值对象
// value: 原始字符串
// 返回SKU编号对象和是否创建成功
func NewSKUNo(value string) (SKUNo, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return SKUNo(value), true
}

// String 返回SKU编号的字符串表示
func (v SKUNo) String() string { return string(v) }

// NewTraceID 创建追踪ID值对象
// value: 原始字符串
// 返回追踪ID对象和是否创建成功
func NewTraceID(value string) (TraceID, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return TraceID(value), true
}

// String 返回追踪ID的字符串表示
func (v TraceID) String() string { return string(v) }

// NewRequestTraceID 创建请求追踪ID值对象
// value: 原始字符串
// 返回请求追踪ID对象和是否创建成功
func NewRequestTraceID(value string) (RequestTraceID, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return RequestTraceID(value), true
}

// String 返回请求追踪ID的字符串表示
func (v RequestTraceID) String() string { return string(v) }
