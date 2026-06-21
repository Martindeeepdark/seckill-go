// Package entity tests the Activity aggregate root.
package entity

import (
	"errors"
	"testing"
	"time"

	"seckill-common/domain"
	"seckill-activity-service/internal/domain/event"
)

func TestActivity_Start(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name        string
		activity    *Activity
		startTime   time.Time
		wantErr     error
		wantStatus  int64
		wantEvent   bool
	}{
		{
			name: "start pending activity successfully",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityPending,
				StartTime:  past,
				EndTime:    future,
			},
			startTime:  now,
			wantErr:    nil,
			wantStatus: ActivityOpen,
			wantEvent:  true,
		},
		{
			name: "fail to start already open activity",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityOpen,
				StartTime:  past,
				EndTime:    future,
			},
			startTime:  now,
			wantErr:    errors.New("activity is already open"),
			wantStatus: ActivityOpen,
			wantEvent:  false,
		},
		{
			name: "fail to start ended activity",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityEnded,
				StartTime:  past,
				EndTime:    future,
			},
			startTime:  now,
			wantErr:    errors.New("activity has ended, cannot start"),
			wantStatus: ActivityEnded,
			wantEvent:  false,
		},
		{
			name: "fail to start activity before scheduled time",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityPending,
				StartTime:  future.Add(1 * time.Hour),
				EndTime:    future.Add(2 * time.Hour),
			},
			startTime:  now,
			wantErr:    errors.New("activity cannot start before scheduled time"),
			wantStatus: ActivityPending,
			wantEvent:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Embed AggregateRoot to enable event recording
			aggregate := &domain.AggregateRoot{}
			tt.activity.AggregateRoot = aggregate

			err := tt.activity.Start(tt.startTime)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Start() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("Start() expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("Start() unexpected error: %v", err)
			}

			if tt.activity.Status != tt.wantStatus {
				t.Errorf("Start() expected status %d, got %d", tt.wantStatus, tt.activity.Status)
			}

			events := tt.activity.GetDomainEvents()
			if tt.wantEvent {
				if len(events) != 1 {
					t.Errorf("Start() expected 1 event, got %d", len(events))
				} else {
					startedEvent, ok := events[0].(*event.ActivityStartedEvent)
					if !ok {
						t.Errorf("Start() expected ActivityStartedEvent, got %T", events[0])
					} else if startedEvent.ActivityNo != tt.activity.ActivityNo {
						t.Errorf("Start() expected ActivityNo %s, got %s", tt.activity.ActivityNo, startedEvent.ActivityNo)
					}
				}
			} else if len(events) != 0 {
				t.Errorf("Start() expected 0 events, got %d", len(events))
			}
		})
	}
}

