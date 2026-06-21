package persistence

import (
	"context"
	"sync"
)

// MockDB 内存数据库实现（用于演示和测试）
type MockDB struct {
	mu sync.Mutex

	deductions map[string]StockDeduction
	releases   map[string]StockRelease

	// 可选的函数覆盖，用于测试
	InsertStockDeductionFunc func(ctx context.Context, deduction StockDeduction) error
	InsertStockReleaseFunc   func(ctx context.Context, release StockRelease) error

	// 调用记录追踪
	insertDeductionCalled bool
	lastDeduction         StockDeduction
	insertReleaseCalled   bool
	lastRelease           StockRelease
}

// NewMockDB 创建内存数据库
func NewMockDB() *MockDB {
	return &MockDB{
		deductions: make(map[string]StockDeduction),
		releases:   make(map[string]StockRelease),
	}
}

// InsertStockDeduction 插入库存扣减记录
func (m *MockDB) InsertStockDeduction(ctx context.Context, deduction StockDeduction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.InsertStockDeductionFunc != nil {
		m.insertDeductionCalled = true
		m.lastDeduction = deduction
		return m.InsertStockDeductionFunc(ctx, deduction)
	}

	key := deduction.OrderNo + ":" + deduction.SKUNo
	if _, exists := m.deductions[key]; exists {
		return ErrDuplicate
	}
	m.deductions[key] = deduction
	m.insertDeductionCalled = true
	m.lastDeduction = deduction
	return nil
}

// InsertStockRelease 插入库存释放记录
func (m *MockDB) InsertStockRelease(ctx context.Context, release StockRelease) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.InsertStockReleaseFunc != nil {
		m.insertReleaseCalled = true
		m.lastRelease = release
		return m.InsertStockReleaseFunc(ctx, release)
	}

	key := release.OrderNo + ":" + release.SKUNo
	if _, exists := m.releases[key]; exists {
		return ErrDuplicate
	}
	m.releases[key] = release
	m.insertReleaseCalled = true
	m.lastRelease = release
	return nil
}

// GetDeductionCount 获取扣减记录数量（用于测试）
func (m *MockDB) GetDeductionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.deductions)
}

// GetReleaseCount 获取释放记录数量（用于测试）
func (m *MockDB) GetReleaseCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.releases)
}

// InsertDeductionCalled 返回是否调用了 InsertStockDeduction
func (m *MockDB) InsertDeductionCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.insertDeductionCalled
}

// LastDeduction 返回最后一次插入的扣减记录
func (m *MockDB) LastDeduction() StockDeduction {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastDeduction
}
