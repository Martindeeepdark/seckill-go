package persistence

import (
	"context"
	"testing"
)

func TestMemoryStoreSeparatesStockAndPurchaseCleanup(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.stock["A1:S1"] = 10
	s.purchases[userPurchaseKey(7, "A1", "S1")] = 1

	deleted, err := s.CleanupActivityStock(ctx, "A1", []string{"S1"})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("stock deleted = %d, want 1", deleted)
	}
	purchaseKey := userPurchaseKey(7, "A1", "S1")
	if got := s.purchases[purchaseKey]; got != 1 {
		t.Fatalf("purchase count after stock cleanup = %d, want 1", got)
	}

	deleted, err = s.CleanupActivityPurchases(ctx, "A1")
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("purchase deleted = %d, want 1", deleted)
	}
	if _, ok := s.purchases[purchaseKey]; ok {
		t.Fatal("purchase count should be deleted by activity data cleanup")
	}
}

func TestMemoryStoreDeductAndRelease(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.stock["A1:S1"] = 100

	ok, err := s.DeductStockWithLimit(ctx, "A1", "S1", 1, 1, 1, "O1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("deduct should succeed")
	}
	if got := s.stock["A1:S1"]; got != 99 {
		t.Fatalf("stock after deduct = %d, want 99", got)
	}

	if err := s.ReleaseStock(ctx, "A1", "S1", 1, 1, "O1"); err != nil {
		t.Fatal(err)
	}
	if got := s.stock["A1:S1"]; got != 100 {
		t.Fatalf("stock after release = %d, want 100", got)
	}
}

func TestMemoryStoreDuctExceedsPurchaseLimit(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.stock["A1:S1"] = 100

	ok, err := s.DeductStockWithLimit(ctx, "A1", "S1", 1, 1, 1, "O1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("first deduct should succeed")
	}

	ok, err = s.DeductStockWithLimit(ctx, "A1", "S1", 1, 1, 1, "O2")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("second deduct should fail due to purchase limit")
	}
}

func TestMemoryStoreDeductInsufficientStock(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.stock["A1:S1"] = 1

	ok, err := s.DeductStockWithLimit(ctx, "A1", "S1", 1, 5, 0, "O1")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("deduct should fail due to insufficient stock")
	}
}

func TestMemoryStorePeekStockNotReady(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	_, err := s.PeekStock(ctx, "A1", "S1")
	if err != ErrStockNotReady {
		t.Fatalf("expected ErrStockNotReady, got %v", err)
	}
}

func TestMemoryStoreDeductIdempotentForSameOrder(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.stock["A1:S1"] = 10

	ok, err := s.DeductStockWithLimit(ctx, "A1", "S1", 1, 2, 0, "O1")
	if err != nil || !ok {
		t.Fatalf("first deduct: ok=%v err=%v", ok, err)
	}
	if got := s.stock["A1:S1"]; got != 8 {
		t.Fatalf("stock after first deduct = %d, want 8", got)
	}

	ok, err = s.DeductStockWithLimit(ctx, "A1", "S1", 1, 2, 0, "O1")
	if err != nil || !ok {
		t.Fatalf("duplicate deduct: ok=%v err=%v", ok, err)
	}
	if got := s.stock["A1:S1"]; got != 8 {
		t.Fatalf("stock after duplicate deduct = %d, want 8 (idempotent)", got)
	}
}

func TestMemoryStoreReleaseIdempotentForSameOrder(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.stock["A1:S1"] = 10

	if _, err := s.DeductStockWithLimit(ctx, "A1", "S1", 1, 3, 0, "O1"); err != nil {
		t.Fatal(err)
	}
	if got := s.stock["A1:S1"]; got != 7 {
		t.Fatalf("stock after deduct = %d, want 7", got)
	}

	if err := s.ReleaseStock(ctx, "A1", "S1", 1, 3, "O1"); err != nil {
		t.Fatal(err)
	}
	if got := s.stock["A1:S1"]; got != 10 {
		t.Fatalf("stock after release = %d, want 10", got)
	}

	if err := s.ReleaseStock(ctx, "A1", "S1", 1, 3, "O1"); err != nil {
		t.Fatal(err)
	}
	if got := s.stock["A1:S1"]; got != 10 {
		t.Fatalf("stock after duplicate release = %d, want 10 (idempotent)", got)
	}
}

func TestMemoryStoreReleaseNotReservedIsNoop(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.stock["A1:S1"] = 10

	if err := s.ReleaseStock(ctx, "A1", "S1", 1, 5, "O999"); err != nil {
		t.Fatal(err)
	}
	if got := s.stock["A1:S1"]; got != 10 {
		t.Fatalf("stock after releasing unreserved order = %d, want 10", got)
	}
}
