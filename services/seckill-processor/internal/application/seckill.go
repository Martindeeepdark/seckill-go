// Package application 提供秒杀应用层服务
// 负责协调领域服务和基础设施，处理消息消费和事件发布
package application

import (
	"context"
	"fmt"
	"time"

	commonlogs "github.com/Martindeeepdark/go-common/logs"
	"github.com/Martindeeepdark/go-common/eventbus"

	"seckill-common/tracing"

	domainevent "seckill-processor-service/internal/domain/event"
	"seckill-processor-service/internal/domain/model"

	"seckill-processor-service/internal/application/usecase"
)

const (
	traceResultTTL             = 5 * time.Minute  // 追踪结果的 TTL
	defaultPaymentTimeoutDelay = 10 * time.Minute // 默认支付超时延迟
)

// SeckillAppOption 秒杀应用配置选项
type SeckillAppOption func(*SeckillApp)

// SeckillApp 秒杀应用服务
// 负责消费秒杀消息并协调领域服务处理
type SeckillApp struct {
	submitSeckill       *usecase.SubmitSeckill  // 秒杀提交 Use Case
	events              eventbus.Bus            // 事件总线
	consumer            MessageConsumer         // 消息消费者
	paymentTimeouts     PaymentTimeoutPublisher // 支付超时任务发布器
	traceResults        TraceResultStore        // gateway result key 存储(前端轮询兼容)
	processorStore      ProcessorStore          // processor 幂等 key 存储(Layer 1)
	paymentTimeoutDelay time.Duration           // 支付超时延迟
}

// NewSeckillApp 创建秒杀应用服务实例
func NewSeckillApp(
	submitSeckill *usecase.SubmitSeckill,
	events eventbus.Bus,
	consumer MessageConsumer,
	_ any, // logger 参数保留用于签名兼容
	opts ...SeckillAppOption,
) *SeckillApp {
	app := &SeckillApp{
		submitSeckill:       submitSeckill,
		events:              events,
		consumer:            consumer,
		paymentTimeoutDelay: defaultPaymentTimeoutDelay,
	}
	for _, opt := range opts {
		opt(app)
	}
	return app
}

// WithPaymentTimeouts 配置支付超时任务发布器
func WithPaymentTimeouts(publisher PaymentTimeoutPublisher, delay time.Duration) SeckillAppOption {
	return func(app *SeckillApp) {
		app.paymentTimeouts = publisher
		if delay > 0 {
			app.paymentTimeoutDelay = delay
		}
	}
}

// WithTraceResults 配置追踪结果存储
func WithTraceResults(store TraceResultStore) SeckillAppOption {
	return func(app *SeckillApp) {
		app.traceResults = store
	}
}

// WithProcessorStore 配置 processor 幂等存储(Layer 1)
// 与 WithTraceResults(gateway result key)互相独立,两者都需注入
func WithProcessorStore(store ProcessorStore) SeckillAppOption {
	return func(app *SeckillApp) {
		app.processorStore = store
	}
}

// RegisterHandlers 注册事件处理器
// 订阅订单创建和秒杀拒绝事件
func (app *SeckillApp) RegisterHandlers() error {
	if app.events == nil {
		return nil
	}
	if err := app.events.Subscribe(domainevent.TopicOrderCreated, app.onOrderCreated); err != nil {
		return fmt.Errorf("subscribe %s: %w", domainevent.TopicOrderCreated, err)
	}
	if err := app.events.Subscribe(domainevent.TopicSeckillRejected, app.onSeckillRejected); err != nil {
		return fmt.Errorf("subscribe %s: %w", domainevent.TopicSeckillRejected, err)
	}
	return nil
}

// Run 启动秒杀消息消费
// 阻塞运行直到上下文取消
func (app *SeckillApp) Run(ctx context.Context, group string, consumerName string) error {
	if app.consumer == nil {
		<-ctx.Done()
		return fmt.Errorf("seckill consumer stopped: %w", ctx.Err())
	}
	if err := app.consumer.Consume(ctx, group, consumerName, app.HandleSeckill); err != nil {
		return fmt.Errorf("consume seckill messages: %w", err)
	}
	return nil
}

