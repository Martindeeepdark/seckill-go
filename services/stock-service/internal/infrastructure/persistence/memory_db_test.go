package persistence

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryDB_InsertStockDeduction_Success(t *testing.T) {
	db := NewMemoryDB()
	ctx := context.Background()

	deduction := StockDeduction{
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		OrderNo:    "ORD001",
		UserID:     1001,
		Quantity:   2,
		CreatedAt:  time.Now(),
	}

	err := db.InsertStockDeduction(ctx, deduction)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMemoryDB_InsertStockDeduction_Duplicate(t *testing.T) {
	db := NewMemoryDB()
	ctx := context.Background()

	deduction := StockDeduction{
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		OrderNo:    "ORD001",
		UserID:     1001,
		Quantity:   2,
		CreatedAt:  time.Now(),
	}

	// First insert should succeed
	err := db.InsertStockDeduction(ctx, deduction)
	if err != nil {
		t.Fatalf("first insert should succeed, got %v", err)
	}

	// Second insert with same orderNo:skuNo should return ErrDuplicate
	err = db.InsertStockDeduction(ctx, deduction)
	if err != ErrDuplicate {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}
}

func TestMemoryDB_InsertStockDeduction_DifferentKeys(t *testing.T) {
	db := NewMemoryDB()
	ctx := context.Background()

	deduction1 := StockDeduction{
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		OrderNo:    "ORD001",
		UserID:     1001,
		Quantity:   2,
		CreatedAt:  time.Now(),
	}

	deduction2 := StockDeduction{
		ActivityNo: "ACT001",
		SKUNo:      "SKU002",
		OrderNo:    "ORD001",
		UserID:     1001,
		Quantity:   1,
		CreatedAt:  time.Now(),
	}

	err := db.InsertStockDeduction(ctx, deduction1)
	if err != nil {
		t.Fatalf("first insert should succeed, got %v", err)
	}

	// Different SKU should succeed (different key: ORD001:SKU002)
	err = db.InsertStockDeduction(ctx, deduction2)
	if err != nil {
		t.Fatalf("second insert with different key should succeed, got %v", err)
	}
}

func TestMemoryDB_InsertStockRelease_Success(t *testing.T) {
	db := NewMemoryDB()
	ctx := context.Background()

	release := StockRelease{
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		OrderNo:    "ORD001",
		UserID:     1001,
		Quantity:   1,
		CreatedAt:  time.Now(),
	}

	err := db.InsertStockRelease(ctx, release)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMemoryDB_InsertStockRelease_Duplicate(t *testing.T) {
	db := NewMemoryDB()
	ctx := context.Background()

	release := StockRelease{
		ActivityNo: "ACT001",
		SKUNo:      "SKU001",
		OrderNo:    "ORD001",
		UserID:     1001,
		Quantity:   1,
		CreatedAt:  time.Now(),
	}

	// First insert should succeed
	err := db.InsertStockRelease(ctx, release)
	if err != nil {
		t.Fatalf("first insert should succeed, got %v", err)
	}

	// Second insert with same orderNo:skuNo should return ErrDuplicate
	err = db.InsertStockRelease(ctx, release)
	if err != ErrDuplicate {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}
}

func TestMemoryDB_ConcurrentInsert(t *testing.T) {
	db := NewMemoryDB()
	ctx := context.Background()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errors := make([]error, goroutines)
	var errMu sync.Mutex

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			deduction := StockDeduction{
				ActivityNo: "ACT001",
				SKUNo:      "SKU001",
				OrderNo:    "ORD001",
				UserID:     int64(idx),
				Quantity:   1,
				CreatedAt:  time.Now(),
			}

			err := db.InsertStockDeduction(ctx, deduction)
			errMu.Lock()
			errors[idx] = err
			errMu.Unlock()
		}(i)
	}

	wg.Wait()

	// Exactly one goroutine should succeed, the rest should get ErrDuplicate
	successCount := 0
	duplicateCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		} else if err == ErrDuplicate {
			duplicateCount++
		} else {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
	if duplicateCount != goroutines-1 {
		t.Errorf("expected %d duplicates, got %d", goroutines-1, duplicateCount)
	}
}

func TestMemoryDB_ImplementsDBInterface(t *testing.T) {
	// Compile-time check that MemoryDB implements DB interface
	var _ DB = (*MemoryDB)(nil)
}
