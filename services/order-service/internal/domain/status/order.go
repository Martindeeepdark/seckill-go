// Package status 提供订单状态机逻辑
package status

import (
	"slices"
	"time"

	"seckill-order-service/internal/domain/entity"
)

const (
	OrderPending = "PENDING_PAY" // 订单待支付
	OrderPaid    = "PAID"        // 订单已支付
	OrderClosed  = "CLOSED"      // 订单已关闭
)

// OrderStatusLabel 订单状态中文标签映射
var OrderStatusLabel = map[string]string{
	OrderPending: "待支付",
	OrderPaid:    "已支付",
	OrderClosed:  "已关闭",
}

// OrderAllowedTransitions 订单状态允许的转移映射
var OrderAllowedTransitions = map[string][]string{
	OrderPending: {OrderPaid, OrderClosed}, // 待支付可以转为已支付或已关闭
	OrderPaid:    {},                       // 已支付是终态，不能再转移
	OrderClosed:  {},                       // 已关闭是终态，不能再转移
}

// CanOrderTransitionTo 检查订单是否可以从当前状态转移到目标状态
// from: 当前状态
// to: 目标状态
// 返回是否允许转移
func CanOrderTransitionTo(from string, to string) bool {
	targets, ok := OrderAllowedTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(targets, to)
}

// TransitOrderPaid 将订单转移到已支付状态
// order: 订单实体
// transactionNo: 交易流水号
// paidAt: 支付时间
// 返回转移后的订单和是否成功
func TransitOrderPaid(order entity.Order, transactionNo string, paidAt time.Time) (entity.Order, bool) {
	// 如果已经是已支付状态，直接返回
	if order.Status == OrderPaid {
		return order, true
	}
	// 检查是否允许转移到已支付状态
	if !CanOrderTransitionTo(order.Status, OrderPaid) {
		return order, false
	}
	// 更新订单状态
	order.Status = OrderPaid
	order.TransactionNo = transactionNo
	order.PaidAt = &paidAt
	return order, true
}

// TransitOrderClosed 将订单转移到已关闭状态
// order: 订单实体
// closedAt: 关闭时间
// 返回转移后的订单和是否成功
func TransitOrderClosed(order entity.Order, closedAt time.Time) (entity.Order, bool) {
	// 如果已经是已关闭状态，直接返回
	if order.Status == OrderClosed {
		return order, true
	}
	// 检查是否允许转移到已关闭状态
	if !CanOrderTransitionTo(order.Status, OrderClosed) {
		return order, false
	}
	// 更新订单状态
	order.Status = OrderClosed
	order.ClosedAt = &closedAt
	return order, true
}
