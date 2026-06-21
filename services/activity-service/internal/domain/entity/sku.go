// Package entity 定义秒杀活动领域的核心实体。
package entity

// SKU 是活动商品实体。
type SKU struct {
	ActivityNo    string // 所属活动编号
	SKUNo         string // 商品编号
	ProductName   string // 商品名称
	ProductImage  string // 商品图片
	OriginalPrice int64  // 原价（分）
	SeckillPrice  int64  // 秒杀价（分）
	TotalStock    int64  // 活动库存
	LimitQuantity int64  // 单次限购数量
	DiscountType  int64  // 优惠类型
	DiscountPrice int64  // 优惠金额（分）
	DiscountPct   int64  // 折扣百分比
}

// EffectiveLimit returns the purchase limit for this SKU, falling back to the activity-level limit.
func (s *SKU) EffectiveLimit(activityLimit int64) int64 {
	if s.LimitQuantity > 0 {
		return s.LimitQuantity
	}
	return activityLimit
}

// HasStock checks if there is available stock.
func (s *SKU) HasStock() bool {
	return s.TotalStock > 0
}