// HandleSeckill 处理秒杀消息
// 负责 tracing + 调用 Use Case
func (app *SeckillApp) HandleSeckill(ctx context.Context, message model.SeckillMessage) error {
	var spanErr error
	ctx, span, requestTraceID := tracing.StartSpan(ctx, "seckill.processor.process", message.RequestTraceID)
	defer func() {
		tracing.EndSpan(span, spanErr)
	}()

	// 标准化 RequestTraceID（由处理器层负责，不进入 Use Case）
	if message.RequestTraceID == "" {
		message.RequestTraceID = requestTraceID
	}

	if err := app.submitSeckill.Execute(ctx, message); err != nil {
		spanErr = err
		return err
	}
	return nil
}

// onOrderCreated 处理订单创建事件
// 发布支付超时任务并标记追踪结果为成功
func (app *SeckillApp) onOrderCreated(created domainevent.OrderCreated) {
	ctx := tracing.WithTraceID(context.Background(), created.RequestTraceID)
	app.publishPaymentTimeout(ctx, created)
	app.markTraceSuccess(ctx, created.TraceID, created.OrderNo)
}

// onSeckillRejected 处理秒杀拒绝事件
// 标记追踪结果为失败
func (app *SeckillApp) onSeckillRejected(rejected domainevent.SeckillRejected) {
	ctx := tracing.WithTraceID(context.Background(), rejected.RequestTraceID)
	app.markTraceFail(ctx, rejected.TraceID, rejected.Reason)
}

// markTraceSuccess 标记追踪成功
// 双 key 写入: 先写 gateway result key(前端轮询立即可见)→ 再写 processor idem key
// 失败为 best-effort,记日志告警(订单已创建,不能因为 Redis 失败而回滚)
func (app *SeckillApp) markTraceSuccess(ctx context.Context, traceID, orderNo string) {
	if traceID == "" {
		return
	}
	// 1. gateway result key (前端轮询兼容)
	if app.traceResults != nil {
		if err := app.traceResults.MarkSuccess(ctx, traceID, orderNo, traceResultTTL); err != nil {
			commonlogs.CtxWarnf(ctx, "mark trace success (gateway key) failed traceId=%s orderNo=%s error=%v",
				traceID, orderNo, err)
		}
	}
	// 2. processor idem key (Layer 1 防重复)
	if app.processorStore != nil {
		if err := app.processorStore.MarkSuccess(ctx, traceID, orderNo, traceResultTTL); err != nil {
			commonlogs.CtxWarnf(ctx, "mark trace success (processor idem key) failed traceId=%s orderNo=%s error=%v",
				traceID, orderNo, err)
		}
	}
}

// markTraceFail 标记追踪失败
// 双 key 写入顺序同 markTraceSuccess
func (app *SeckillApp) markTraceFail(ctx context.Context, traceID, reason string) {
	if traceID == "" {
		return
	}
	// 1. gateway result key
	if app.traceResults != nil {
		if err := app.traceResults.MarkFail(ctx, traceID, reason, traceResultTTL); err != nil {
			commonlogs.CtxWarnf(ctx, "mark trace fail (gateway key) failed traceId=%s reason=%s error=%v",
				traceID, reason, err)
		}
	}
	// 2. processor idem key
	if app.processorStore != nil {
		if err := app.processorStore.MarkFail(ctx, traceID, reason, traceResultTTL); err != nil {
			commonlogs.CtxWarnf(ctx, "mark trace fail (processor idem key) failed traceId=%s reason=%s error=%v",
				traceID, reason, err)
		}
	}
}

// publishPaymentTimeout 发布支付超时任务
func (app *SeckillApp) publishPaymentTimeout(ctx context.Context, created domainevent.OrderCreated) {
	if app.paymentTimeouts == nil {
		return
	}
	task := model.PaymentTimeoutTask{
		OrderNo:        created.OrderNo,
		RequestTraceID: created.RequestTraceID,
		DueAt:          time.Now().Add(app.paymentTimeoutDelay),
	}
	if err := app.paymentTimeouts.PublishPaymentTimeout(ctx, task); err != nil {
		commonlogs.CtxWarnf(ctx, "publish payment timeout task failed orderNo=%s error=%v", created.OrderNo, err)
	} else {
		commonlogs.CtxInfof(ctx, "payment timeout task published orderNo=%s dueAt=%s",
			created.OrderNo, task.DueAt.Format(time.RFC3339))
	}
}
