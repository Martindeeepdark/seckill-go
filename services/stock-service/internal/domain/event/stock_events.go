package event

import "seckill-common/domain"

// StockReservedEvent 库存扣减成功事件
type StockReservedEvent struct {
	domain.BaseEvent
	ActivityNo string `json:"activityNo"`
	SKUNo      string `json:"skuNo"`
	Quantity   int64  `json:"quantity"`
	UserID     int64  `json:"userId"`
	OrderNo    string `json:"orderNo"`
}

// NewStockReservedEvent 创建库存扣减事件
func NewStockReservedEvent(activityNo, skuNo string, quantity, userID int64, orderNo string) *StockReservedEvent {
	return &StockReservedEvent{
		BaseEvent:  domain.NewBaseEvent(activityNo),
		ActivityNo: activityNo,
		SKUNo:      skuNo,
		Quantity:   quantity,
		UserID:     userID,
		OrderNo:    orderNo,
	}
}

// EventName 返回事件名称
func (e *StockReservedEvent) EventName() string {
	return "stock.reserved"
}

// StockReleasedEvent 库存释放事件
type StockReleasedEvent struct {
	domain.BaseEvent
	ActivityNo string `json:"activityNo"`
	SKUNo      string `json:"skuNo"`
	Quantity   int64  `json:"quantity"`
	UserID     int64  `json:"userId"`
	OrderNo    string `json:"orderNo"`
}

// NewStockReleasedEvent 创建库存释放事件
func NewStockReleasedEvent(activityNo, skuNo string, quantity, userID int64, orderNo string) *StockReleasedEvent {
	return &StockReleasedEvent{
		BaseEvent:  domain.NewBaseEvent(activityNo),
		ActivityNo: activityNo,
		SKUNo:      skuNo,
		Quantity:   quantity,
		UserID:     userID,
		OrderNo:    orderNo,
	}
}

// EventName 返回事件名称
func (e *StockReleasedEvent) EventName() string {
	return "stock.released"
}
