// Package admin 提供秒杀活动的管理应用服务，包括活动创建、更新、状态管理和商品管理。
package admin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Martindeeepdark/go-common/snowflake"

	domain "seckill-activity-service/internal/domain/entity"
	seckillfactory "seckill-activity-service/internal/domain/factory"
	"seckill-activity-service/internal/domain/repository"
	vo "seckill-activity-service/internal/domain/vo"
)

// ErrInvalidRequest 表示管理请求参数无效。
var ErrInvalidRequest = errors.New("invalid admin request")

// ActivityGateway 定义活动管理所需的数据访问接口。
type ActivityGateway interface {
	ListActivities(ctx context.Context) ([]domain.Activity, error)
	GetActivity(ctx context.Context, activityNo string) (domain.Activity, error)
	CreateActivity(ctx context.Context, activity domain.Activity) (domain.Activity, error)
	UpdateActivity(ctx context.Context, activity domain.Activity) error
	UpdateActivityStatus(ctx context.Context, activityNo string, status int64) error
	AddActivitySKU(ctx context.Context, activityNo string, sku domain.SKU) error
	RemoveActivitySKU(ctx context.Context, activityNo, skuNo string) error
}

// ActivityCacheInvalidator 定义活动缓存失效接口。
type ActivityCacheInvalidator interface {
	EvictActivity(ctx context.Context, activityNo string) error
}

// Option 是应用的配置选项函数。
type Option func(*App)

// App 是活动管理应用服务。
type App struct {
	activities ActivityGateway
	cache      ActivityCacheInvalidator
	clock      func() time.Time
}

// NewApp 创建活动管理应用服务实例。
func NewApp(activities ActivityGateway, opts ...Option) *App {
	app := &App{activities: activities}
	for _, opt := range opts {
		opt(app)
	}
	return app
}

// WithCacheInvalidator 设置缓存失效器。
func WithCacheInvalidator(cache ActivityCacheInvalidator) Option {
	return func(app *App) {
		app.cache = cache
	}
}

// ListActivities 根据查询条件分页返回活动列表。
func (a *App) ListActivities(ctx context.Context, query ActivityQuery) ([]ActivityListVO, error) {
	activities, err := a.activities.ListActivities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	name := strings.TrimSpace(query.ActivityName)
	list := make([]ActivityListVO, 0, len(activities))
	for _, activity := range activities {
		if query.ActivityStatus != nil && activity.Status != *query.ActivityStatus {
			continue
		}
		if name != "" && !strings.Contains(activity.Name, name) {
			continue
		}
		list = append(list, ActivityListVO{
			ActivityNo:     activity.ActivityNo,
			ActivityName:   activity.Name,
			ActivityStatus: activity.Status,
			StartTime:      activity.StartTime,
			EndTime:        activity.EndTime,
			PurchaseLimit:  activity.PurchaseLimit,
			ProductCount:   len(activity.SKUs),
			CreatedAt:      activity.CreatedAt,
		})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ActivityNo < list[j].ActivityNo
	})
	return pageActivities(list, query.Page, query.Size), nil
}

// GetActivityDetail 返回活动详情。
func (a *App) GetActivityDetail(ctx context.Context, activityNo string) (ActivityDetailVO, error) {
	activity, err := a.activities.GetActivity(ctx, strings.TrimSpace(activityNo))
	if err != nil {
		return ActivityDetailVO{}, fmt.Errorf("get activity: %w", err)
	}
	return toActivityDetail(activity), nil
}

// CreateActivity 创建新的秒杀活动，自动生成活动编号并验证业务规则。
func (a *App) CreateActivity(ctx context.Context, command CreateActivityCommand) (string, error) {
	name := strings.TrimSpace(command.ActivityName)
	if name == "" || command.StartTime.IsZero() || command.EndTime.IsZero() || !command.EndTime.After(command.StartTime) || command.StartTime.Before(a.now()) {
		return "", ErrInvalidRequest
	}
	activityNo := "A" + strconv.FormatInt(snowflake.NewID(), 10)
	activityID, ok := vo.NewActivityNo(activityNo)
	if !ok {
		return "", ErrInvalidRequest
	}
	activityName, ok := vo.NewActivityName(name)
	if !ok {
		return "", ErrInvalidRequest
	}
	limit := vo.DefaultPurchaseLimit()
	if command.PurchaseLimit > 0 {
		parsed, ok := vo.NewPurchaseLimit(command.PurchaseLimit)
		if !ok {
			return "", ErrInvalidRequest
		}
		limit = parsed
	}
	activity, ok := (seckillfactory.ActivityFactory{}).NewPending(seckillfactory.PendingActivityParams{
		ActivityNo:    activityID,
		Name:          activityName,
		StartTime:     command.StartTime,
		EndTime:       command.EndTime,
		PurchaseLimit: limit,
		Remark:        strings.TrimSpace(command.Remark),
		CreatedAt:     a.now(),
	})
	if !ok {
		return "", ErrInvalidRequest
	}
	created, err := a.activities.CreateActivity(ctx, activity)
	if err != nil {
		return "", fmt.Errorf("create activity: %w", err)
	}
	a.evictActivity(ctx, created.ActivityNo)
	return created.ActivityNo, nil
}

