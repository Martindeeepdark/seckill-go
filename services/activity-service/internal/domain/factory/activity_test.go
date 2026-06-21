package factory

import (
	"testing"
	"time"

	"seckill-activity-service/internal/domain/entity"
	vo "seckill-activity-service/internal/domain/vo"
)

func TestActivityFactoryNewPendingBuildsValidActivity(t *testing.T) {
	activityNo, _ := vo.NewActivityNo("A1")
	name, _ := vo.NewActivityName("秒杀活动")
	limit := vo.DefaultPurchaseLimit()
	start := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	createdAt := start.Add(-time.Hour)

	activity, ok := (ActivityFactory{}).NewPending(PendingActivityParams{
		ActivityNo:    activityNo,
		Name:          name,
		StartTime:     start,
		EndTime:       end,
		PurchaseLimit: limit,
		Remark:        "remark",
		CreatedAt:     createdAt,
	})
	if !ok {
		t.Fatal("pending activity should be valid")
	}
	if activity.ActivityNo != "A1" || activity.Name != "秒杀活动" || activity.Status != entity.ActivityPending {
		t.Fatalf("unexpected activity fields: %+v", activity)
	}
	if activity.PurchaseLimit != 1 || activity.Remark != "remark" || !activity.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected activity business fields: %+v", activity)
	}
}

func TestActivityFactoryNewPendingRejectsInvalidSchedule(t *testing.T) {
	activityNo, _ := vo.NewActivityNo("A1")
	name, _ := vo.NewActivityName("秒杀活动")
	limit := vo.DefaultPurchaseLimit()
	start := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

	_, ok := (ActivityFactory{}).NewPending(PendingActivityParams{
		ActivityNo:    activityNo,
		Name:          name,
		StartTime:     start,
		EndTime:       start,
		PurchaseLimit: limit,
		CreatedAt:     start.Add(-time.Hour),
	})
	if ok {
		t.Fatal("end time equal to start time should be invalid")
	}
}
