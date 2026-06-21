// Package application 提供支持服务的应用层逻辑
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"seckill-common/tracing"

	paymentdomain "seckill-support-service/internal/domain/entity"
)

// ErrUnknownPostPayTask 表示未知的支付后任务类型。
var ErrUnknownPostPayTask = errors.New("unknown post-pay task")

// PostPayTaskConsumer 支付后任务消费者接口
type PostPayTaskConsumer interface {
	ConsumePostPayTasks(ctx context.Context, consumer string, handler func(context.Context, paymentdomain.PostPayTask) error) error
}

// PostPayProcessor 支付后任务处理器
type PostPayProcessor struct {
	cards    FreeCardGateway     // 自由卡网关
	sync     OrderSyncGateway    // 订单同步网关
	consumer PostPayTaskConsumer // 任务消费者
	logger   *slog.Logger        // 日志记录器
}

// NewPostPayProcessor 创建支付后任务处理器
func NewPostPayProcessor(cards FreeCardGateway, sync OrderSyncGateway, consumer PostPayTaskConsumer, logger *slog.Logger) *PostPayProcessor {
	if logger == nil {
		logger = slog.Default()
	}
	return &PostPayProcessor{cards: cards, sync: sync, consumer: consumer, logger: logger}
}

// Run 启动任务处理器
func (p *PostPayProcessor) Run(ctx context.Context, consumer string) error {
	// 消费者为空时直接退出
	if p.consumer == nil {
		<-ctx.Done()
		return fmt.Errorf("post pay processor stopped: %w", ctx.Err())
	}
	// 开始消费任务
	if err := p.consumer.ConsumePostPayTasks(ctx, consumer, p.handle); err != nil {
		return fmt.Errorf("consume post pay tasks: %w", err)
	}
	return nil
}

// handle 处理单个支付后任务
func (p *PostPayProcessor) handle(ctx context.Context, task paymentdomain.PostPayTask) (err error) {
	// 创建追踪span
	ctx, span, tid := tracing.StartSpan(ctx, "support.post_pay.handle", task.RequestTraceID)
	defer func() { tracing.EndSpan(span, err) }()
	// 补充trace ID
	if task.RequestTraceID == "" {
		task.RequestTraceID = tid
	}
	// 根据任务类型分发处理
	switch task.Type {
	case paymentdomain.PostPayTaskSyncOrder:
		if task.SyncOrder == nil {
			return fmt.Errorf("%w: missing sync order", ErrUnknownPostPayTask)
		}
		if err := p.sync.SyncOrder(ctx, *task.SyncOrder); err != nil {
			return fmt.Errorf("sync order: %w", err)
		}
		return nil
	case paymentdomain.PostPayTaskIssueCard:
		if task.IssueCard == nil {
			return fmt.Errorf("%w: missing issue card", ErrUnknownPostPayTask)
		}
		_, err := p.cards.IssueCard(ctx, *task.IssueCard)
		if err != nil {
			return fmt.Errorf("issue card: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnknownPostPayTask, task.Type)
	}
}
