// Package persistence 提供内存存储实现
package persistence

import (
	"context"
	"sort"
	"sync"
	"time"

	"seckill-order-service/internal/domain/entity"
	statemachine "seckill-order-service/internal/domain/status"
)

// MemoryStore 内存存储，用于测试或作为降级方案
type MemoryStore struct {
	mu     sync.Mutex              // 互斥锁保护并发访问
	orders map[string]entity.Order // 订单映射，key为订单号
}

// NewMemoryStore 创建内存存储实例
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{orders: map[string]entity.Order{}}
}

// CreateOrder 创建订单
// ctx: 上下文（未使用）
// order: 订单实体
// 返回错误表示创建失败
func (s *MemoryStore) CreateOrder(_ context.Context, order entity.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 检查订单是否已存在
	if _, exists := s.orders[order.OrderNo]; exists {
		return ErrDuplicate
	}
	s.orders[order.OrderNo] = order
	return nil
}

// GetOrder 获取订单
// ctx: 上下文（未使用）
// orderNo: 订单号
// 返回订单实体和错误
func (s *MemoryStore) GetOrder(_ context.Context, orderNo string) (entity.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderNo]
	if !ok {
		return entity.Order{}, ErrNotFound
	}
	return order, nil
}

// GetByUserAndTrace 根据 (user_id, trace_id) 组合查询订单
// 用于 DuplicateKey (23505) 回查; 未找到返回 ErrNotFound
// ctx: 上下文（未使用）
// userID: 用户ID
// traceID: 链路追踪 ID
func (s *MemoryStore) GetByUserAndTrace(_ context.Context, userID int64, traceID string) (entity.Order, error) {
	if traceID == "" {
		return entity.Order{}, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, o := range s.orders {
		if o.UserID == userID && o.TraceID == traceID {
			return o, nil
		}
	}
	return entity.Order{}, ErrNotFound
}

// ListOrdersByActivity 根据活动编号列出订单
// ctx: 上下文（未使用）
// activityNo: 活动编号
// 返回订单列表和错误
func (s *MemoryStore) ListOrdersByActivity(_ context.Context, activityNo string) ([]entity.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	orders := make([]entity.Order, 0)
	for _, order := range s.orders {
		if order.ActivityNo == activityNo {
			orders = append(orders, order)
		}
	}
	sortOrdersByCreateTimeDesc(orders)
	return orders, nil
}

// ListOrdersByActivities 根据多个活动编号批量列出订单
// ctx: 上下文（未使用）
// activityNos: 活动编号列表
// 返回按活动编号分组的订单映射和错误
func (s *MemoryStore) ListOrdersByActivities(_ context.Context, activityNos []string) (map[string][]entity.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string][]entity.Order, len(activityNos))
	if len(activityNos) == 0 {
		return result, nil
	}
	// 构建需要查询的活动编号集合
	wanted := make(map[string]struct{}, len(activityNos))
	for _, activityNo := range activityNos {
		if activityNo == "" {
			continue
		}
		wanted[activityNo] = struct{}{}
		if _, exists := result[activityNo]; !exists {
			result[activityNo] = nil
		}
	}
	// 遍历所有订单，分组到对应活动
	for _, order := range s.orders {
		if _, ok := wanted[order.ActivityNo]; ok {
			result[order.ActivityNo] = append(result[order.ActivityNo], order)
		}
	}
	// 对每个活动的订单按创建时间降序排序
	for activityNo := range result {
		sortOrdersByCreateTimeDesc(result[activityNo])
	}
	return result, nil
}

// ListOrdersByUser 根据用户ID列出订单
// ctx: 上下文（未使用）
// userID: 用户ID
// 返回订单列表和错误
func (s *MemoryStore) ListOrdersByUser(_ context.Context, userID int64) ([]entity.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	orders := make([]entity.Order, 0)
	for _, order := range s.orders {
		if order.UserID == userID {
			orders = append(orders, order)
		}
	}
	sortOrdersByCreateTimeDesc(orders)
	return orders, nil
}

// MarkOrderPaid 标记订单为已支付
// ctx: 上下文（未使用）
// orderNo: 订单号
// transactionNo: 交易流水号
// paidAt: 支付时间
// 返回错误表示标记失败
func (s *MemoryStore) MarkOrderPaid(_ context.Context, orderNo string, transactionNo string, paidAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderNo]
	if !ok {
		return ErrNotFound
	}
	// 使用状态机进行状态转移
	updated, ok := statemachine.TransitOrderPaid(order, transactionNo, paidAt)
	if !ok {
		return ErrInvalidState
	}
	s.orders[orderNo] = updated
	return nil
}

// CloseOrder 关闭订单
// ctx: 上下文（未使用）
// orderNo: 订单号
// 返回错误表示关闭失败
func (s *MemoryStore) CloseOrder(_ context.Context, orderNo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderNo]
	if !ok {
		return ErrNotFound
	}
	// 使用状态机进行状态转移
	now := time.Now()
	updated, ok := statemachine.TransitOrderClosed(order, now)
	if !ok {
		return ErrInvalidState
	}
	s.orders[orderNo] = updated
	return nil
}

// sortOrdersByCreateTimeDesc 按创建时间降序排序订单
// 如果创建时间相同，则按订单号降序排序
func sortOrdersByCreateTimeDesc(orders []entity.Order) {
	sort.Slice(orders, func(i, j int) bool {
		if orders[i].CreatedAt.Equal(orders[j].CreatedAt) {
			return orders[i].OrderNo > orders[j].OrderNo
		}
		return orders[i].CreatedAt.After(orders[j].CreatedAt)
	})
}
