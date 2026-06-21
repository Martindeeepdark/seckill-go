// Package persistence 提供内存存储实现
package persistence

import (
	"context"
	"sort"
	"sync"
	"time"

	"seckill-order-service/internal/domain/entity"
)

// MemoryOrderRepository 内存订单仓储，用于测试
type MemoryOrderRepository struct {
	mu     sync.RWMutex
	orders map[string]*entity.Order
}

// NewMemoryOrderRepository 创建内存订单仓储
func NewMemoryOrderRepository() *MemoryOrderRepository {
	return &MemoryOrderRepository{
		orders: make(map[string]*entity.Order),
	}
}

// Save 保存订单
func (r *MemoryOrderRepository) Save(_ context.Context, order *entity.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.orders[order.OrderNo] = order
	return nil
}

// GetByOrderNo 根据订单号获取订单
func (r *MemoryOrderRepository) GetByOrderNo(_ context.Context, orderNo string) (*entity.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, ok := r.orders[orderNo]
	if !ok {
		return nil, ErrNotFound
	}
	return order, nil
}

// GetByUserID 根据用户ID获取订单列表
func (r *MemoryOrderRepository) GetByUserID(_ context.Context, userID int64) ([]*entity.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var orders []*entity.Order
	for _, order := range r.orders {
		if order.UserID == userID {
			orders = append(orders, order)
		}
	}

	// 按创建时间降序排序
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].CreatedAt.After(orders[j].CreatedAt)
	})

	return orders, nil
}

// GetPendingOrders 获取待支付订单
func (r *MemoryOrderRepository) GetPendingOrders(_ context.Context, beforeTime time.Time, limit int) ([]*entity.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var orders []*entity.Order
	for _, order := range r.orders {
		if order.Status == entity.OrderPending && order.CreatedAt.Before(beforeTime) {
			orders = append(orders, order)
			if len(orders) >= limit {
				break
			}
		}
	}

	return orders, nil
}
