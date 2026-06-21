package application

import (
	"context"
	"log/slog"
	"seckill-common/domain"
	"seckill-activity-service/internal/domain/entity"
	"seckill-activity-service/internal/domain/repository"
	"testing"
	"time"
)

type mockActivityRepository struct {
	activities map[string]*entity.Activity
}

func newMockActivityRepository() *mockActivityRepository {
	return &mockActivityRepository{
		activities: make(map[string]*entity.Activity),
	}
}

func (m *mockActivityRepository) Save(ctx context.Context, activity *entity.Activity) error {
	m.activities[activity.ActivityNo] = activity
	return nil
}

func (m *mockActivityRepository) GetByActivityNo(ctx context.Context, activityNo string) (*entity.Activity, error) {
	activity, ok := m.activities[activityNo]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return activity, nil
}

func (m *mockActivityRepository) GetOpenActivities(ctx context.Context, now time.Time) ([]*entity.Activity, error) {
	return nil, nil
}

func (m *mockActivityRepository) GetActivitiesByStatus(ctx context.Context, status int64, limit int) ([]*entity.Activity, error) {
	return nil, nil
}

type mockEventBus struct {
	publishedEvents []domain.Event
}

func (m *mockEventBus) Publish(ctx context.Context, event domain.Event) error {
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

func TestActivityAppService_StartActivity(t *testing.T) {
	// Given
	repo := newMockActivityRepository()
	bus := &mockEventBus{}
	logger := slog.Default()
	svc := NewActivityAppService(repo, bus, logger)

	activity := &entity.Activity{
		AggregateRoot: &domain.AggregateRoot{},
		ActivityNo: "ACT-001",
		Status:     entity.ActivityPending,
		StartTime:  time.Now().Add(-1 * time.Hour),
		EndTime:    time.Now().Add(1 * time.Hour),
	}
	repo.Save(context.Background(), activity)

	cmd := StartActivityCommand{
		ActivityNo: "ACT-001",
		StartedAt:  time.Now(),
	}

	// When
	err := svc.StartActivity(context.Background(), cmd)

	// Then
	if err != nil {
		t.Fatalf("StartActivity failed: %v", err)
	}

	updatedActivity, _ := repo.GetByActivityNo(context.Background(), "ACT-001")
	if updatedActivity.Status != entity.ActivityOpen {
		t.Errorf("expected status %d, got %d", entity.ActivityOpen, updatedActivity.Status)
	}

	if len(bus.publishedEvents) != 1 {
		t.Errorf("expected 1 event, got %d", len(bus.publishedEvents))
	}

	eventName := bus.publishedEvents[0].EventName()
	if eventName != "activity.started" {
		t.Errorf("expected event name 'activity.started', got '%s'", eventName)
	}
}

func TestActivityAppService_AddSKU(t *testing.T) {
	// Given
	repo := newMockActivityRepository()
	bus := &mockEventBus{}
	logger := slog.Default()
	svc := NewActivityAppService(repo, bus, logger)

	activity := &entity.Activity{
		AggregateRoot: &domain.AggregateRoot{},
		ActivityNo: "ACT-001",
		Status:     entity.ActivityPending,
		StartTime:  time.Now().Add(-1 * time.Hour),
		EndTime:    time.Now().Add(1 * time.Hour),
	}
	repo.Save(context.Background(), activity)

	cmd := AddSKUCommand{
		ActivityNo: "ACT-001",
		SKUNo:      "SKU-001",
		Stock:      100,
		Price:      9999,
	}

	// When
	err := svc.AddSKU(context.Background(), cmd)

	// Then
	if err != nil {
		t.Fatalf("AddSKU failed: %v", err)
	}

	updatedActivity, _ := repo.GetByActivityNo(context.Background(), "ACT-001")
	if len(updatedActivity.SKUs) != 1 {
		t.Errorf("expected 1 SKU, got %d", len(updatedActivity.SKUs))
	}

	if len(bus.publishedEvents) != 1 {
		t.Errorf("expected 1 event, got %d", len(bus.publishedEvents))
	}

	eventName := bus.publishedEvents[0].EventName()
	if eventName != "activity.sku.added" {
		t.Errorf("expected event name 'activity.sku.added', got '%s'", eventName)
	}
}

func TestActivityAppService_StartActivity_InvalidCommand(t *testing.T) {
	// Given
	repo := newMockActivityRepository()
	bus := &mockEventBus{}
	logger := slog.Default()
	svc := NewActivityAppService(repo, bus, logger)

	cmd := StartActivityCommand{
		ActivityNo: "", // Invalid
		StartedAt:  time.Now(),
	}

	// When
	err := svc.StartActivity(context.Background(), cmd)

	// Then
	if err == nil {
		t.Fatal("expected error for invalid command, got nil")
	}
}

func TestActivityAppService_StartActivity_NotFound(t *testing.T) {
	// Given
	repo := newMockActivityRepository()
	bus := &mockEventBus{}
	logger := slog.Default()
	svc := NewActivityAppService(repo, bus, logger)

	cmd := StartActivityCommand{
		ActivityNo: "NON-EXISTENT",
		StartedAt:  time.Now(),
	}

	// When
	err := svc.StartActivity(context.Background(), cmd)

	// Then
	if err == nil {
		t.Fatal("expected error for non-existent activity, got nil")
	}
}

func TestActivityAppService_AddSKU_InvalidCommand(t *testing.T) {
	// Given
	repo := newMockActivityRepository()
	bus := &mockEventBus{}
	logger := slog.Default()
	svc := NewActivityAppService(repo, bus, logger)

	cmd := AddSKUCommand{
		ActivityNo: "ACT-001",
		SKUNo:      "", // Invalid
		Stock:      100,
		Price:      9999,
	}

	// When
	err := svc.AddSKU(context.Background(), cmd)

	// Then
	if err == nil {
		t.Fatal("expected error for invalid command, got nil")
	}
}

func TestActivityAppService_AddSKU_NotFound(t *testing.T) {
	// Given
	repo := newMockActivityRepository()
	bus := &mockEventBus{}
	logger := slog.Default()
	svc := NewActivityAppService(repo, bus, logger)

	cmd := AddSKUCommand{
		ActivityNo: "NON-EXISTENT",
		SKUNo:      "SKU-001",
		Stock:      100,
		Price:      9999,
	}

	// When
	err := svc.AddSKU(context.Background(), cmd)

	// Then
	if err == nil {
		t.Fatal("expected error for non-existent activity, got nil")
	}
}
