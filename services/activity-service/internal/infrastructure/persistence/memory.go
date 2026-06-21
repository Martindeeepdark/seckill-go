// Package persistence 提供基于内存的仓储实现，用于开发和测试。
package persistence

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	domain "seckill-activity-service/internal/domain/entity"
	statemachine "seckill-activity-service/internal/domain/status"
)

// expiringValue 表示带过期时间的值。
type expiringValue struct {
	value     string
	expiresAt time.Time
}

// MemoryStore 是基于内存的仓储实现。
type MemoryStore struct {
	mu sync.Mutex

	activities map[string]domain.Activity
	skus       map[string]domain.SKU
	stock      map[string]int64
	purchases  map[string]int64

	traceResults map[string]expiringValue
}

// NewMemoryStore 创建内存仓储实例。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		activities:   map[string]domain.Activity{},
		skus:         map[string]domain.SKU{},
		stock:        map[string]int64{},
		purchases:    map[string]int64{},
		traceResults: map[string]expiringValue{},
	}
}

// SampleActivity 返回用于演示的示例活动。
func SampleActivity(now time.Time) domain.Activity {
	return domain.Activity{
		ActivityNo:    "1001",
		Name:          "Go 秒杀演示活动",
		StartTime:     now.Add(-time.Minute),
		EndTime:       now.Add(time.Hour),
		Status:        statemachine.ActivityOpen,
		PurchaseLimit: 1,
		Remark:        "本地默认活动，可直接用 X-User-ID 体验接口",
		SKUs: []domain.SKU{
			{
				ActivityNo:    "1001",
				SKUNo:         "2001",
				ProductName:   "高并发实战课",
				ProductImage:  "https://example.com/seckill-course.png",
				OriginalPrice: 19900,
				SeckillPrice:  9900,
				TotalStock:    100,
				LimitQuantity: 1,
			},
		},
	}
}

// AddActivity 添加活动，自动初始化库存。
func (s *MemoryStore) AddActivity(_ context.Context, activity domain.Activity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if activity.CreatedAt.IsZero() {
		activity.CreatedAt = now
	}
	activity.UpdatedAt = now
	s.activities[activity.ActivityNo] = activity
	for _, sku := range activity.SKUs {
		sku.ActivityNo = activity.ActivityNo
		key := skuKey(activity.ActivityNo, sku.SKUNo)
		s.skus[key] = sku
		if _, exists := s.stock[key]; !exists {
			s.stock[key] = sku.TotalStock
		}
	}
	return nil
}

// UpdateActivity 更新活动基本信息。
func (s *MemoryStore) UpdateActivity(_ context.Context, activity domain.Activity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.activities[activity.ActivityNo]
	if !ok {
		return ErrNotFound
	}
	if activity.Name != "" {
		existing.Name = activity.Name
	}
	if !activity.StartTime.IsZero() {
		existing.StartTime = activity.StartTime
	}
	if !activity.EndTime.IsZero() {
		existing.EndTime = activity.EndTime
	}
	if activity.PurchaseLimit > 0 {
		existing.PurchaseLimit = activity.PurchaseLimit
	}
	if activity.Remark != "" {
		existing.Remark = activity.Remark
	}
	existing.UpdatedAt = time.Now()
	s.activities[activity.ActivityNo] = existing
	return nil
}

// UpdateActivityStatus 更新活动状态，校验状态流转规则。
func (s *MemoryStore) UpdateActivityStatus(_ context.Context, activityNo string, status int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	activity, ok := s.activities[activityNo]
	if !ok {
		return ErrNotFound
	}
	updated, ok := statemachine.TransitActivity(activity, status, time.Now())
	if !ok {
		return ErrInvalidState
	}
	activity = updated
	s.activities[activityNo] = activity
	return nil
}

// AddActivitySKU 向活动添加商品，若库存未初始化则初始化。
func (s *MemoryStore) AddActivitySKU(_ context.Context, activityNo string, sku domain.SKU) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	activity, ok := s.activities[activityNo]
	if !ok {
		return ErrNotFound
	}
	sku.ActivityNo = activityNo
	key := skuKey(activityNo, sku.SKUNo)
	s.skus[key] = sku
	if _, exists := s.stock[key]; !exists {
		s.stock[key] = sku.TotalStock
	}
	replaced := false
	for i := range activity.SKUs {
		if activity.SKUs[i].SKUNo == sku.SKUNo {
			activity.SKUs[i] = sku
			replaced = true
			break
		}
	}
	if !replaced {
		activity.SKUs = append(activity.SKUs, sku)
	}
	activity.UpdatedAt = time.Now()
	s.activities[activityNo] = activity
	return nil
}

// RemoveActivitySKU 从活动移除商品并清理库存。
func (s *MemoryStore) RemoveActivitySKU(_ context.Context, activityNo, skuNo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	activity, ok := s.activities[activityNo]
	if !ok {
		return ErrNotFound
	}
	key := skuKey(activityNo, skuNo)
	if _, ok := s.skus[key]; !ok {
		return ErrNotFound
	}
	delete(s.skus, key)
	delete(s.stock, key)
	skus := activity.SKUs[:0]
	for _, sku := range activity.SKUs {
		if sku.SKUNo != skuNo {
			skus = append(skus, sku)
		}
	}
	activity.SKUs = skus
	activity.UpdatedAt = time.Now()
	s.activities[activityNo] = activity
	return nil
}

