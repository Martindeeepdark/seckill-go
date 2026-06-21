// Package persistence 提供风险控制的持久化层实现
package persistence

import (
	"context"
	"sort"
	"sync"
	"time"

	"seckill-risk-service/internal/domain/entity"
)

// MemoryStore 内存存储实现，用于开发测试
type MemoryStore struct {
	mu sync.Mutex // 保护并发访问

	riskUsers   map[int64]time.Time           // 风险用户及其过期时间
	riskRecords map[int64][]entity.RiskRecord // 用户风险行为记录
}

// NewMemoryStore 创建新的内存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		riskUsers:   map[int64]time.Time{},
		riskRecords: map[int64][]entity.RiskRecord{},
	}
}

// MarkRiskUser 标记用户为风险用户，ttl 指定过期时长
func (s *MemoryStore) MarkRiskUser(_ context.Context, userID int64, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.riskUsers[userID] = time.Now().Add(ttl)
	return nil
}

// IsRiskUser 检查用户是否为风险用户
func (s *MemoryStore) IsRiskUser(_ context.Context, userID int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	expiresAt, ok := s.riskUsers[userID]
	if !ok || expired(expiresAt) {
		delete(s.riskUsers, userID)
		return false, nil
	}
	return true, nil
}

// RecordRiskAction 记录风险行为
func (s *MemoryStore) RecordRiskAction(_ context.Context, record entity.RiskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	s.riskRecords[record.UserID] = append(s.riskRecords[record.UserID], record)
	return nil
}

// CountRecentRiskActions 统计指定时间以来的风险行为数量
func (s *MemoryStore) CountRecentRiskActions(_ context.Context, userID int64, actionType string, since time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, record := range s.riskRecords[userID] {
		if record.ActionType == actionType && !record.CreatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

// HasHighRiskRecord 检查是否存在高风险记录
func (s *MemoryStore) HasHighRiskRecord(_ context.Context, userID int64, since time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range s.riskRecords[userID] {
		if record.RiskLevel >= entity.RiskLevelHigh && !record.CreatedAt.Before(since) {
			return true, nil
		}
	}
	return false, nil
}

// ListRiskRecords 列出用户的风险行为记录，按创建时间倒序
func (s *MemoryStore) ListRiskRecords(_ context.Context, userID int64) ([]entity.RiskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := append([]entity.RiskRecord(nil), s.riskRecords[userID]...)
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	return records, nil
}

// CleanupExpiredRiskUsers 清理过期的风险用户，返回清理数量
func (s *MemoryStore) CleanupExpiredRiskUsers(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	deleted := 0
	for userID, expiresAt := range s.riskUsers {
		if !expiresAt.IsZero() && !expiresAt.After(now) {
			delete(s.riskUsers, userID)
			deleted++
		}
	}
	return deleted, nil
}

// expired 检查过期时间是否已过
func expired(expiresAt time.Time) bool {
	return !expiresAt.IsZero() && time.Now().After(expiresAt)
}
