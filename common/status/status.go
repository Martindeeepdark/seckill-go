// Package status 定义订单状态常量和状态转换规则
package status

import "errors"

// 订单状态常量
const (
	OrderPending  = "PENDING_PAY" // 待支付
	OrderPaid     = "PAID"        // 已支付
	OrderClosed   = "CLOSED"      // 已关闭
	OrderCanceled = "CANCELED"    // 已取消
)

// 支付状态常量
const (
	PayStatusUnpaid = "UNPAID" // 未支付
	PayStatusPaid   = "PAID"   // 已支付
	PayStatusClosed = "CLOSED" // 已关闭
)

// ErrInvalidStateTransition 非法状态转换错误
var ErrInvalidStateTransition = errors.New("invalid state transition")

// TransitionRules 定义合法的状态转换规则
// 键为当前状态，值为允许转换到的目标状态集合
//
// 状态转换图：
//
//	PENDING_PAY → PAID      (支付成功)
//	PENDING_PAY → CLOSED    (支付超时)
//	PENDING_PAY → CANCELED  (用户取消)
//	PAID        → (终态)
//	CLOSED      → (终态)
//	CANCELED    → (终态)
var TransitionRules = map[string][]string{
	OrderPending:  {OrderPaid, OrderClosed, OrderCanceled},
	OrderPaid:     {},
	OrderClosed:   {},
	OrderCanceled: {},
}

// CanTransitTo 检查状态转换是否合法
//
// 参数：
//   - from: 当前状态
//   - to: 目标状态
//
// 返回值：
//   - bool: true 表示可以转换，false 表示不允许转换
//
// 示例：
//
//	CanTransitTo(OrderPending, OrderPaid)     // true
//	CanTransitTo(OrderPending, OrderClosed)   // true
//	CanTransitTo(OrderPaid, OrderClosed)      // false (已支付不能关闭)
//	CanTransitTo(OrderClosed, OrderPaid)      // false (已关闭不能支付)
func CanTransitTo(from, to string) bool {
	allowedStates, exists := TransitionRules[from]
	if !exists {
		return false
	}
	for _, allowed := range allowedStates {
		if allowed == to {
			return true
		}
	}
	return false
}

// ValidateTransition 验证状态转换，不合法时返回错误
//
// 参数：
//   - from: 当前状态
//   - to: 目标状态
//
// 返回值：
//   - error: 转换合法返回 nil，否则返回 ErrInvalidStateTransition
func ValidateTransition(from, to string) error {
	if !CanTransitTo(from, to) {
		return ErrInvalidStateTransition
	}
	return nil
}

// IsTerminalState 判断是否为终态
// 终态不能再转换到其他状态
func IsTerminalState(state string) bool {
	allowedStates, exists := TransitionRules[state]
	return exists && len(allowedStates) == 0
}