// ListActivities 列出所有活动，并附加实时库存。
func (s *MemoryStore) ListActivities(_ context.Context) ([]domain.Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	activities := make([]domain.Activity, 0, len(s.activities))
	for _, activity := range s.activities {
		activities = append(activities, s.attachRuntimeStockLocked(activity))
	}
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].ActivityNo < activities[j].ActivityNo
	})
	return activities, nil
}

// GetActivity 获取单个活动，并附加实时库存。
func (s *MemoryStore) GetActivity(_ context.Context, activityNo string) (domain.Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	activity, ok := s.activities[activityNo]
	if !ok {
		return domain.Activity{}, ErrNotFound
	}
	return s.attachRuntimeStockLocked(activity), nil
}

// GetSKU 获取活动商品。
func (s *MemoryStore) GetSKU(_ context.Context, activityNo, skuNo string) (domain.SKU, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sku, ok := s.skus[skuKey(activityNo, skuNo)]
	if !ok {
		return domain.SKU{}, ErrNotFound
	}
	return sku, nil
}

// PeekStock 查看当前库存。
func (s *MemoryStore) PeekStock(_ context.Context, activityNo, skuNo string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stock, ok := s.stock[skuKey(activityNo, skuNo)]
	if !ok {
		return 0, ErrStockNotReady
	}
	return stock, nil
}

// DeductStockWithLimit 扣减库存并校验限购。
func (s *MemoryStore) DeductStockWithLimit(_ context.Context, activityNo, skuNo string, userID int64, quantity int64, purchaseLimit int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := skuKey(activityNo, skuNo)
	current, ok := s.stock[key]
	if !ok {
		return false, ErrStockNotReady
	}
	purchaseKey := userPurchaseKey(userID, activityNo, skuNo)
	if purchaseLimit > 0 && s.purchases[purchaseKey]+quantity > purchaseLimit {
		return false, nil
	}
	if current < int64(quantity) {
		return false, nil
	}
	s.stock[key] = current - int64(quantity)
	if purchaseLimit > 0 {
		s.purchases[purchaseKey] += quantity
	}
	return true, nil
}

// ReleaseStock 释放库存并回退限购计数。
func (s *MemoryStore) ReleaseStock(_ context.Context, activityNo, skuNo string, userID int64, quantity int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stock[skuKey(activityNo, skuNo)] += int64(quantity)
	purchaseKey := userPurchaseKey(userID, activityNo, skuNo)
	if s.purchases[purchaseKey] <= quantity {
		delete(s.purchases, purchaseKey)
		return nil
	}
	s.purchases[purchaseKey] -= quantity
	return nil
}

// CleanupActivityStock 清理活动库存，可指定 SKU 列表或清理全部。
func (s *MemoryStore) CleanupActivityStock(_ context.Context, activityNo string, skuNos []string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := int64(0)
	if len(skuNos) > 0 {
		for _, skuNo := range skuNos {
			key := skuKey(activityNo, skuNo)
			if _, ok := s.stock[key]; ok {
				delete(s.stock, key)
				deleted++
			}
		}
	} else {
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

// CleanupActivityPurchases 清理活动的用户购买记录。
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

// TryStartTrace 尝试启动异步链路追踪，返回是否成功启动。
func (s *MemoryStore) TryStartTrace(_ context.Context, traceID string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, exists := s.traceResults[traceID]
	if exists && !expired(value.expiresAt) {
		return false, nil
	}
	s.traceResults[traceID] = expiringValue{value: TraceProcessing, expiresAt: time.Now().Add(ttl)}
	return true, nil
}

// MarkTraceSuccess 标记链路追踪成功，写入订单号。
func (s *MemoryStore) MarkTraceSuccess(_ context.Context, traceID, orderNo string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traceResults[traceID] = expiringValue{value: orderNo, expiresAt: time.Now().Add(ttl)}
	return nil
}

// MarkTraceFail 标记链路追踪失败，写入失败原因。
func (s *MemoryStore) MarkTraceFail(_ context.Context, traceID, reason string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traceResults[traceID] = expiringValue{value: reason, expiresAt: time.Now().Add(ttl)}
	return nil
}

// GetTraceResult 获取链路追踪结果。
func (s *MemoryStore) GetTraceResult(_ context.Context, traceID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.traceResults[traceID]
	if !ok || expired(value.expiresAt) {
		delete(s.traceResults, traceID)
		return "", ErrNotFound
	}
	return value.value, nil
}

// DeleteTrace 删除链路追踪记录。
func (s *MemoryStore) DeleteTrace(_ context.Context, traceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.traceResults, traceID)
	return nil
}

// attachRuntimeStockLocked 将实时库存附加到活动的 SKU 上，必须在持有锁时调用。
func (s *MemoryStore) attachRuntimeStockLocked(activity domain.Activity) domain.Activity {
	for i := range activity.SKUs {
		activity.SKUs[i].TotalStock = s.stock[skuKey(activity.ActivityNo, activity.SKUs[i].SKUNo)]
	}
	return activity
}

// expired 判断是否已过期。
func expired(expiresAt time.Time) bool {
	return !expiresAt.IsZero() && time.Now().After(expiresAt)
}

// skuKey 生成 SKU 唯一键。
func skuKey(activityNo, skuNo string) string {
	return activityNo + ":" + skuNo
}

// userPurchaseKey 生成用户购买记录键。
func userPurchaseKey(userID int64, activityNo, skuNo string) string {
	return fmt.Sprintf("%d:%s:%s", userID, activityNo, skuNo)
}
