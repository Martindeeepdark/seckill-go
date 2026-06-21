package model

import (
	"time"

	"seckill-processor-service/internal/domain/status"
)

// ActivityInfo 活动信息
type ActivityInfo struct {
	ActivityNo    string    // 活动编号
	Name          string    // 活动名称
	StartTime     time.Time // 开始时间
	EndTime       time.Time // 结束时间
	Status        int64     // 状态：0=未开始, 1=进行中, 3=已结束
	PurchaseLimit int64     // 每人限购数量
}

// IsOpen 判断活动是否正在进行
func (a ActivityInfo) IsOpen(now time.Time) bool {
	return a.Status == status.ActivityOpen &&
		!a.StartTime.After(now) &&
		a.EndTime.After(now)
}

// SKUInfo 商品 SKU 信息
type SKUInfo struct {
	ActivityNo    string // 活动编号
	SKUNo         string // SKU 编号
	TotalStock    int64  // 总库存
	SeckillPrice  int64  // 秒杀价（分）
	LimitQuantity int64  // 每单限购数量（0 表示不限制）
}

// EffectiveLimit 计算有效的限购数量
// 优先使用 SKU 级别的限购，否则使用活动级别限购
func (s SKUInfo) EffectiveLimit(activityLimit int64) int64 {
	if s.LimitQuantity > 0 {
		return s.LimitQuantity
	}
	return activityLimit
}

// CalcPrice 计算总价
func (s SKUInfo) CalcPrice(quantity int64) int64 {
	return s.SeckillPrice * quantity
}
