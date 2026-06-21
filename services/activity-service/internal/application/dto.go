// Package application 提供应用层的数据传输对象（DTO）
package application

import (
	"time"

	"seckill-activity-service/internal/domain/entity"
)

// ActivityDTO 活动数据传输对象
type ActivityDTO struct {
	ActivityNo    string    `json:"activityNo"`
	Name          string    `json:"name"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	Status        int64     `json:"status"`
	PurchaseLimit int64     `json:"purchaseLimit"`
	Remark        string    `json:"remark"`
	SKUs          []SKUDTO  `json:"skus,omitempty"`
	CreatedAt     time.Time `json:"createdAt,omitempty"`
	UpdatedAt     time.Time `json:"updatedAt,omitempty"`
}

// SKUDTO 商品数据传输对象
type SKUDTO struct {
	ActivityNo    string `json:"activityNo"`
	SKUNo         string `json:"skuNo"`
	ProductName   string `json:"productName"`
	ProductImage  string `json:"productImage"`
	OriginalPrice int64  `json:"originalPrice"`
	SeckillPrice  int64  `json:"seckillPrice"`
	TotalStock    int64  `json:"totalStock"`
	LimitQuantity int64  `json:"limitQuantity"`
	DiscountType  int64  `json:"discountType,omitempty"`
	DiscountPrice int64  `json:"discountPrice,omitempty"`
	DiscountPct   int64  `json:"discountPercent,omitempty"`
}

// ToActivityDTO 将领域实体转换为DTO
func ToActivityDTO(activity entity.Activity) ActivityDTO {
	skus := make([]SKUDTO, len(activity.SKUs))
	for i, sku := range activity.SKUs {
		skus[i] = ToSKUDTO(sku)
	}
	return ActivityDTO{
		ActivityNo:    activity.ActivityNo,
		Name:          activity.Name,
		StartTime:     activity.StartTime,
		EndTime:       activity.EndTime,
		Status:        activity.Status,
		PurchaseLimit: activity.PurchaseLimit,
		Remark:        activity.Remark,
		SKUs:          skus,
		CreatedAt:     activity.CreatedAt,
		UpdatedAt:     activity.UpdatedAt,
	}
}

// ToActivityDTOList 将活动列表转换为DTO列表
func ToActivityDTOList(activities []entity.Activity) []ActivityDTO {
	dtos := make([]ActivityDTO, len(activities))
	for i, activity := range activities {
		dtos[i] = ToActivityDTO(activity)
	}
	return dtos
}

// ToSKUDTO 将SKU领域实体转换为DTO
func ToSKUDTO(sku entity.SKU) SKUDTO {
	return SKUDTO{
		ActivityNo:    sku.ActivityNo,
		SKUNo:         sku.SKUNo,
		ProductName:   sku.ProductName,
		ProductImage:  sku.ProductImage,
		OriginalPrice: sku.OriginalPrice,
		SeckillPrice:  sku.SeckillPrice,
		TotalStock:    sku.TotalStock,
		LimitQuantity: sku.LimitQuantity,
		DiscountType:  sku.DiscountType,
		DiscountPrice: sku.DiscountPrice,
		DiscountPct:   sku.DiscountPct,
	}
}

// ToSKUListDTO 将SKU列表转换为DTO列表
func ToSKUListDTO(skus []entity.SKU) []SKUDTO {
	dtos := make([]SKUDTO, len(skus))
	for i, sku := range skus {
		dtos[i] = ToSKUDTO(sku)
	}
	return dtos
}

// ToActivity 从DTO重建领域实体（慎用，仅用于反序列化）
func ToActivity(dto ActivityDTO) entity.Activity {
	skus := make([]entity.SKU, len(dto.SKUs))
	for i, sku := range dto.SKUs {
		skus[i] = ToSKU(sku)
	}
	return entity.Activity{
		ActivityNo:    dto.ActivityNo,
		Name:          dto.Name,
		StartTime:     dto.StartTime,
		EndTime:       dto.EndTime,
		Status:        dto.Status,
		PurchaseLimit: dto.PurchaseLimit,
		Remark:        dto.Remark,
		SKUs:          skus,
		CreatedAt:     dto.CreatedAt,
		UpdatedAt:     dto.UpdatedAt,
	}
}

// ToSKU 从DTO重建SKU领域实体（慎用，仅用于反序列化）
func ToSKU(dto SKUDTO) entity.SKU {
	return entity.SKU{
		ActivityNo:    dto.ActivityNo,
		SKUNo:         dto.SKUNo,
		ProductName:   dto.ProductName,
		ProductImage:  dto.ProductImage,
		OriginalPrice: dto.OriginalPrice,
		SeckillPrice:  dto.SeckillPrice,
		TotalStock:    dto.TotalStock,
		LimitQuantity: dto.LimitQuantity,
		DiscountType:  dto.DiscountType,
		DiscountPrice: dto.DiscountPrice,
		DiscountPct:   dto.DiscountPct,
	}
}
