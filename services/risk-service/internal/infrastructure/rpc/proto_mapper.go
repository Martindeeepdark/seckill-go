// Package rpc 提供 gRPC 服务相关实现
package rpc

import (
	"time"

	activityv1 "seckill-api/activity/v1"
	riskv1 "seckill-api/risk/v1"

	"seckill-risk-service/internal/domain/entity"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// activityFromPB 将 protobuf 活动消息转换为领域实体
// 处理 nil 情况并递归转换 SKU 列表
func activityFromPB(activity *activityv1.Activity) entity.Activity {
	if activity == nil {
		return entity.Activity{}
	}
	skus := make([]entity.SKU, 0, len(activity.GetSkus()))
	for _, sku := range activity.GetSkus() {
		skus = append(skus, skuFromPB(sku))
	}
	return entity.Activity{
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

// skuFromPB 将 protobuf SKU 消息转换为领域实体
// 处理 nil 情况并映射所有字段
func skuFromPB(sku *activityv1.SKU) entity.SKU {
	if sku == nil {
		return entity.SKU{}
	}
	return entity.SKU{
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

// riskEvaluationToPB 将风险评估实体转换为 protobuf 消息
// 用于 gRPC 响应
func riskEvaluationToPB(evaluation entity.RiskEvaluation) *riskv1.RiskEvaluation {
	return &riskv1.RiskEvaluation{
		Risk:   evaluation.Risk,
		Level:  evaluation.Level,
		Reason: evaluation.Reason,
	}
}

// riskRecordFromPB 将 protobuf 风险记录消息转换为领域实体
// 用于 gRPC 请求处理
func riskRecordFromPB(record *riskv1.RiskRecord) entity.RiskRecord {
	if record == nil {
		return entity.RiskRecord{}
	}
	return entity.RiskRecord{
		UserID:      record.GetUserId(),
		ActionType:  record.GetActionType(),
		RiskLevel:   record.GetRiskLevel(),
		RequestIP:   record.GetRequestIp(),
		RequestInfo: record.GetRequestInfo(),
		CreatedAt:   timeFromPB(record.GetCreatedAt()),
	}
}

// timeFromPB 将 protobuf 时间戳转换为 Go time.Time
// 处理 nil 情况，返回零值时间
func timeFromPB(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
