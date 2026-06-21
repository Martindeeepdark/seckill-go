package persistence

import (
	"context"
	"testing"
	"time"

	domain "seckill-activity-service/internal/domain/entity"
	statemachine "seckill-activity-service/internal/domain/status"
)

func TestMemoryStoreSeparatesStockAndPurchaseCleanup(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	activity := domain.Activity{
		ActivityNo:    "A1",
		Name:          "ended",
		StartTime:     time.Now().Add(-2 * time.Hour),
		EndTime:       time.Now().Add(-time.Hour),
		Status:        statemachine.ActivityEnded,
		PurchaseLimit: 1,
		SKUs: []domain.SKU{
			{SKUNo: "S1", TotalStock: 10, LimitQuantity: 1},
		},
	}
	if err := store.AddActivity(ctx, activity); err != nil {
		t.Fatal(err)
	}
	ok, err := store.DeductStockWithLimit(ctx, "A1", "S1", 7, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("deduct stock should succeed")
	}
	purchaseKey := userPurchaseKey(7, "A1", "S1")
	if got := store.purchases[purchaseKey]; got != 1 {
		t.Fatalf("purchase count = %d, want 1", got)
	}

	deleted, err := store.CleanupActivityStock(ctx, "A1", []string{"S1"})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("stock deleted = %d, want 1", deleted)
	}
	if got := store.purchases[purchaseKey]; got != 1 {
		t.Fatalf("purchase count after stock cleanup = %d, want 1", got)
	}

	deleted, err = store.CleanupActivityPurchases(ctx, "A1")
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("purchase deleted = %d, want 1", deleted)
	}
	if _, ok := store.purchases[purchaseKey]; ok {
		t.Fatal("purchase count should be deleted by activity data cleanup")
	}
}
