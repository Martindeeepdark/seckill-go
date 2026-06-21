// Package application 提供支付后任务处理器
// 负责处理订单同步和自由卡发放等支付后任务
package application

import (
	"context"
	"fmt"
	"log/slog"

	"seckill-processor-service/internal/application/usecase"
	"seckill-processor-service/internal/domain/model"

	"seckill-common/tracing"
)

// PostPayProcessor 支付后任务处理器
// 处理订单同步和自由卡发放
type PostPayProcessor struct {
	handlePostPay *usecase.HandlePostPay  // 支付后任务处理 Use Case
	consumer      PostPayTaskConsumer     // 支付后任务消费者
	logger        *slog.Logger            // 日志记录器
}

// NewPostPayProcessor 创建支付后任务处理器实例
func NewPostPayProcessor(
	handlePostPay *usecase.HandlePostPay,
	consumer PostPayTaskConsumer,
	logger *slog.Logger,
) *PostPayProcessor {
	if logger == nil {
		logger = slog.Default()
	}
	return &PostPayProcessor{
		handlePostPay: handlePostPay,
		consumer:      consumer,
		logger:        logger,
	}
}

// Run 启动支付后任务消费
// 阻塞运行直到上下文取消
func (p *PostPayProcessor) Run(ctx context.Context, consumer string) error {
	if p.consumer == nil {
		<-ctx.Done()
		return fmt.Errorf("post-pay consumer stopped: %w", ctx.Err())
	}
	if err := p.consumer.ConsumePostPayTasks(ctx, consumer, p.handle); err != nil {
		return fmt.Errorf("consume post-pay tasks: %w", err)
	}
	return nil
}

// handle 处理单个支付后任务
// 负责 tracing + 调用 Use Case
func (p *PostPayProcessor) handle(ctx context.Context, task model.PostPayTask) (err error) {
	ctx, span, requestTraceID := tracing.StartSpan(ctx, "support.post_pay.handle", task.RequestTraceID)
	defer func() {
		tracing.EndSpan(span, err)
	}()
	if task.RequestTraceID == "" {
		task.RequestTraceID = requestTraceID
	}

	return p.handlePostPay.Execute(ctx, task)
}
