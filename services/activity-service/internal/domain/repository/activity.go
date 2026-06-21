// Package repository 定义活动仓储接口
package repository

import (
	"context"
	"errors"
	"time"

	"seckill-activity-service/internal/domain/entity"
)

var (
	// ErrNotFound 资源未找到错误
	ErrNotFound = errors.New("not found")
	// ErrDuplicate 资源重复错误
	ErrDuplicate = errors.New("duplicate")
	// ErrOptimisticLock 乐观锁冲突错误
	ErrOptimisticLock = errors.New("optimistic lock conflict")
)

// ActivityRepository 活动仓储接口
type ActivityRepository interface {
	// Save 保存聚合根（完整保存，使用乐观锁）
	Save(ctx context.Context, activity *entity.Activity) error

	// GetByActivityNo 根据活动编号加载聚合根
	GetByActivityNo(ctx context.Context, activityNo string) (*entity.Activity, error)

	// GetOpenActivities 获取进行中的活动列表
	GetOpenActivities(ctx context.Context, now time.Time) ([]*entity.Activity, error)

	// GetActivitiesByStatus 根据状态获取活动列表
	GetActivitiesByStatus(ctx context.Context, status int64, limit int) ([]*entity.Activity, error)
}
