package application

import (
	"context"
	"fmt"

	"seckill-common/eventbus"
	"seckill-stock-service/internal/domain/entity"
	"seckill-stock-service/internal/domain/repository"
)

// ErrStockInsufficient 库存不足错误（从 domain entity 复用）。
var ErrStockInsufficient = entity.ErrStockInsufficient

// StockAppService 库存应用服务，负责编排业务流程
type StockAppService struct {
	repo repository.StockRepository
	bus  eventbus.Bus
}

// NewStockAppService 创建库存应用服务
func NewStockAppService(repo repository.StockRepository, bus eventbus.Bus) *StockAppService {
	return &StockAppService{
		repo: repo,
		bus:  bus,
	}
}

// ReserveStock 扣减库存
func (s *StockAppService) ReserveStock(ctx context.Context, cmd ReserveStockCommand) error {
	// 1. 验证命令
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrCommandValidation, err)
	}

	// 2. 调用 Repository（Redis Lua 原子扣减）
	ok, err := s.repo.DeductStockWithLimit(
		ctx, cmd.ActivityNo, cmd.SKUNo,
		cmd.UserID, cmd.Quantity, cmd.PurchaseLimit, cmd.OrderNo,
	)
	if err != nil {
		return fmt.Errorf("deduct stock failed: %w", err)
	}

	// 3. 扣减失败不发布事件
	if !ok {
		return ErrStockInsufficient
	}

	// 4. 从 Redis 加载最新状态
	available, err := s.repo.PeekStock(ctx, cmd.ActivityNo, cmd.SKUNo)
	if err != nil {
		return fmt.Errorf("peek stock failed: %w", err)
	}

	stock, err := entity.NewStockFromState(cmd.ActivityNo, cmd.SKUNo, available, available)
	if err != nil {
		return fmt.Errorf("init stock from state: %w", err)
	}

	// 5. 构造聚合根并记录事件
	agg := entity.NewStockAggregate(stock)
	agg.RecordReserved(cmd.Quantity, cmd.UserID, cmd.OrderNo)

	// 6. 发布事件
	for _, evt := range agg.GetDomainEvents() {
		if err := s.bus.Publish(ctx, evt); err != nil {
			return fmt.Errorf("publish event failed: %w", err)
		}
	}

	agg.ClearDomainEvents()
	return nil
}

// ReleaseStock 释放库存
func (s *StockAppService) ReleaseStock(ctx context.Context, cmd ReleaseStockCommand) error {
	// 1. 验证命令
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrCommandValidation, err)
	}

	// 2. 调用 Repository 释放库存
	if err := s.repo.ReleaseStock(
		ctx, cmd.ActivityNo, cmd.SKUNo,
		cmd.UserID, cmd.Quantity, cmd.OrderNo,
	); err != nil {
		return fmt.Errorf("release stock failed: %w", err)
	}

	// 3. 从 Redis 加载最新状态
	available, err := s.repo.PeekStock(ctx, cmd.ActivityNo, cmd.SKUNo)
	if err != nil {
		return fmt.Errorf("peek stock failed: %w", err)
	}

	stock, err := entity.NewStockFromState(cmd.ActivityNo, cmd.SKUNo, available, available)
	if err != nil {
		return fmt.Errorf("restore stock from state: %w", err)
	}

	// 4. 构造聚合根并记录事件
	agg := entity.NewStockAggregate(stock)
	agg.RecordReleased(cmd.Quantity, cmd.UserID, cmd.OrderNo)

	// 5. 发布事件
	for _, evt := range agg.GetDomainEvents() {
		if err := s.bus.Publish(ctx, evt); err != nil {
			return fmt.Errorf("publish event failed: %w", err)
		}
	}

	agg.ClearDomainEvents()
	return nil
}