func TestActivity_End(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		activity    *Activity
		reason      string
		endTime     time.Time
		wantErr     error
		wantStatus  int64
		wantEvent   bool
	}{
		{
			name: "end open activity successfully",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityOpen,
				StartTime:  now.Add(-1 * time.Hour),
				EndTime:    now.Add(1 * time.Hour),
			},
			reason:     "manual end",
			endTime:    now,
			wantErr:    nil,
			wantStatus: ActivityEnded,
			wantEvent:  true,
		},
		{
			name: "end pending activity successfully",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityPending,
				StartTime:  now.Add(-1 * time.Hour),
				EndTime:    now.Add(1 * time.Hour),
			},
			reason:     "cancelled",
			endTime:    now,
			wantErr:    nil,
			wantStatus: ActivityEnded,
			wantEvent:  true,
		},
		{
			name: "fail to end already ended activity",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityEnded,
				StartTime:  now.Add(-2 * time.Hour),
				EndTime:    now.Add(-1 * time.Hour),
			},
			reason:     "duplicate end",
			endTime:    now,
			wantErr:    errors.New("activity already ended"),
			wantStatus: ActivityEnded,
			wantEvent:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Embed AggregateRoot to enable event recording
			aggregate := &domain.AggregateRoot{}
			tt.activity.AggregateRoot = aggregate

			err := tt.activity.End(tt.reason, tt.endTime)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("End() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("End() expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("End() unexpected error: %v", err)
			}

			if tt.activity.Status != tt.wantStatus {
				t.Errorf("End() expected status %d, got %d", tt.wantStatus, tt.activity.Status)
			}

			events := tt.activity.GetDomainEvents()
			if tt.wantEvent {
				if len(events) != 1 {
					t.Errorf("End() expected 1 event, got %d", len(events))
				} else {
					endedEvent, ok := events[0].(*event.ActivityEndedEvent)
					if !ok {
						t.Errorf("End() expected ActivityEndedEvent, got %T", events[0])
					} else if endedEvent.ActivityNo != tt.activity.ActivityNo {
						t.Errorf("End() expected ActivityNo %s, got %s", tt.activity.ActivityNo, endedEvent.ActivityNo)
					} else if endedEvent.Reason != tt.reason {
						t.Errorf("End() expected reason %s, got %s", tt.reason, endedEvent.Reason)
					}
				}
			} else if len(events) != 0 {
				t.Errorf("End() expected 0 events, got %d", len(events))
			}
		})
	}
}

func TestActivity_AddSKU(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		activity    *Activity
		sku         SKU
		wantErr     error
		wantSKUCount int
		wantEvent   bool
	}{
		{
			name: "add SKU to pending activity successfully",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityPending,
				StartTime:  now.Add(-1 * time.Hour),
				EndTime:    now.Add(1 * time.Hour),
				SKUs:       []SKU{},
			},
			sku: SKU{
				ActivityNo:    "ACT001",
				SKUNo:         "SKU001",
				ProductName:   "Test Product",
				OriginalPrice: 10000,
				SeckillPrice:  8000,
				TotalStock:    100,
				LimitQuantity: 2,
			},
			wantErr:      nil,
			wantSKUCount: 1,
			wantEvent:    true,
		},
		{
			name: "add SKU to open activity successfully",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityOpen,
				StartTime:  now.Add(-1 * time.Hour),
				EndTime:    now.Add(1 * time.Hour),
				SKUs:       []SKU{},
			},
			sku: SKU{
				ActivityNo:    "ACT001",
				SKUNo:         "SKU002",
				ProductName:   "Another Product",
				OriginalPrice: 20000,
				SeckillPrice:  15000,
				TotalStock:    50,
				LimitQuantity: 1,
			},
			wantErr:      nil,
			wantSKUCount: 1,
			wantEvent:    true,
		},
		{
			name: "fail to add duplicate SKU",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityPending,
				StartTime:  now.Add(-1 * time.Hour),
				EndTime:    now.Add(1 * time.Hour),
				SKUs: []SKU{
					{
						ActivityNo: "ACT001",
						SKUNo:      "SKU001",
						ProductName: "Test Product",
					},
				},
			},
			sku: SKU{
				ActivityNo:    "ACT001",
				SKUNo:         "SKU001",
				ProductName:   "Duplicate Product",
				OriginalPrice: 10000,
				SeckillPrice:  8000,
				TotalStock:    100,
			},
			wantErr:      errors.New("SKU already exists in activity"),
			wantSKUCount: 1,
			wantEvent:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Embed AggregateRoot to enable event recording
			aggregate := &domain.AggregateRoot{}
			tt.activity.AggregateRoot = aggregate

			err := tt.activity.AddSKU(tt.sku)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("AddSKU() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("AddSKU() expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("AddSKU() unexpected error: %v", err)
			}

			if len(tt.activity.SKUs) != tt.wantSKUCount {
				t.Errorf("AddSKU() expected %d SKUs, got %d", tt.wantSKUCount, len(tt.activity.SKUs))
			}

			events := tt.activity.GetDomainEvents()
			if tt.wantEvent {
				if len(events) != 1 {
					t.Errorf("AddSKU() expected 1 event, got %d", len(events))
				} else {
					addedEvent, ok := events[0].(*event.SKUAddedEvent)
					if !ok {
						t.Errorf("AddSKU() expected SKUAddedEvent, got %T", events[0])
					} else if addedEvent.SKUNo != tt.sku.SKUNo {
						t.Errorf("AddSKU() expected SKUNo %s, got %s", tt.sku.SKUNo, addedEvent.SKUNo)
					}
				}
			} else if len(events) != 0 {
				t.Errorf("AddSKU() expected 0 events, got %d", len(events))
			}
		})
	}
}

