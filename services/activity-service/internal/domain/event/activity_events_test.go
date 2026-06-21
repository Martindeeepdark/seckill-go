package event

import (
	"testing"
	"time"
)

func TestActivityStartedEvent(t *testing.T) {
	// Given
	activityNo := "ACT-001"
	startedAt := time.Now()

	// When
	event := NewActivityStartedEvent(activityNo, startedAt)

	// Then
	if event.EventName() != "activity.started" {
		t.Errorf("expected event name 'activity.started', got %s", event.EventName())
	}
	if event.AggregateID() != activityNo {
		t.Errorf("expected aggregate ID %s, got %s", activityNo, event.AggregateID())
	}
	if event.ActivityNo != activityNo {
		t.Errorf("expected activity no %s, got %s", activityNo, event.ActivityNo)
	}
	if !event.StartedAt.Equal(startedAt) {
		t.Errorf("expected started at %v, got %v", startedAt, event.StartedAt)
	}
}

func TestActivityEndedEvent(t *testing.T) {
	// Given
	activityNo := "ACT-001"
	reason := "time_expired"
	endedAt := time.Now()

	// When
	event := NewActivityEndedEvent(activityNo, reason, endedAt)

	// Then
	if event.EventName() != "activity.ended" {
		t.Errorf("expected event name 'activity.ended', got %s", event.EventName())
	}
	if event.Reason != reason {
		t.Errorf("expected reason %s, got %s", reason, event.Reason)
	}
}

func TestSKUAddedEvent(t *testing.T) {
	// Given
	activityNo := "ACT-001"
	skuNo := "SKU-001"
	stock := int64(100)
	price := int64(9900)

	// When
	event := NewSKUAddedEvent(activityNo, skuNo, stock, price)

	// Then
	if event.EventName() != "activity.sku.added" {
		t.Errorf("expected event name 'activity.sku.added', got %s", event.EventName())
	}
	if event.SKUNo != skuNo {
		t.Errorf("expected sku no %s, got %s", skuNo, event.SKUNo)
	}
	if event.Stock != stock {
		t.Errorf("expected stock %d, got %d", stock, event.Stock)
	}
	if event.Price != price {
		t.Errorf("expected price %d, got %d", price, event.Price)
	}
}

func TestSKURemovedEvent(t *testing.T) {
	// Given
	activityNo := "ACT-001"
	skuNo := "SKU-001"
	reason := "out_of_stock"

	// When
	event := NewSKURemovedEvent(activityNo, skuNo, reason)

	// Then
	if event.EventName() != "activity.sku.removed" {
		t.Errorf("expected event name 'activity.sku.removed', got %s", event.EventName())
	}
	if event.SKUNo != skuNo {
		t.Errorf("expected sku no %s, got %s", skuNo, event.SKUNo)
	}
	if event.Reason != reason {
		t.Errorf("expected reason %s, got %s", reason, event.Reason)
	}
}
