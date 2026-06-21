package usecase

import (
	"context"
	"fmt"

	commonlogs "github.com/Martindeeepdark/go-common/logs"

	"seckill-processor-service/internal/domain"
	"seckill-processor-service/internal/domain/model"
	"seckill-processor-service/internal/domain/service"
	"seckill-processor-service/internal/domain/status"

	commonerrors "seckill-common/errors"
)

// PostPayFreeCardGateway 自由卡网关接口
type PostPayFreeCardGateway = service.FreeCardGateway

// PostPayOrderSyncGateway 订单同步网关接口
type PostPayOrderSyncGateway = service.OrderSyncGateway

// HandlePostPay 支付后任务处理 Use Case
// 从 PostPayProcessor.handle() 提取的任务分发逻辑：
// 根据任务类型（SYNC_ORDER / ISSUE_CARD）分发到对应的处理器
type HandlePostPay struct {
	cards PostPayFreeCardGateway
	sync  PostPayOrderSyncGateway
}

// NewHandlePostPay 创建支付后任务处理 Use Case 实例
func NewHandlePostPay(
	cards PostPayFreeCardGateway,
	sync PostPayOrderSyncGateway,
	_ any, // logger 参数保留用于签名兼容
) *HandlePostPay {
	return &HandlePostPay{
		cards: cards,
		sync:  sync,
	}
}

// Execute 执行支付后任务处理
// 根据任务类型分发到不同的处理器
func (uc *HandlePostPay) Execute(ctx context.Context, task model.PostPayTask) error {
	switch task.Type {
	case status.PostPayTaskSyncOrder:
		if task.SyncOrder == nil {
			return commonerrors.WrapTerminal(fmt.Errorf("%w: missing sync order payload", domain.ErrUnknownPostPayTask))
		}
		if err := uc.sync.SyncOrder(ctx, *task.SyncOrder); err != nil {
			return fmt.Errorf("sync order %s: %w", task.OrderNo, err)
		}
		commonlogs.CtxInfof(ctx, "post-pay order sync handled orderNo=%s", task.OrderNo)
		return nil

	case status.PostPayTaskIssueCard:
		// 处理自由卡发放任务
		if task.IssueCard == nil {
			return commonerrors.WrapTerminal(fmt.Errorf("%w: missing issue card payload", domain.ErrUnknownPostPayTask))
		}
		cardNo, err := uc.cards.IssueCard(ctx, *task.IssueCard)
		if err != nil {
			return fmt.Errorf("issue card for order %s: %w", task.OrderNo, err)
		}
		commonlogs.CtxInfof(ctx, "post-pay card issue handled orderNo=%s cardNo=%s", task.OrderNo, cardNo)
		return nil

	default:
		return commonerrors.WrapTerminal(fmt.Errorf("%w: %s", domain.ErrUnknownPostPayTask, task.Type))
	}
}
