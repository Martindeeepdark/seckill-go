package application

import (
	"context"
	"fmt"

	commonlogs "github.com/Martindeeepdark/go-common/logs"

	"seckill-activity-service/internal/domain/entity"
	"seckill-activity-service/internal/domain/repository"
	"seckill-activity-service/internal/infrastructure/eventbus"
)

// ActivityAppService 活动应用服务，纯编排器
type ActivityAppService struct {
	activityRepo repository.ActivityRepository
	eventBus     eventbus.Bus
}

// NewActivityAppService 创建活动应用服务实例
func NewActivityAppService(
	activityRepo repository.ActivityRepository,
	eventBus eventbus.Bus,
	_ any, // logger 参数保留用于签名兼容
) *ActivityAppService {
	return &ActivityAppService{
		activityRepo: activityRepo,
		eventBus:     eventBus,
	}
}

// StartActivity 开始活动
func (s *ActivityAppService) StartActivity(ctx context.Context, cmd StartActivityCommand) error {
	// 验证命令
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// 加载聚合根
	activity, err := s.activityRepo.GetByActivityNo(ctx, cmd.ActivityNo)
	if err != nil {
		return fmt.Errorf("load activity: %w", err)
	}

	// 执行业务逻辑（委托给聚合根）
	if err := activity.Start(cmd.StartedAt); err != nil {
		return fmt.Errorf("start activity: %w", err)
	}

	// 保存聚合根
	if err := s.activityRepo.Save(ctx, activity); err != nil {
		return fmt.Errorf("save activity: %w", err)
	}

	// 发布领域事件
	s.publishEvents(ctx, activity)

	return nil
}

// AddSKU 添加商品
func (s *ActivityAppService) AddSKU(ctx context.Context, cmd AddSKUCommand) error {
	// 验证命令
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// 加载聚合根
	activity, err := s.activityRepo.GetByActivityNo(ctx, cmd.ActivityNo)
	if err != nil {
		return fmt.Errorf("load activity: %w", err)
	}

	// 执行业务逻辑（委托给聚合根）
	sku := entity.SKU{
		SKUNo:        cmd.SKUNo,
		TotalStock:   cmd.Stock,
		SeckillPrice: cmd.Price,
	}
	if err := activity.AddSKU(sku); err != nil {
		return fmt.Errorf("add sku: %w", err)
	}

	// 保存聚合根
	if err := s.activityRepo.Save(ctx, activity); err != nil {
		return fmt.Errorf("save activity: %w", err)
	}

	// 发布领域事件
	s.publishEvents(ctx, activity)

	return nil
}

// publishEvents 发布领域事件
func (s *ActivityAppService) publishEvents(ctx context.Context, activity *entity.Activity) {
	events := activity.GetDomainEvents()
	for _, event := range events {
		if err := s.eventBus.Publish(ctx, event); err != nil {
			commonlogs.CtxErrorf(ctx, "publish event failed event=%s error=%v", event.EventName(), err)
		}
	}
	activity.ClearDomainEvents()
}
