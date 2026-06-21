package persistence

import (
	"context"
	"sync"
)

// MemoryDB 内存数据库实现，并发安全，支持幂等写入
type MemoryDB struct {
	mu         sync.Mutex
	deductions map[string]StockDeduction
	releases   map[string]StockRelease
}

// NewMemoryDB 创建内存数据库
func NewMemoryDB() *MemoryDB {
	return &MemoryDB{
		deductions: make(map[string]StockDeduction),
		releases:   make(map[string]StockRelease),
	}
}

// InsertStockDeduction 幂等写入扣减记录，key 为 orderNo:skuNo
func (m *MemoryDB) InsertStockDeduction(_ context.Context, deduction StockDeduction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := deduction.OrderNo + ":" + deduction.SKUNo
	if _, exists := m.deductions[key]; exists {
		return ErrDuplicate
	}
	m.deductions[key] = deduction
	return nil
}

// InsertStockRelease 幂等写入释放记录，key 为 orderNo:skuNo
func (m *MemoryDB) InsertStockRelease(_ context.Context, release StockRelease) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := release.OrderNo + ":" + release.SKUNo
	if _, exists := m.releases[key]; exists {
		return ErrDuplicate
	}
	m.releases[key] = release
	return nil
}
