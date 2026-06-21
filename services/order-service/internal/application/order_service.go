package application

import (
	"context"
	"fmt"

	commonlogs "github.com/Martindeeepdark/go-common/logs"

	eventbus "seckill-common/eventbus"
	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/domain/repository"
)

// OrderAppService 是订单应用服务，编排订单创建、支付、关闭等用例并发布领域事件。
type OrderAppService struct {
	orderRepo repository.OrderRepository
	eventBus  eventbus.Bus
}

// NewOrderAppService 创建订单应用服务实例，注入订单仓储、事件总线和 logger。
func NewOrderAppService(
	orderRepo repository.OrderRepository,
	eventBus eventbus.Bus,
	_ any, // logger 参数保留用于签名兼容
) *OrderAppService {
	return &OrderAppService{
		orderRepo: orderRepo,
		eventBus:  eventBus,
	}
}

// CreateOrder 校验命令后创建订单聚合，持久化并发布订单创建事件。
func (s *OrderAppService) CreateOrder(ctx context.Context, cmd CreateOrderCommand) error {
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	order, err := entity.CreateOrder(
		cmd.OrderNo,
		cmd.UserID,
		cmd.ActivityNo,
		cmd.SKUNo,
		cmd.Quantity,
		cmd.PayAmount,
		cmd.TraceID,
	)
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	order.RequestTraceID = cmd.RequestTraceID

	if err := s.orderRepo.Save(ctx, order); err != nil {
		return fmt.Errorf("save order: %w", err)
	}

	s.publishEvents(ctx, order)
	return nil
}

// PayOrder 加载订单并标记为已支付，持久化并发布订单支付事件。
func (s *OrderAppService) PayOrder(ctx context.Context, cmd PayOrderCommand) error {
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	order, err := s.orderRepo.GetByOrderNo(ctx, cmd.OrderNo)
	if err != nil {
		return fmt.Errorf("load order: %w", err)
	}

	if err := order.MarkAsPaid(cmd.TransactionNo, cmd.Amount, cmd.PaidAt); err != nil {
		return fmt.Errorf("mark paid: %w", err)
	}

	if err := s.orderRepo.Save(ctx, order); err != nil {
		return fmt.Errorf("save order: %w", err)
	}

	s.publishEvents(ctx, order)
	return nil
}

// CloseOrder 加载订单并执行关单，持久化并发布订单关闭事件。
func (s *OrderAppService) CloseOrder(ctx context.Context, cmd CloseOrderCommand) error {
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	order, err := s.orderRepo.GetByOrderNo(ctx, cmd.OrderNo)
	if err != nil {
		return fmt.Errorf("load order: %w", err)
	}

	if err := order.Close(cmd.ClosedAt); err != nil {
		return fmt.Errorf("close order: %w", err)
	}

	if err := s.orderRepo.Save(ctx, order); err != nil {
		return fmt.Errorf("save order: %w", err)
	}

	s.publishEvents(ctx, order)
	return nil
}

func (s *OrderAppService) publishEvents(ctx context.Context, order *entity.Order) {
	events := order.GetUncommittedEvents()
	for _, event := range events {
		if err := s.eventBus.Publish(ctx, event); err != nil {
			commonlogs.CtxErrorf(ctx, "publish event failed event=%s orderNo=%s error=%v",
				event.EventName(), order.OrderNo, err)
		}
	}
	order.ClearEvents()
}