func TestActivity_RemoveSKU(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		activity    *Activity
		skuNo       string
		reason      string
		wantErr     error
		wantSKUCount int
		wantEvent   bool
	}{
		{
			name: "remove SKU from pending activity successfully",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityPending,
				StartTime:  now.Add(-1 * time.Hour),
				EndTime:    now.Add(1 * time.Hour),
				SKUs: []SKU{
					{ActivityNo: "ACT001", SKUNo: "SKU001", ProductName: "Product 1"},
					{ActivityNo: "ACT001", SKUNo: "SKU002", ProductName: "Product 2"},
				},
			},
			skuNo:       "SKU001",
			reason:      "out of stock",
			wantErr:     nil,
			wantSKUCount: 1,
			wantEvent:   true,
		},
		{
			name: "fail to remove SKU from open activity",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityOpen,
				StartTime:  now.Add(-1 * time.Hour),
				EndTime:    now.Add(1 * time.Hour),
				SKUs: []SKU{
					{ActivityNo: "ACT001", SKUNo: "SKU001", ProductName: "Product 1"},
				},
			},
			skuNo:       "SKU001",
			reason:      "test",
			wantErr:     errors.New("cannot remove SKU from open activity"),
			wantSKUCount: 1,
			wantEvent:   false,
		},
		{
			name: "fail to remove non-existent SKU",
			activity: &Activity{
				ActivityNo: "ACT001",
				Status:     ActivityPending,
				StartTime:  now.Add(-1 * time.Hour),
				EndTime:    now.Add(1 * time.Hour),
				SKUs: []SKU{
					{ActivityNo: "ACT001", SKUNo: "SKU001", ProductName: "Product 1"},
				},
			},
			skuNo:       "SKU999",
			reason:      "test",
			wantErr:     errors.New("SKU not found in activity"),
			wantSKUCount: 1,
			wantEvent:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Embed AggregateRoot to enable event recording
			aggregate := &domain.AggregateRoot{}
			tt.activity.AggregateRoot = aggregate

			err := tt.activity.RemoveSKU(tt.skuNo, tt.reason)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("RemoveSKU() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("RemoveSKU() expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("RemoveSKU() unexpected error: %v", err)
			}

			if len(tt.activity.SKUs) != tt.wantSKUCount {
				t.Errorf("RemoveSKU() expected %d SKUs, got %d", tt.wantSKUCount, len(tt.activity.SKUs))
			}

			events := tt.activity.GetDomainEvents()
			if tt.wantEvent {
				if len(events) != 1 {
					t.Errorf("RemoveSKU() expected 1 event, got %d", len(events))
				} else {
					removedEvent, ok := events[0].(*event.SKURemovedEvent)
					if !ok {
						t.Errorf("RemoveSKU() expected SKURemovedEvent, got %T", events[0])
					} else if removedEvent.SKUNo != tt.skuNo {
						t.Errorf("RemoveSKU() expected SKUNo %s, got %s", tt.skuNo, removedEvent.SKUNo)
					} else if removedEvent.Reason != tt.reason {
						t.Errorf("RemoveSKU() expected reason %s, got %s", tt.reason, removedEvent.Reason)
					}
				}
			} else if len(events) != 0 {
				t.Errorf("RemoveSKU() expected 0 events, got %d", len(events))
			}
		})
	}
}
