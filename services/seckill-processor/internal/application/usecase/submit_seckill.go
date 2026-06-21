// Package usecase 提供应用层的 Use Case 实现
// 每个用例封装一个独立的业务流程，从处理器中提取纯粹的业务逻辑
package usecase

import (
	"context"
	"fmt"
	"time"

	commonlogs "github.com/Martindeeepdark/go-common/logs"

	"seckill-processor-service/internal/domain/model"
	"seckill-processor-service/internal/domain/service"
)

const (
	submitTraceProcessingTTL = time.Minute // processor idem key 的 PROCESSING TTL(崩溃恢复窗口)
)

// TraceResultStore 追踪结果存储接口(gateway result key)
// 用于 application.SeckillApp 写入前端轮询兼容的 gateway result key
// 这里保留为最小接口,供上层注入时类型约束
type TraceResultStore interface {
	TryStart(ctx context.Context, traceID string, ttl time.Duration) (bool, error)
	Delete(ctx context.Context, traceID string) error
}

// ProcessorStore processor 端的幂等存储接口
// 由 common/traceresult.ProcessorStore 实现,使用独立 key 前缀 seckill:processor:idem:
// 与 gateway 的 TraceResultStore 隔离,解决 SetNX 永远 false 的失效问题
type ProcessorStore interface {
	TryStart(ctx context.Context, traceID string, ttl time.Duration) (bool, error)
	MarkSuccess(ctx context.Context, traceID, orderNo string, ttl time.Duration) error
	MarkFail(ctx context.Context, traceID, reason string, ttl time.Duration) error
	Release(ctx context.Context, traceID string) error
}

// SubmitSeckill 秒杀提交 Use Case
// 从 SeckillApp.HandleSeckill() 提取的业务逻辑：
// 1. traceId 标准化
// 2. Layer 1 防重复检查(独立 processor idem key,SetNX)
// 3. 调用领域服务 seckill.Submit()
// 4. 失败时 Release processor idem key(仅删 PROCESSING 值,不动 gateway result key)
type SubmitSeckill struct {
	seckill        *service.SeckillService
	processorStore ProcessorStore // 独立 processor 幂等 key,为 nil 时跳过 Layer 1
}

// NewSubmitSeckill 创建秒杀提交 Use Case 实例
func NewSubmitSeckill(
	seckill *service.SeckillService,
	processorStore ProcessorStore,
	_ any, // logger 参数保留用于签名兼容
) *SubmitSeckill {
	return &SubmitSeckill{
		seckill:        seckill,
		processorStore: processorStore,
	}
}

// Execute 执行秒杀提交
func (uc *SubmitSeckill) Execute(ctx context.Context, message model.SeckillMessage) error {
	// 标准化消息字段
	if message.TraceID == "" {
		message.TraceID = message.RequestTraceID
	}
	if message.Quantity <= 0 {
		message.Quantity = 1
	}

	// Layer 1: processor 端幂等检查(独立 key,与 gateway result key 隔离)
	if uc.processorStore != nil {
		acquired, err := uc.processorStore.TryStart(ctx, message.TraceID, submitTraceProcessingTTL)
		if err != nil {
			return fmt.Errorf("reserve processor idem %s: %w", message.TraceID, err)
		}
		if !acquired {
			commonlogs.CtxInfof(ctx, "seckill message already processing or processed traceId=%s requestTraceId=%s",
				message.TraceID, message.RequestTraceID)
			return nil
		}
	}

	// 提交给领域服务处理
	err := uc.seckill.Submit(ctx, model.SubmitCommand{
		TraceID:        message.TraceID,
		RequestTraceID: message.RequestTraceID,
		ActivityNo:     message.ActivityNo,
		SKUNo:          message.SKUNo,
		UserID:         message.UserID,
		Quantity:       message.Quantity,
		TotalFee:       message.TotalFee,
		RequestIP:      message.RequestIP,
		RunID:          message.RunID,
	})
	if err != nil {
		// 基础设施错误(临时性) → Release processor idem key 允许重试
		// 注意: gateway result key 不动(用户继续看到 PROCESSING 直到最终结果写入)
		uc.releaseProcessorIdem(ctx, message.TraceID)
		commonlogs.CtxErrorf(ctx, "order process failed traceId=%s requestTraceId=%s error=%v",
			message.TraceID, message.RequestTraceID, err)
		return fmt.Errorf("submit seckill: %w", err)
	}
	return nil
}

// releaseProcessorIdem 释放 processor 幂等 key(处理失败时调用)
// 使用 Lua CAS: 仅删除值=PROCESSING 的 key,不误删最终结果
func (uc *SubmitSeckill) releaseProcessorIdem(ctx context.Context, traceID string) {
	if uc.processorStore == nil || traceID == "" {
		return
	}
	if err := uc.processorStore.Release(ctx, traceID); err != nil {
		commonlogs.CtxWarnf(ctx, "release processor idem failed traceId=%s error=%v", traceID, err)
	}
}
