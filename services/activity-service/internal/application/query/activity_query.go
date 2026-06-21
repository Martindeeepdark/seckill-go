// Package query 提供秒杀活动的查询应用服务，面向 C 端用户。
package query

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	domain "seckill-activity-service/internal/domain/entity"
	"seckill-activity-service/internal/domain/status"
)

// ActivityGateway 定义活动查询所需的数据访问接口。
type ActivityGateway interface {
	ListActivities(ctx context.Context) ([]domain.Activity, error)
	GetActivity(ctx context.Context, activityNo string) (domain.Activity, error)
	GetSKU(ctx context.Context, activityNo, skuNo string) (domain.SKU, error)
}

// ActivityListVO 定义活动列表视图对象。
type ActivityListVO struct {
	ActivityNo     string    `json:"activityNo"`
	ActivityName   string    `json:"activityName"`
	StartTime      time.Time `json:"startTime"`
	EndTime        time.Time `json:"endTime"`
	ActivityStatus int64     `json:"activityStatus"`
}

// ActivityDetailVO 定义活动详情视图对象。
type ActivityDetailVO struct {
	ActivityNo     string      `json:"activityNo"`
	ActivityName   string      `json:"activityName"`
	StartTime      time.Time   `json:"startTime"`
	EndTime        time.Time   `json:"endTime"`
	ActivityStatus int64       `json:"activityStatus"`
	PurchaseLimit  int64       `json:"purchaseLimit"`
	ActivityOpen   bool        `json:"activityOpen"`
	Products       []ProductVO `json:"products"`
}

// ProductVO 定义商品视图对象。
type ProductVO struct {
	SKUNo           string `json:"skuNo,omitempty"`
	ProductName     string `json:"productName"`
	ProductImage    string `json:"productImage"`
	OriginalPrice   int64  `json:"originalPrice"`
	SeckillPrice    int64  `json:"seckillPrice,omitempty"`
	ActivityStock   int64  `json:"activityStock,omitempty"`
	DiscountType    int64  `json:"discountType,omitempty"`
	DiscountPrice   int64  `json:"discountPrice,omitempty"`
	DiscountPercent int64  `json:"discountPercent,omitempty"`
	SortOrder       int    `json:"sortOrder,omitempty"`
}

// App 是活动查询应用服务。
type App struct {
	activity ActivityGateway
}

// NewApp 创建活动查询应用服务实例。
func NewApp(activity ActivityGateway) *App {
	return &App{activity: activity}
}

// ActiveActivities 返回进行中的活动列表，按创建时间倒序排列。
func (a *App) ActiveActivities(ctx context.Context) ([]ActivityListVO, error) {
	activities, err := a.activity.ListActivities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	sort.Slice(activities, func(i, j int) bool {
		if !activities[i].CreatedAt.Equal(activities[j].CreatedAt) {
			return activities[i].CreatedAt.After(activities[j].CreatedAt)
		}
		return activities[i].ActivityNo < activities[j].ActivityNo
	})
	list := make([]ActivityListVO, 0, len(activities))
	for _, activity := range activities {
		if activity.Status != status.ActivityOpen {
			continue
		}
		list = append(list, activityListVO(activity))
	}
	return list, nil
}

// ActivityDetail 返回活动详情，包括是否开放参与的状态。
func (a *App) ActivityDetail(ctx context.Context, activityNo string) (ActivityDetailVO, error) {
	activity, err := a.activity.GetActivity(ctx, strings.TrimSpace(activityNo))
	if err != nil {
		return ActivityDetailVO{}, fmt.Errorf("get activity detail: %w", err)
	}
	return activityDetailVO(activity, time.Now()), nil
}

// ActivityProducts 返回活动商品列表。
func (a *App) ActivityProducts(ctx context.Context, activityNo string) ([]ProductVO, error) {
	activity, err := a.activity.GetActivity(ctx, strings.TrimSpace(activityNo))
	if err != nil {
		return nil, fmt.Errorf("get activity: %w", err)
	}
	return productVOs(activity.SKUs), nil
}

// activityListVO 将活动领域实体转换为列表视图对象。
func activityListVO(activity domain.Activity) ActivityListVO {
	return ActivityListVO{
		ActivityNo:     activity.ActivityNo,
		ActivityName:   activity.Name,
		StartTime:      activity.StartTime,
		EndTime:        activity.EndTime,
		ActivityStatus: activity.Status,
	}
}

// activityDetailVO 将活动领域实体转换为详情视图对象。
func activityDetailVO(activity domain.Activity, now time.Time) ActivityDetailVO {
	return ActivityDetailVO{
		ActivityNo:     activity.ActivityNo,
		ActivityName:   activity.Name,
		StartTime:      activity.StartTime,
		EndTime:        activity.EndTime,
		ActivityStatus: activity.Status,
		PurchaseLimit:  activity.PurchaseLimit,
		ActivityOpen:   status.IsActivityOpen(activity, now),
		Products:       productVOs(activity.SKUs),
	}
}

// productVOs 将 SKU 列表转换为商品视图对象列表。
func productVOs(skus []domain.SKU) []ProductVO {
	products := make([]ProductVO, 0, len(skus))
	for index, sku := range skus {
		products = append(products, ProductVO{
			SKUNo:           sku.SKUNo,
			ProductName:     sku.ProductName,
			ProductImage:    sku.ProductImage,
			OriginalPrice:   sku.OriginalPrice,
			SeckillPrice:    sku.SeckillPrice,
			ActivityStock:   sku.TotalStock,
			DiscountType:    sku.DiscountType,
			DiscountPrice:   sku.DiscountPrice,
			DiscountPercent: sku.DiscountPct,
			SortOrder:       index + 1,
		})
	}
	return products
}
