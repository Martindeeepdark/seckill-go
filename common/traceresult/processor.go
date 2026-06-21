// Package traceresult 提供链路追踪结果存储
// processor.go 提供 seckill-processor 专用的幂等存储,与 gateway 用的 RedisStore 隔离
package traceresult

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// processorKeyPrefix 是 processor 幂等 key 的前缀,与 gateway 的 keyPrefix 区别开
// gateway:  seckill:order:result:<traceID>  (TTL 5min,前端轮询窗口)
// processor: seckill:processor:idem:<traceID> (TTL 60s,崩溃恢复窗口)
const processorKeyPrefix = "seckill:processor:idem:"

// releaseScript 仅当 key 当前值为 PROCESSING 时才删除
// 避免误删最终结果(订单号或失败原因)
const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
`

// ProcessorStore processor 端的幂等存储
// 与 gateway 用的 RedisStore 共享 Redis 实例但 key 前缀独立,
// 避免 gateway 写 PROCESSING 后 processor 的 SetNX 永远 false 的失效问题.
//
// 状态机:
//
//	TryStart    -> SetNX 写 PROCESSING (推荐 60s TTL)
//	MarkSuccess -> Set 写订单号 (推荐 5min TTL), 覆盖 PROCESSING
//	MarkFail    -> Set 写失败原因 (推荐 5min TTL), 覆盖 PROCESSING
//	Release     -> Lua CAS 删除 (仅当值=PROCESSING), 允许重试不误删最终结果
type ProcessorStore struct {
	client *goredis.Client
}

// NewProcessorStore 创建 processor 幂等存储实例
func NewProcessorStore(client *goredis.Client) *ProcessorStore {
	return &ProcessorStore{client: client}
}

// processorKey 生成 processor 幂等 key
func processorKey(traceID string) string {
	return processorKeyPrefix + traceID
}

// ProcessorKey 暴露 key 生成函数(供测试断言使用)
func ProcessorKey(traceID string) string {
	return processorKey(traceID)
}

// TryStart 尝试抢占处理权
// 使用 SetNX 原子操作:首次设置返回 true,已存在返回 false
// 推荐使用 60s TTL(崩溃恢复窗口)
func (s *ProcessorStore) TryStart(ctx context.Context, traceID string, ttl time.Duration) (bool, error) {
	if traceID == "" {
		return false, nil
	}
	ok, err := s.client.SetNX(ctx, processorKey(traceID), Processing, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("processor setnx %s: %w", traceID, err)
	}
	return ok, nil
}

// MarkSuccess 把 PROCESSING 覆盖为订单号(最终结果)
// 直接 Set,允许覆盖任何当前值(包括 PROCESSING 或上一次失败原因)
func (s *ProcessorStore) MarkSuccess(ctx context.Context, traceID, orderNo string, ttl time.Duration) error {
	if traceID == "" {
		return nil
	}
	if err := s.client.Set(ctx, processorKey(traceID), orderNo, ttl).Err(); err != nil {
		return fmt.Errorf("processor mark success %s: %w", traceID, err)
	}
	return nil
}

// MarkFail 把 PROCESSING 覆盖为失败原因(最终结果)
func (s *ProcessorStore) MarkFail(ctx context.Context, traceID, reason string, ttl time.Duration) error {
	if traceID == "" {
		return nil
	}
	if err := s.client.Set(ctx, processorKey(traceID), reason, ttl).Err(); err != nil {
		return fmt.Errorf("processor mark fail %s: %w", traceID, err)
	}
	return nil
}

// Release 释放幂等 key,允许下次重试
// 使用 Lua CAS: 仅当当前值=PROCESSING 时才删除
// 如果值已经是最终结果(订单号/失败原因),不删除(防误删)
func (s *ProcessorStore) Release(ctx context.Context, traceID string) error {
	if traceID == "" {
		return nil
	}
	if err := s.client.Eval(ctx, releaseScript, []string{processorKey(traceID)}, Processing).Err(); err != nil {
		return fmt.Errorf("processor release %s: %w", traceID, err)
	}
	return nil
}
