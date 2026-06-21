// Package admin 提供管理应用服务的数据传输对象（DTO）。
package admin

import (
	"time"

	domain "seckill-activity-service/internal/domain/entity"
)

// ActivityQuery 定义活动查询条件。
type ActivityQuery struct {
	ActivityName   string // 活动名称（模糊匹配）
	ActivityStatus *int64 // 活动状态（精确匹配）
	Page           int    // 页码
	Size           int    // 每页数量
}

// CreateActivityCommand 定义创建活动的命令。
type CreateActivityCommand struct {
	ActivityName  string    `json:"activityName"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	PurchaseLimit int64     `json:"purchaseLimit"`
	Remark        string    `json:"remark"`
}

// UpdateActivityCommand 定义更新活动的命令。
type UpdateActivityCommand struct {
	ActivityNo    string    `json:"activityNo"`
	ActivityName  string    `json:"activityName"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	PurchaseLimit int64     `json:"purchaseLimit"`
	Remark        string    `json:"remark"`
}

// AddProductCommand 定义向活动添加商品的命令。
type AddProductCommand struct {
	ActivityNo      string `json:"activityNo"`
	SKUNo           string `json:"skuNo"`
	ProductName     string `json:"productName"`
	ProductImage    string `json:"productImage"`
	ActivityStock   int64  `json:"activityStock"`
	OriginalPrice   int64  `json:"originalPrice"`
	SeckillPrice    int64  `json:"seckillPrice"`
	LimitQuantity   int64  `json:"limitQuantity"`
	DiscountType    int64  `json:"discountType"`
	DiscountPrice   int64  `json:"discountPrice"`
	DiscountPercent int64  `json:"discountPercent"`
}

// ActivityListVO 定义活动列表视图对象。
type ActivityListVO struct {
	ActivityNo     string    `json:"activityNo"`
	ActivityName   string    `json:"activityName"`
	ActivityStatus int64     `json:"activityStatus"`
	StartTime      time.Time `json:"startTime"`
	EndTime        time.Time `json:"endTime"`
	PurchaseLimit  int64     `json:"purchaseLimit"`
	ProductCount   int       `json:"productCount"`
	CreatedAt      time.Time `json:"createTime,omitempty"`
}

// ActivityDetailVO 定义活动详情视图对象。
type ActivityDetailVO struct {
	ActivityNo     string              `json:"activityNo"`
	ActivityName   string              `json:"activityName"`
	ActivityStatus int64               `json:"activityStatus"`
	StartTime      time.Time           `json:"startTime"`
	EndTime        time.Time           `json:"endTime"`
	PurchaseLimit  int64               `json:"purchaseLimit"`
	Remark         string              `json:"remark"`
	Products       []ActivityProductVO `json:"products"`
	CreatedAt      time.Time           `json:"createTime,omitempty"`
}

// ActivityProductVO 定义活动商品视图对象。
type ActivityProductVO struct {
	SKUNo           string `json:"skuNo"`
	ProductName     string `json:"productName"`
	ActivityStock   int64  `json:"activityStock"`
	DiscountType    int64  `json:"discountType,omitempty"`
	DiscountPrice   int64  `json:"discountPrice,omitempty"`
	DiscountPercent int64  `json:"discountPercent,omitempty"`
}

// toActivityDetail 将活动领域实体转换为详情视图对象。
func toActivityDetail(activity domain.Activity) ActivityDetailVO {
	products := make([]ActivityProductVO, 0, len(activity.SKUs))
	for _, sku := range activity.SKUs {
		products = append(products, ActivityProductVO{
			SKUNo:           sku.SKUNo,
			ProductName:     sku.ProductName,
			ActivityStock:   sku.TotalStock,
			DiscountType:    sku.DiscountType,
			DiscountPrice:   sku.DiscountPrice,
			DiscountPercent: sku.DiscountPct,
		})
	}
	return ActivityDetailVO{
		ActivityNo:     activity.ActivityNo,
		ActivityName:   activity.Name,
		ActivityStatus: activity.Status,
		StartTime:      activity.StartTime,
		EndTime:        activity.EndTime,
		PurchaseLimit:  activity.PurchaseLimit,
		Remark:         activity.Remark,
		Products:       products,
		CreatedAt:      activity.CreatedAt,
	}
}

// pageActivities 对活动列表进行分页。
func pageActivities(values []ActivityListVO, page int, size int) []ActivityListVO {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	start := (page - 1) * size
	if start >= len(values) {
		return []ActivityListVO{}
	}
	end := start + size
	if end > len(values) {
		end = len(values)
	}
	return values[start:end]
}
