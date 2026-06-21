package rpc

import (
	"testing"
	"time"

	domain "seckill-activity-service/internal/domain/entity"
	statemachine "seckill-activity-service/internal/domain/status"
)

func TestActivityProtoRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 30, 0, 0, time.UTC)
	activity := domain.Activity{
		ActivityNo:    "A1001",
		Name:          "flash sale",
		StartTime:     now,
		EndTime:       now.Add(time.Hour),
		Status:        statemachine.ActivityOpen,
		PurchaseLimit: 2,
		Remark:        "limited",
		SKUs: []domain.SKU{
			{
				ActivityNo:    "A1001",
				SKUNo:         "SKU1001",
				ProductName:   "phone",
				ProductImage:  "phone.png",
				OriginalPrice: 699900,
				SeckillPrice:  599900,
				TotalStock:    100,
				LimitQuantity: 1,
				DiscountType:  1,
				DiscountPrice: 100000,
				DiscountPct:   8500,
			},
		},
		CreatedAt: now.Add(-time.Minute),
	}

	got := activityFromPB(activityToPB(activity))

	if got.ActivityNo != activity.ActivityNo ||
		got.Name != activity.Name ||
		!got.StartTime.Equal(activity.StartTime) ||
		!got.EndTime.Equal(activity.EndTime) ||
		got.Status != activity.Status ||
		got.PurchaseLimit != activity.PurchaseLimit ||
		got.Remark != activity.Remark ||
		!got.CreatedAt.Equal(activity.CreatedAt) ||
		!got.UpdatedAt.IsZero() {
		t.Fatalf("activity round trip mismatch: got %+v, want %+v", got, activity)
	}
	if len(got.SKUs) != 1 {
		t.Fatalf("sku count = %d, want 1", len(got.SKUs))
	}
	if got.SKUs[0] != activity.SKUs[0] {
		t.Fatalf("sku round trip = %+v, want %+v", got.SKUs[0], activity.SKUs[0])
	}
}
