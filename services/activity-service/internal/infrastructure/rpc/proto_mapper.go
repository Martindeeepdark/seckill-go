// Package rpc 提供领域实体与 protobuf 消息的双向映射。
package rpc

import (
	"time"

	domain "seckill-activity-service/internal/domain/entity"
	activityv1 "seckill-api/activity/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// activityToPB 将活动领域实体转换为 protobuf 消息。
func activityToPB(activity domain.Activity) *activityv1.Activity {
	skus := make([]*activityv1.SKU, 0, len(activity.SKUs))
	for _, sku := range activity.SKUs {
		skus = append(skus, skuToPB(sku))
	}
	return &activityv1.Activity{
		ActivityNo:    activity.ActivityNo,
		Name:          activity.Name,
		StartTime:     timeToPB(activity.StartTime),
		EndTime:       timeToPB(activity.EndTime),
		Status:        activity.Status,
		PurchaseLimit: activity.PurchaseLimit,
		Remark:        activity.Remark,
		Skus:          skus,
		CreatedAt:     timeToPB(activity.CreatedAt),
		UpdatedAt:     timeToPB(activity.UpdatedAt),
	}
}

// activityFromPB 将 protobuf 消息转换为活动领域实体。
func activityFromPB(activity *activityv1.Activity) domain.Activity {
	if activity == nil {
		return domain.Activity{}
	}
	skus := make([]domain.SKU, 0, len(activity.GetSkus()))
	for _, sku := range activity.GetSkus() {
		skus = append(skus, skuFromPB(sku))
	}
	return domain.Activity{
		ActivityNo:    activity.GetActivityNo(),
		Name:          activity.GetName(),
		StartTime:     timeFromPB(activity.GetStartTime()),
		EndTime:       timeFromPB(activity.GetEndTime()),
		Status:        activity.GetStatus(),
		PurchaseLimit: activity.GetPurchaseLimit(),
		Remark:        activity.GetRemark(),
		SKUs:          skus,
		CreatedAt:     timeFromPB(activity.GetCreatedAt()),
		UpdatedAt:     timeFromPB(activity.GetUpdatedAt()),
	}
}

// skuToPB 将 SKU 领域实体转换为 protobuf 消息。
func skuToPB(sku domain.SKU) *activityv1.SKU {
	return &activityv1.SKU{
		ActivityNo:      sku.ActivityNo,
		SkuNo:           sku.SKUNo,
		ProductName:     sku.ProductName,
		ProductImage:    sku.ProductImage,
		OriginalPrice:   sku.OriginalPrice,
		SeckillPrice:    sku.SeckillPrice,
		TotalStock:      sku.TotalStock,
		LimitQuantity:   sku.LimitQuantity,
		DiscountType:    sku.DiscountType,
		DiscountPrice:   sku.DiscountPrice,
		DiscountPercent: sku.DiscountPct,
	}
}

// skuFromPB 将 protobuf 消息转换为 SKU 领域实体。
func skuFromPB(sku *activityv1.SKU) domain.SKU {
	if sku == nil {
		return domain.SKU{}
	}
	return domain.SKU{
		ActivityNo:    sku.GetActivityNo(),
		SKUNo:         sku.GetSkuNo(),
		ProductName:   sku.GetProductName(),
		ProductImage:  sku.GetProductImage(),
		OriginalPrice: sku.GetOriginalPrice(),
		SeckillPrice:  sku.GetSeckillPrice(),
		TotalStock:    sku.GetTotalStock(),
		LimitQuantity: sku.GetLimitQuantity(),
		DiscountType:  sku.GetDiscountType(),
		DiscountPrice: sku.GetDiscountPrice(),
		DiscountPct:   sku.GetDiscountPercent(),
	}
}

// timeToPB 将 Go 时间转换为 protobuf 时间戳。
func timeToPB(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// timeFromPB 将 protobuf 时间戳转换为 Go 时间。
func timeFromPB(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
