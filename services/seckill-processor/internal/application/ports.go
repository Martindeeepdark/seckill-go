// Package application 定义应用层的端口（接口）和数据类型
package application

import (
	"context"
	"time"

	"seckill-processor-service/internal/domain/model"
	"seckill-processor-service/internal/domain/service"
)

// ActivityGateway 活动网关接口，通过 gRPC 访问活动和 SKU 数据
type ActivityGateway = service.ActivityQuery

// StockGateway 库存网关接口，通过 gRPC 进行库存扣减和释放
type StockGateway = service.StockGateway

// RiskGateway 风控网关接口，通过 gRPC 进行风控评估
type RiskGateway = service.RiskGateway

// OrderGateway 订单网关接口，通过 gRPC 进行订单操作
type OrderGateway interface {
	service.OrderCreator
	GetOrder(ctx context.Context, orderNo string) (model.OrderInfo, error)
	MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error
	CloseOrder(ctx context.Context, orderNo string) error
}

// PaymentGateway 支付网关接口，通过 gRPC 查询和关闭支付
type PaymentGateway interface {
	QueryPayment(ctx context.Context, orderNo string) (model.PayQueryResult, error)
	ClosePayment(ctx context.Context, orderNo string) error
}

// FreeCardGateway 自由卡网关接口
type FreeCardGateway = service.FreeCardGateway

// OrderSyncGateway 订单同步网关接口
type OrderSyncGateway = service.OrderSyncGateway

// MessagePublisher 秒杀消息发布器接口
type MessagePublisher interface {
	Publish(ctx context.Context, message model.SeckillMessage) error
}

// MessageConsumer 秒杀消息消费者接口
type MessageConsumer interface {
	Consume(ctx context.Context, group string, consumer string, handler func(context.Context, model.SeckillMessage) error) error
}

// TraceResultStore 追踪结果存储接口
// 用于记录异步秒杀处理结果，防止重复处理
type TraceResultStore interface {
	TryStart(ctx context.Context, traceID string, ttl time.Duration) (bool, error)
	MarkSuccess(ctx context.Context, traceID, orderNo string, ttl time.Duration) error
	MarkFail(ctx context.Context, traceID, reason string, ttl time.Duration) error
	Delete(ctx context.Context, traceID string) error
}

// ProcessorStore processor 端的幂等存储接口
// 由 common/traceresult.ProcessorStore 实现
// 与 TraceResultStore (gateway key) 隔离,使用独立 key 前缀
type ProcessorStore interface {
	TryStart(ctx context.Context, traceID string, ttl time.Duration) (bool, error)
	MarkSuccess(ctx context.Context, traceID, orderNo string, ttl time.Duration) error
	MarkFail(ctx context.Context, traceID, reason string, ttl time.Duration) error
	Release(ctx context.Context, traceID string) error
}

// PaymentTimeoutPublisher 支付超时任务发布器接口
type PaymentTimeoutPublisher interface {
	PublishPaymentTimeout(ctx context.Context, task model.PaymentTimeoutTask) error
}

// PaymentTimeoutConsumer 支付超时任务消费者接口
type PaymentTimeoutConsumer interface {
	ConsumePaymentTimeouts(ctx context.Context, consumer string, handler func(context.Context, model.PaymentTimeoutTask) error) error
}

// PostPayTaskConsumer 支付后任务消费者接口
type PostPayTaskConsumer interface {
	ConsumePostPayTasks(ctx context.Context, consumer string, handler func(context.Context, model.PostPayTask) error) error
}