// UpdateActivity 更新活动基本信息，已结束的活动不允许修改。
func (a *App) UpdateActivity(ctx context.Context, command UpdateActivityCommand) error {
	activityNo := strings.TrimSpace(command.ActivityNo)
	if activityNo == "" {
		return ErrInvalidRequest
	}
	existing, err := a.activities.GetActivity(ctx, activityNo)
	if err != nil {
		return fmt.Errorf("get activity: %w", err)
	}
	if err := existing.CanModify(); err != nil {
		return repository.ErrInvalidState
	}
	nextStartTime := existing.StartTime
	if !command.StartTime.IsZero() {
		nextStartTime = command.StartTime
	}
	nextEndTime := existing.EndTime
	if !command.EndTime.IsZero() {
		nextEndTime = command.EndTime
	}
	if nextStartTime.IsZero() || nextEndTime.IsZero() || !nextEndTime.After(nextStartTime) {
		return ErrInvalidRequest
	}
	err = a.activities.UpdateActivity(ctx, domain.Activity{
		ActivityNo:    activityNo,
		Name:          strings.TrimSpace(command.ActivityName),
		StartTime:     command.StartTime,
		EndTime:       command.EndTime,
		PurchaseLimit: command.PurchaseLimit,
		Remark:        strings.TrimSpace(command.Remark),
	})
	if err != nil {
		return fmt.Errorf("update activity: %w", err)
	}
	a.evictActivity(ctx, activityNo)
	return nil
}

// EndActivity 结束活动。
func (a *App) EndActivity(ctx context.Context, activityNo string) error {
	activityNo = strings.TrimSpace(activityNo)
	if activityNo == "" {
		return ErrInvalidRequest
	}
	existing, err := a.activities.GetActivity(ctx, activityNo)
	if err != nil {
		return fmt.Errorf("get activity: %w", err)
	}
	if err := existing.CanModify(); err != nil {
		return repository.ErrInvalidState
	}
	if err := a.activities.UpdateActivityStatus(ctx, activityNo, domain.ActivityEnded); err != nil {
		return fmt.Errorf("update activity status: %w", err)
	}
	a.evictActivity(ctx, activityNo)
	return nil
}

// AddProduct 向活动添加商品，已结束的活动不允许添加。
func (a *App) AddProduct(ctx context.Context, command AddProductCommand) error {
	activityNo := strings.TrimSpace(command.ActivityNo)
	skuNo := strings.TrimSpace(command.SKUNo)
	if activityNo == "" || skuNo == "" || command.ActivityStock <= 0 {
		return ErrInvalidRequest
	}
	existing, err := a.activities.GetActivity(ctx, activityNo)
	if err != nil {
		return fmt.Errorf("get activity: %w", err)
	}
	if err := existing.CanModify(); err != nil {
		return repository.ErrInvalidState
	}
	productName := strings.TrimSpace(command.ProductName)
	if productName == "" {
		productName = skuNo
	}
	price := command.SeckillPrice
	if price <= 0 {
		price = command.DiscountPrice
	}
	limit := command.LimitQuantity
	if limit <= 0 {
		limit = existing.PurchaseLimit
	}
	err = a.activities.AddActivitySKU(ctx, activityNo, domain.SKU{
		ActivityNo:    activityNo,
		SKUNo:         skuNo,
		ProductName:   productName,
		ProductImage:  strings.TrimSpace(command.ProductImage),
		OriginalPrice: command.OriginalPrice,
		SeckillPrice:  price,
		TotalStock:    command.ActivityStock,
		LimitQuantity: limit,
		DiscountType:  command.DiscountType,
		DiscountPrice: command.DiscountPrice,
		DiscountPct:   command.DiscountPercent,
	})
	if err != nil {
		return fmt.Errorf("add activity sku: %w", err)
	}
	a.evictActivity(ctx, activityNo)
	return nil
}

// RemoveProduct 从活动中移除商品，进行中的活动不允许移除。
func (a *App) RemoveProduct(ctx context.Context, activityNo string, skuNo string) error {
	activityNo = strings.TrimSpace(activityNo)
	skuNo = strings.TrimSpace(skuNo)
	if activityNo == "" || skuNo == "" {
		return ErrInvalidRequest
	}
	existing, err := a.activities.GetActivity(ctx, activityNo)
	if err != nil {
		return fmt.Errorf("get activity: %w", err)
	}
	if err := existing.CanRemoveProducts(); err != nil {
		return repository.ErrInvalidState
	}
	if err := a.activities.RemoveActivitySKU(ctx, activityNo, skuNo); err != nil {
		return fmt.Errorf("remove activity sku: %w", err)
	}
	a.evictActivity(ctx, activityNo)
	return nil
}

// now 返回当前时间，可注入用于测试。
func (a *App) now() time.Time {
	if a.clock != nil {
		return a.clock()
	}
	return time.Now()
}

// evictActivity 失效活动缓存。
func (a *App) evictActivity(ctx context.Context, activityNo string) {
	if a.cache == nil {
		return
	}
	_ = a.cache.EvictActivity(ctx, activityNo) //nolint:errcheck // best-effort cache eviction
}
