package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"seckill-common/domain"
	"seckill-common/eventbus"
	"seckill-stock-service/internal/domain/event"
)

var (
	ErrDuplicate = errors.New("duplicate entry")
)

// DB 数据库接口
type DB interface {
	InsertStockDeduction(ctx context.Context, deduction StockDeduction) error
	InsertStockRelease(ctx context.Context, release StockRelease) error
}

// StockDeduction 库存扣减记录
type StockDeduction struct {
	ActivityNo string
	SKUNo      string
	OrderNo    string
	UserID     int64
	Quantity   int64
	CreatedAt  time.Time
}

// StockRelease 库存释放记录
type StockRelease struct {
	ActivityNo string
	SKUNo      string
	OrderNo    string
	UserID     int64
	Quantity   int64
	CreatedAt  time.Time
}

// AsyncDBWriter 异步数据库写入器
type AsyncDBWriter struct {
	db     DB
	bus    eventbus.Bus
	logger *zap.Logger
}

// NewAsyncDBWriter 创建异步写入器
func NewAsyncDBWriter(db DB, bus eventbus.Bus, logger *zap.Logger) *AsyncDBWriter {
	return &AsyncDBWriter{
		db:     db,
		bus:    bus,
		logger: logger,
	}
}

// Start 启动消费者
func (w *AsyncDBWriter) Start(ctx context.Context) error {
	// 订阅 stock.reserved 事件
	if err := w.bus.Subscribe("stock.reserved", w.handleReserved); err != nil {
		return fmt.Errorf("subscribe deduction event: %w", err)
	}

	// 订阅 stock.released 事件
	if err := w.bus.Subscribe("stock.released", w.handleReleased); err != nil {
		return fmt.Errorf("subscribe release event: %w", err)
	}

	return nil
}

// handleReserved 处理库存扣减事件
func (w *AsyncDBWriter) handleReserved(evt domain.Event) error {
	domainEvt, ok := evt.(*event.StockReservedEvent)
	if !ok {
		return nil
	}

	ctx := context.Background()
	deduction := StockDeduction{
		ActivityNo: domainEvt.ActivityNo,
		SKUNo:      domainEvt.SKUNo,
		OrderNo:    domainEvt.OrderNo,
		UserID:     domainEvt.UserID,
		Quantity:   domainEvt.Quantity,
		CreatedAt:  domainEvt.OccurredAt(),
	}

	err := w.db.InsertStockDeduction(ctx, deduction)
	if errors.Is(err, ErrDuplicate) {
		// 幂等命中，忽略
		w.logger.Info("idempotent hit", zap.String("orderNo", domainEvt.OrderNo))
		return nil
	}
	if err != nil {
		return fmt.Errorf("insert stock deduction: %w", err)
	}
	return nil
}

// handleReleased 处理库存释放事件
func (w *AsyncDBWriter) handleReleased(evt domain.Event) error {
	domainEvt, ok := evt.(*event.StockReleasedEvent)
	if !ok {
		return nil
	}

	ctx := context.Background()
	release := StockRelease{
		ActivityNo: domainEvt.ActivityNo,
		SKUNo:      domainEvt.SKUNo,
		OrderNo:    domainEvt.OrderNo,
		UserID:     domainEvt.UserID,
		Quantity:   domainEvt.Quantity,
		CreatedAt:  domainEvt.OccurredAt(),
	}

	err := w.db.InsertStockRelease(ctx, release)
	if errors.Is(err, ErrDuplicate) {
		w.logger.Info("idempotent hit", zap.String("orderNo", domainEvt.OrderNo))
		return nil
	}
	if err != nil {
		return fmt.Errorf("insert stock release: %w", err)
	}
	return nil
}
