// Package status 提供活动状态机的业务规则。
package status

import (
	"slices"
	"time"

	"seckill-activity-service/internal/domain/entity"
)

const (
	ActivityPending = 0 // 待开始
	ActivityOpen    = 1 // 进行中
	ActivityPaused  = 2 // 已暂停
	ActivityEnded   = 3 // 已结束
)

// ActivityStatusLabel 定义活动状态码在页面和日志中的中文含义。
var ActivityStatusLabel = map[int64]string{
	ActivityPending: "待开始",
	ActivityOpen:    "进行中",
	ActivityPaused:  "已暂停",
	ActivityEnded:   "已结束",
}

// ActivityAllowedTransitions 定义活动状态机允许的流转方向。
var ActivityAllowedTransitions = map[int64][]int64{
	ActivityPending: {ActivityOpen, ActivityEnded},
	ActivityOpen:    {ActivityPaused, ActivityEnded},
	ActivityPaused:  {ActivityOpen, ActivityEnded},
	ActivityEnded:   {},
}

// CanActivityTransitionTo 判断活动状态是否允许从 from 流转到 to。
func CanActivityTransitionTo(from int64, to int64) bool {
	targets, ok := ActivityAllowedTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(targets, to)
}

// TransitActivity 返回更新状态后的活动副本，非法流转时返回 false。
func TransitActivity(activity entity.Activity, target int64, now time.Time) (entity.Activity, bool) {
	if activity.Status == target {
		return activity, true
	}
	if !CanActivityTransitionTo(activity.Status, target) {
		return activity, false
	}
	activity.Status = target
	activity.UpdatedAt = now
	return activity, true
}

// IsActivityOpen 判断活动当前是否开放参与（状态为进行中且在时间窗口内）。
func IsActivityOpen(a entity.Activity, now time.Time) bool {
	return a.Status == ActivityOpen && !now.Before(a.StartTime) && now.Before(a.EndTime)
}
