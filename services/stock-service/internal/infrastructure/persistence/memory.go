package persistence

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// MemoryStore 内存库存存储实现
type MemoryStore struct {
	mu sync.Mutex

	stock     map[string]int64  // SKU库存映射
	purchases map[string]int64  // 用户购买数量映射
	reserved  map[string]string // 订单预留映射
}

// NewMemoryStore 创建内存库存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		stock:     map[string]int64{},
		purchases: map[string]int64{},
		reserved:  map[string]string{},
	}
}

// PeekStock 查询库存
func (s *MemoryStore) PeekStock(_ context.Context, activityNo, skuNo string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stock, ok := s.stock[skuKey(activityNo, skuNo)]
	if !ok {
		return 0, ErrStockNotReady
	}
	return stock, nil
}

// DeductStockWithLimit 扣减库存（支持购买限制）
func (s *MemoryStore) DeductStockWithLimit(_ context.Context, activityNo, skuNo string, userID int64, quantity int64, purchaseLimit int64, orderNo string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := skuKey(activityNo, skuNo)
	reserveKey := reservedKey(activityNo, skuNo, orderNo)
	// 检查是否已预留（幂等处理）
	if _, exists := s.reserved[reserveKey]; exists {
		return true, nil
	}
	// 检查库存是否存在
	current, ok := s.stock[key]
	if !ok {
		return false, ErrStockNotReady
	}
	// 检查用户购买限制
	purchaseKey := userPurchaseKey(userID, activityNo, skuNo)
	if purchaseLimit > 0 && s.purchases[purchaseKey]+quantity > purchaseLimit {
		return false, nil
	}
	// 检查库存是否充足
	if current < quantity {
		return false, nil
	}
	// 扣减库存
	s.stock[key] = current - quantity
	if purchaseLimit > 0 {
		s.purchases[purchaseKey] += quantity
	}
	s.reserved[reserveKey] = "1"
	return true, nil
}

// ReleaseStock 释放库存
func (s *MemoryStore) ReleaseStock(_ context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	reserveKey := reservedKey(activityNo, skuNo, orderNo)
	// 检查是否有预留记录
	if _, exists := s.reserved[reserveKey]; !exists {
		return nil
	}
	// 释放预留
	delete(s.reserved, reserveKey)
	s.stock[skuKey(activityNo, skuNo)] += quantity
	// 减少用户购买数量
	purchaseKey := userPurchaseKey(userID, activityNo, skuNo)
	if s.purchases[purchaseKey] <= quantity {
		delete(s.purchases, purchaseKey)
		return nil
	}
	s.purchases[purchaseKey] -= quantity
	return nil
}

// CleanupActivityStock 清理活动库存
func (s *MemoryStore) CleanupActivityStock(_ context.Context, activityNo string, skuNos []string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := int64(0)
	if len(skuNos) > 0 {
		// 按指定的SKU清理
		for _, skuNo := range skuNos {
			key := skuKey(activityNo, skuNo)
			if _, ok := s.stock[key]; ok {
				delete(s.stock, key)
				deleted++
			}
		}
	} else {
		// 按活动号前缀清理
		prefix := activityNo + ":"
		for key := range s.stock {
			if strings.HasPrefix(key, prefix) {
				delete(s.stock, key)
				deleted++
			}
		}
	}
	return deleted, nil
}

// CleanupActivityPurchases 清理活动购买记录
func (s *MemoryStore) CleanupActivityPurchases(_ context.Context, activityNo string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := int64(0)
	activitySegment := ":" + activityNo + ":"
	for key := range s.purchases {
		if strings.Contains(key, activitySegment) {
			delete(s.purchases, key)
			deleted++
		}
	}
	return deleted, nil
}

// skuKey 生成SKU库存键
func skuKey(activityNo, skuNo string) string {
	return activityNo + ":" + skuNo
}

// userPurchaseKey 生成用户购买记录键
func userPurchaseKey(userID int64, activityNo, skuNo string) string {
	return fmt.Sprintf("%d:%s:%s", userID, activityNo, skuNo)
}

// reservedKey 生成预留记录键
func reservedKey(activityNo, skuNo, orderNo string) string {
	return fmt.Sprintf("%s:%s:%s", activityNo, skuNo, orderNo)
}
