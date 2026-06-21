package event

import (
	"seckill-common/domain"
	"time"
)

// OrderCreatedEvent 表示订单创建的领域事件。
type OrderCreatedEvent struct {
	domain.BaseEvent
	OrderNo    string    `json:"orderNo"`
	UserID     int64     `json:"userId"`
	ActivityNo string    `json:"activityNo"`
	SKUNo      string    `json:"skuNo"`
	Quantity   int64     `json:"quantity"`
	PayAmount  int64     `json:"payAmount"`
	CreatedAt  time.Time `json:"createdAt"`
}

// NewOrderCreatedEvent 构造订单创建事件，以订单号作为聚合标识。
func NewOrderCreatedEvent(orderNo string, userID int64, activityNo, skuNo string, quantity, payAmount int64) *OrderCreatedEvent {
	return &OrderCreatedEvent{
		BaseEvent:  domain.NewBaseEvent(orderNo),
		OrderNo:    orderNo,
		UserID:     userID,
		ActivityNo: activityNo,
		SKUNo:      skuNo,
		Quantity:   quantity,
		PayAmount:  payAmount,
		CreatedAt:  time.Now(),
	}
}

// EventName 返回订单创建事件名称。
func (e *OrderCreatedEvent) EventName() string {
	return "order.created"
}

// OrderPaidEvent 表示订单支付的领域事件。
type OrderPaidEvent struct {
	domain.BaseEvent
	OrderNo       string    `json:"orderNo"`
	UserID        int64     `json:"userId"`
	TransactionNo string    `json:"transactionNo"`
	Amount        int64     `json:"amount"`
	PaidAt        time.Time `json:"paidAt"`
}

// NewOrderPaidEvent 构造订单支付事件。
func NewOrderPaidEvent(orderNo string, userID int64, transactionNo string, amount int64, paidAt time.Time) *OrderPaidEvent {
	return &OrderPaidEvent{
		BaseEvent:     domain.NewBaseEvent(orderNo),
		OrderNo:       orderNo,
		UserID:        userID,
		TransactionNo: transactionNo,
		Amount:        amount,
		PaidAt:        paidAt,
	}
}

// EventName 返回订单支付事件名称。
func (e *OrderPaidEvent) EventName() string {
	return "order.paid"
}

// OrderClosedEvent 表示订单关闭的领域事件。
type OrderClosedEvent struct {
	domain.BaseEvent
	OrderNo  string    `json:"orderNo"`
	UserID   int64     `json:"userId"`
	Reason   string    `json:"reason"`
	ClosedAt time.Time `json:"closedAt"`
}

// NewOrderClosedEvent 构造订单关闭事件。
func NewOrderClosedEvent(orderNo string, userID int64, reason string, closedAt time.Time) *OrderClosedEvent {
	return &OrderClosedEvent{
		BaseEvent: domain.NewBaseEvent(orderNo),
		OrderNo:   orderNo,
		UserID:    userID,
		Reason:    reason,
		ClosedAt:  closedAt,
	}
}

// EventName 返回订单关闭事件名称。
func (e *OrderClosedEvent) EventName() string {
	return "order.closed"
}
