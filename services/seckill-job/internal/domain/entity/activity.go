// Package entity 提供定时任务领域实体模型
package entity

import "time"

// Activity 秒杀活动实体
type Activity struct {
	ActivityNo    string    // 活动编号
	Name          string    // 活动名称
	StartTime     time.Time // 开始时间
	EndTime       time.Time // 结束时间
	Status        int       // 状态（0待开始/1进行中/3已结束）
	PurchaseLimit int       // 每人购买限制
	Remark        string    // 备注
	SKUs          []SKU     // SKU列表
	CreatedAt     time.Time // 创建时间
	UpdatedAt     time.Time // 更新时间
}

// SKU 库存单位实体
type SKU struct {
	ActivityNo string // 活动编号
	SKUNo      string // SKU编号
	TotalStock int64  // 总库存
}

// Activity status constants
const (
	ActivityPending = 0 // 活动未开始
	ActivityOpen    = 1 // 活动进行中
	ActivityEnded   = 3 // 活动已结束
)

// IsOpen checks if the activity is currently open.
func (a *Activity) IsOpen(now time.Time) bool {
	return a.Status == ActivityOpen && !now.Before(a.StartTime) && now.Before(a.EndTime)
}

// IsEndedOrExpired checks if the activity has ended or expired.
func (a *Activity) IsEndedOrExpired(now time.Time) bool {
	if a.Status == ActivityEnded {
		return true
	}
	return !a.EndTime.IsZero() && !a.EndTime.After(now)
}

// IsActive checks if the activity is currently active and within its time window.
func (a *Activity) IsActive(now time.Time) bool {
	return a.Status == ActivityOpen && !a.StartTime.After(now) && a.EndTime.After(now)
}

// IsUpcoming checks if the activity is pending and will start before the deadline.
func (a *Activity) IsUpcoming(now time.Time, deadline time.Time) bool {
	return a.Status == ActivityPending && a.StartTime.After(now) && !a.StartTime.After(deadline)
}

// ShouldOpen checks if a pending activity should transition to open.
func (a *Activity) ShouldOpen(now time.Time) bool {
	return a.Status == ActivityPending && !now.Before(a.StartTime)
}

// ShouldEnd checks if an open activity should transition to ended.
func (a *Activity) ShouldEnd(now time.Time) bool {
	return a.Status == ActivityOpen && !a.EndTime.IsZero() && !a.EndTime.After(now)
}
