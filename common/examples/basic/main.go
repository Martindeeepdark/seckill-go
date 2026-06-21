package main

import (
	"context"
	"fmt"
	"seckill-common/domain"
	"seckill-common/eventbus/local"
)

// 示例事件
type OrderCreatedEvent struct {
	domain.BaseEvent
	OrderNo string
	UserID  int64
}

func (e *OrderCreatedEvent) EventName() string {
	return "order.created"
}

// 示例聚合根
type Order struct {
	domain.AggregateRoot
	OrderNo string
	UserID  int64
	Status  string
}

func (o *Order) Create(orderNo string, userID int64) {
	o.OrderNo = orderNo
	o.UserID = userID
	o.Status = "PENDING"

	// 记录事件
	o.RecordEvent(&OrderCreatedEvent{
		BaseEvent: domain.NewBaseEvent(orderNo),
		OrderNo:   orderNo,
		UserID:    userID,
	})
}

func main() {
	// 创建事件总线
	bus := local.NewLocalBus()

	// 订阅事件
	err := bus.Subscribe("order.created", func(event domain.DomainEvent) error {
		orderEvent, ok := event.(*OrderCreatedEvent)
		if !ok {
			return fmt.Errorf("invalid event type")
		}

		fmt.Printf("Order created: %s for user %d\n", orderEvent.OrderNo, orderEvent.UserID)
		return nil
	})
	if err != nil {
		panic(err)
	}

	// 创建订单（聚合根）
	order := &Order{}
	order.Create("ORD-001", 12345)

	// 发布事件
	events := order.GetUncommittedEvents()
	for _, event := range events {
		if err := bus.Publish(context.Background(), event); err != nil {
			panic(err)
		}
	}
	order.ClearEvents()

	fmt.Println("Example completed successfully!")
}
