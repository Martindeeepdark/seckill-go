package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"seckill-activity-service/internal/infrastructure/adapter"
	"seckill-activity-service/internal/infrastructure/persistence"
	domain "seckill-activity-service/internal/domain/entity"
	"seckill-activity-service/internal/domain/repository"
	statemachine "seckill-activity-service/internal/domain/status"
)

func TestCreateActivityAndAddProduct(t *testing.T) {
	now := time.Now()
	repo := persistence.NewMemoryStore()
	app := NewApp(adapter.LocalActivityGateway{Store: repo})
	app.clock = func() time.Time { return now }

	activityNo, err := app.CreateActivity(context.Background(), CreateActivityCommand{
		ActivityName:  "admin-created",
		StartTime:     now.Add(time.Hour),
		EndTime:       now.Add(2 * time.Hour),
		PurchaseLimit: 2,
		Remark:        "from admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if activityNo == "" {
		t.Fatal("activityNo should not be empty")
	}

	err = app.AddProduct(context.Background(), AddProductCommand{
		ActivityNo:    activityNo,
		SKUNo:         "S-admin",
		ProductName:   "admin sku",
		ActivityStock: 5,
		SeckillPrice:  99,
	})
	if err != nil {
		t.Fatal(err)
	}

	detail, err := app.GetActivityDetail(context.Background(), activityNo)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ActivityName != "admin-created" || len(detail.Products) != 1 {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if detail.Products[0].ActivityStock != 5 {
		t.Fatalf("activity stock = %d, want 5", detail.Products[0].ActivityStock)
	}
}

func TestRemoveProductRejectsOpenActivity(t *testing.T) {
	repo := persistence.NewMemoryStore()
	activity := domain.Activity{
		ActivityNo:    "A1",
		Name:          "open",
		StartTime:     time.Now().Add(-time.Minute),
		EndTime:       time.Now().Add(time.Hour),
		Status:        statemachine.ActivityOpen,
		PurchaseLimit: 1,
		SKUs: []domain.SKU{{
			ActivityNo:    "A1",
			SKUNo:         "S1",
			ProductName:   "sku",
			TotalStock:    1,
			LimitQuantity: 1,
		}},
	}
	if err := repo.AddActivity(context.Background(), activity); err != nil {
		t.Fatal(err)
	}
	app := NewApp(adapter.LocalActivityGateway{Store: repo})

	err := app.RemoveProduct(context.Background(), "A1", "S1")
	if !errors.Is(err, repository.ErrInvalidState) {
		t.Fatalf("remove product error = %v, want invalid state", err)
	}
}

func TestListActivitiesFiltersStatus(t *testing.T) {
	repo := persistence.NewMemoryStore()
	for _, activity := range []domain.Activity{
		{ActivityNo: "A1", Name: "pending", Status: statemachine.ActivityPending},
		{ActivityNo: "A2", Name: "open", Status: statemachine.ActivityOpen},
	} {
		if err := repo.AddActivity(context.Background(), activity); err != nil {
			t.Fatal(err)
		}
	}
	app := NewApp(adapter.LocalActivityGateway{Store: repo})
	status := int64(statemachine.ActivityOpen)

	list, err := app.ListActivities(context.Background(), ActivityQuery{ActivityStatus: &status})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ActivityNo != "A2" {
		t.Fatalf("filtered activities = %+v, want A2", list)
	}
}

func TestUpdateActivityEvictsCache(t *testing.T) {
	now := time.Now()
	repo := persistence.NewMemoryStore()
	if err := repo.AddActivity(context.Background(), domain.Activity{
		ActivityNo:    "A1",
		Name:          "pending",
		StartTime:     now.Add(time.Hour),
		EndTime:       now.Add(2 * time.Hour),
		Status:        statemachine.ActivityPending,
		PurchaseLimit: 1,
	}); err != nil {
		t.Fatal(err)
	}
	cache := &fakeActivityCacheInvalidator{}
	app := NewApp(adapter.LocalActivityGateway{Store: repo}, WithCacheInvalidator(cache))

	err := app.UpdateActivity(context.Background(), UpdateActivityCommand{ActivityNo: "A1", ActivityName: "updated"})
	if err != nil {
		t.Fatal(err)
	}
	if cache.activityNo != "A1" {
		t.Fatalf("evicted activityNo = %q, want A1", cache.activityNo)
	}
}

func TestUpdateActivityRejectsMergedInvalidSchedule(t *testing.T) {
	now := time.Now()
	repo := persistence.NewMemoryStore()
	if err := repo.AddActivity(context.Background(), domain.Activity{
		ActivityNo:    "A1",
		Name:          "pending",
		StartTime:     now.Add(time.Hour),
		EndTime:       now.Add(2 * time.Hour),
		Status:        statemachine.ActivityPending,
		PurchaseLimit: 1,
	}); err != nil {
		t.Fatal(err)
	}
	app := NewApp(adapter.LocalActivityGateway{Store: repo})

	err := app.UpdateActivity(context.Background(), UpdateActivityCommand{
		ActivityNo: "A1",
		StartTime:  now.Add(3 * time.Hour),
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("update activity error = %v, want invalid request", err)
	}
}

type fakeActivityCacheInvalidator struct {
	activityNo string
}

func (c *fakeActivityCacheInvalidator) EvictActivity(_ context.Context, activityNo string) error {
	c.activityNo = activityNo
	return nil
}
