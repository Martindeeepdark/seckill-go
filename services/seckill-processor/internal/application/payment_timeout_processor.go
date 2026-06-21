// Package application 提供支付超时处理器
// 负责关闭超时未支付的订单并释放库存
package application

import (
	"context"
	"fmt"
	"log/slog"

	"seckill-processor-service/internal/application/usecase"
	"seckill-processor-service/internal/domain/model"

	"seckill-common/tracing"
)

// PaymentTimeoutProcessor 支付超时处理器
// 关闭超过支付窗口的订单并释放其预留库存
type PaymentTimeoutProcessor struct {
	handlePaymentTimeout *usecase.HandlePaymentTimeout // 支付超时处理 Use Case
	consumer             PaymentTimeoutConsumer        // 支付超时任务消费者
	logger               *slog.Logger                  // 日志记录器
}

// NewPaymentTimeoutProcessor 创建支付超时处理器实例
func NewPaymentTimeoutProcessor(
	handlePaymentTimeout *usecase.HandlePaymentTimeout,
	consumer PaymentTimeoutConsumer,
	logger *slog.Logger,
) *PaymentTimeoutProcessor {
	if logger == nil {
		logger = slog.Default()
	}
	return &PaymentTimeoutProcessor{
		handlePaymentTimeout: handlePaymentTimeout,
		consumer:             consumer,
		logger:               logger,
	}
}

// Run 启动支付超时任务消费
// 阻塞运行直到上下文取消
func (p *PaymentTimeoutProcessor) Run(ctx context.Context, consumer string) error {
	if p.consumer == nil {
		<-ctx.Done()
		return fmt.Errorf("payment timeout consumer stopped: %w", ctx.Err())
	}
	if err := p.consumer.ConsumePaymentTimeouts(ctx, consumer, p.Handle); err != nil {
		return fmt.Errorf("consume payment timeouts: %w", err)
	}
	return nil
}

// Handle 处理单个支付超时任务
// 负责 tracing + 调用 Use Case
func (p *PaymentTimeoutProcessor) Handle(ctx context.Context, task model.PaymentTimeoutTask) (err error) {
	ctx, span, requestTraceID := tracing.StartSpan(ctx, "seckill.payment_timeout.handle", task.RequestTraceID)
	defer func() {
		tracing.EndSpan(span, err)
	}()
	if task.RequestTraceID == "" {
		task.RequestTraceID = requestTraceID
	}

	return p.handlePaymentTimeout.Execute(ctx, task)
}
