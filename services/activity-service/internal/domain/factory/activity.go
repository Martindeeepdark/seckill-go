// Package factory 提供活动聚合的工厂方法，封装活动创建的业务规则。
package factory

import (
	"time"

	"seckill-activity-service/internal/domain/entity"
	vo "seckill-activity-service/internal/domain/vo"
)

// ActivityFactory 负责创建秒杀活动聚合，集中校验活动创建时的不变量。
type ActivityFactory struct{}

// PendingActivityParams 是创建待开始活动所需的领域参数。
type PendingActivityParams struct {
	ActivityNo    vo.ActivityNo
	Name          vo.ActivityName
	StartTime     time.Time
	EndTime       time.Time
	PurchaseLimit vo.PurchaseLimit
	Remark        string
	CreatedAt     time.Time
}

// NewPending 创建待开始活动，并保证新活动只能以待开始状态进入领域。
func (ActivityFactory) NewPending(params PendingActivityParams) (entity.Activity, bool) {
	if params.ActivityNo.String() == "" || params.Name.String() == "" {
		return entity.Activity{}, false
	}
	if params.StartTime.IsZero() || params.EndTime.IsZero() || !params.EndTime.After(params.StartTime) {
		return entity.Activity{}, false
	}
	if params.CreatedAt.IsZero() || params.PurchaseLimit.Int() <= 0 {
		return entity.Activity{}, false
	}
	return entity.Activity{
		ActivityNo:    params.ActivityNo.String(),
		Name:          params.Name.String(),
		StartTime:     params.StartTime,
		EndTime:       params.EndTime,
		Status:        entity.ActivityPending,
		PurchaseLimit: params.PurchaseLimit.Int(),
		Remark:        params.Remark,
		CreatedAt:     params.CreatedAt,
	}, true
}
