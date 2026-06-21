package status

// OrderStatus 订单状态类型
type OrderStatus = string

const (
	OrderPending OrderStatus = "PENDING_PAY" // 订单待支付
	OrderPaid    OrderStatus = "PAID"        // 订单已支付
	OrderClosed  OrderStatus = "CLOSED"      // 订单已关闭
)

// OrderAllowedTransitions 订单允许的状态转换
var OrderAllowedTransitions = map[OrderStatus][]OrderStatus{
	OrderPending: {OrderPaid, OrderClosed},
	OrderPaid:    {},
	OrderClosed:  {},
}

// CanOrderTransitionTo 检查订单是否可以从 from 状态转换到 to 状态
func CanOrderTransitionTo(from, to OrderStatus) bool {
	targets, ok := OrderAllowedTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}
