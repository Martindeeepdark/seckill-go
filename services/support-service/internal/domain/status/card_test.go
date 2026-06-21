package status

import (
	"testing"
	"time"

	"seckill-support-service/internal/domain/entity"
)

func TestCanCardTransitionTo(t *testing.T) {
	tests := []struct{ from, to int64; want bool }{
		{CardInactive, CardActive, true},
		{CardActive, CardFrozen, true},
		{CardExpired, CardActive, false},
		{CardInactive, CardInactive, false},
	}
	for _, tt := range tests {
		if got := CanCardTransitionTo(tt.from, tt.to); got != tt.want {
			t.Errorf("CanCardTransitionTo(%d,%d)=%v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestActivateCardSetsValidity(t *testing.T) {
	now := time.Date(2026, 6, 6, 11, 0, 0, 0, time.UTC)
	active, ok := TransitCardActive(entity.FreeCard{CardNo: "FC1", Status: CardInactive, ValidDays: 30}, now)
	if !ok { t.Fatal("should transit to active") }
	if active.Status != CardActive || active.ExpireAt == nil { t.Fatal("bad active state") }
}
