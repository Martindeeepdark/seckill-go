// +build integration

// Package integration 提供集成测试，验证完整的活动生命周期
package integration

import (
	"context"
	"testing"
	"time"

	"seckill-common/domain"
	"seckill-activity-service/internal/application"
	"seckill-activity-service/internal/domain/entity"
	"seckill-activity-service/internal/domain/repository"
	"seckill-activity-service/internal/infrastructure/eventbus"
)

// mockRepository 用于集成测试的 mock 实现
type mockRepository struct {
	activities map[string]*entity.Activity
}

func newMockRepository() repository.ActivityRepository {
	return &mockRepository{
		activities: make(map[string]*entity.Activity),
	}
}

func (m *mockRepository) Save(ctx context.Context, activity *entity.Activity) error {
	m.activities[activity.ActivityNo] = activity
	return nil
}

func (m *mockRepository) GetByActivityNo(ctx context.Context, activityNo string) (*entity.Activity, error) {
	activity, ok := m.activities[activityNo]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return activity, nil
}

func (m *mockRepository) GetOpenActivities(ctx context.Context, now time.Time) ([]*entity.Activity, error) {
	return nil, nil
}

func (m *mockRepository) GetActivitiesByStatus(ctx context.Context, status int64, limit int) ([]*entity.Activity, error) {
	return nil, nil
}

// TestActivityLifecycle 测试完整的活动生命周期
func TestActivityLifecycle(t *testing.T) {
	// Given
	repo := newMockRepository()
	bus := eventbus.NewLocalBus()
	svc := application.NewActivityAppService(repo, bus, nil)

	ctx := context.Background()
	now := time.Now()

	// 创建活动
	activity := &entity.Activity{
		ActivityNo: "ACT-TEST-001",
		Status:     entity.ActivityPending,
		StartTime:  now.Add(-1 * time.Hour),
		Name:       "Test Activity",
	}
	// 嵌入 AggregateRoot 以支持事件记录
	aggregate := &domain.AggregateRoot{}
	activity.AggregateRoot = aggregate
	repo.Save(ctx, activity)

	// When: 开始活动
	err := svc.StartActivity(ctx, application.StartActivityCommand{
		ActivityNo: "ACT-TEST-001",
		StartedAt:  now,
	})

	// Then: 验证活动已开始
	if err != nil {
		t.Fatalf("StartActivity failed: %v", err)
	}

	activity, _ = repo.GetByActivityNo(ctx, "ACT-TEST-001")
	if activity.Status != entity.ActivityOpen {
		t.Errorf("expected status %d, got %d", entity.ActivityOpen, activity.Status)
	}

	// When: 添加商品
	err = svc.AddSKU(ctx, application.AddSKUCommand{
		ActivityNo: "ACT-TEST-001",
		SKUNo:      "SKU-TEST-001",
		Stock:      100,
		Price:      9900,
	})

	// Then: 验证商品已添加
	if err != nil {
		t.Fatalf("AddSKU failed: %v", err)
	}

	activity, _ = repo.GetByActivityNo(ctx, "ACT-TEST-001")
	if !activity.HasSKU("SKU-TEST-001") {
		t.Errorf("SKU not added to activity")
	}

	t.Log("Activity lifecycle test completed successfully")
}
