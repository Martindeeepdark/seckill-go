// Package application 提供应用层的数据传输对象（DTO）
package application

import (
	"time"

	"seckill-risk-service/internal/domain/entity"
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

// RiskRecordDTO 风控记录数据传输对象
type RiskRecordDTO struct {
	UserID      int64     `json:"userId"`
	ActionType  string    `json:"actionType"`
	RiskLevel   int64     `json:"riskLevel"`
	RequestIP   string    `json:"requestIp,omitempty"`
	RequestInfo string    `json:"requestInfo,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// RiskEvaluationDTO 风险评估结果数据传输对象
type RiskEvaluationDTO struct {
	Risk   bool   `json:"risk"`
	Level  int64  `json:"level"`
	Reason string `json:"reason,omitempty"`
}

// ToActivityDTO 将活动领域实体转换为DTO
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

// ToActivity 从DTO重建活动领域实体（慎用，仅用于反序列化）
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

// ToRiskRecordDTO 将风控记录领域实体转换为DTO
func ToRiskRecordDTO(record entity.RiskRecord) RiskRecordDTO {
	return RiskRecordDTO{
		UserID:      record.UserID,
		ActionType:  record.ActionType,
		RiskLevel:   record.RiskLevel,
		RequestIP:   record.RequestIP,
		RequestInfo: record.RequestInfo,
		CreatedAt:   record.CreatedAt,
	}
}

// ToRiskRecord 从DTO重建风控记录领域实体（慎用，仅用于反序列化）
func ToRiskRecord(dto RiskRecordDTO) entity.RiskRecord {
	return entity.RiskRecord{
		UserID:      dto.UserID,
		ActionType:  dto.ActionType,
		RiskLevel:   dto.RiskLevel,
		RequestIP:   dto.RequestIP,
		RequestInfo: dto.RequestInfo,
		CreatedAt:   dto.CreatedAt,
	}
}

// ToRiskEvaluationDTO 将风险评估结果领域实体转换为DTO
func ToRiskEvaluationDTO(evaluation entity.RiskEvaluation) RiskEvaluationDTO {
	return RiskEvaluationDTO{
		Risk:   evaluation.Risk,
		Level:  evaluation.Level,
		Reason: evaluation.Reason,
	}
}

// ToRiskEvaluation 从DTO重建风险评估结果领域实体（慎用，仅用于反序列化）
func ToRiskEvaluation(dto RiskEvaluationDTO) entity.RiskEvaluation {
	return entity.RiskEvaluation{
		Risk:   dto.Risk,
		Level:  dto.Level,
		Reason: dto.Reason,
	}
}
