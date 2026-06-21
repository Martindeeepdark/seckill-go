package entity

import (
	"seckill-common/domain"
	"seckill-stock-service/internal/domain/event"
)

// StockAggregate 库存聚合根
type StockAggregate struct {
	*domain.AggregateRoot
	stock Stock
}

// NewStockAggregate 创建记录事件的聚合根
func NewStockAggregate(stock Stock) *StockAggregate {
	agg := &StockAggregate{stock: stock}
	agg.AggregateRoot = domain.NewAggregateRoot()
	return agg
}

// RecordReserved 记录扣减成功事件
func (a *StockAggregate) RecordReserved(quantity, userID int64, orderNo string) {
	a.RecordEvent(event.NewStockReservedEvent(
		a.stock.ActivityNo(), a.stock.SKUNo(),
		quantity, userID, orderNo,
	))
}

// RecordReleased 记录释放成功事件
func (a *StockAggregate) RecordReleased(quantity, userID int64, orderNo string) {
	a.RecordEvent(event.NewStockReleasedEvent(
		a.stock.ActivityNo(), a.stock.SKUNo(),
		quantity, userID, orderNo,
	))
}

// GetStock 获取当前库存状态
func (a *StockAggregate) GetStock() Stock {
	return a.stock
}
