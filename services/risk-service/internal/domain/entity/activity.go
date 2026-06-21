// Package entity 提供秒杀活动的领域实体定义
package entity

import "time"

// Activity 表示秒杀活动实体
type Activity struct {
	ActivityNo    string    // 活动编号
	Name          string    // 活动名称
	StartTime     time.Time // 开始时间
	EndTime       time.Time // 结束时间
	Status        int64     // 活动状态：1-未开始，2-进行中，3-已结束，4-已取消
	PurchaseLimit int64     // 每人最大购买数量
	Remark        string    // 备注信息
	SKUs          []SKU     // SKU 列表
	CreatedAt     time.Time // 创建时间
	UpdatedAt     time.Time // 更新时间
}

// SKU 表示秒杀商品库存单位
type SKU struct {
	ActivityNo    string // 所属活动编号
	SKUNo         string // SKU 编号
	ProductName   string // 商品名称
	ProductImage  string // 商品图片URL
	OriginalPrice int64  // 原价（分）
	SeckillPrice  int64  // 秒杀价（分）
	TotalStock    int64  // 总库存数量
	LimitQuantity int64  // 每人限购数量
	DiscountType  int64  // 优惠类型：1-减免金额，2-折扣百分比
	DiscountPrice int64  // 减免金额（分），当 DiscountType=1 时使用
	DiscountPct   int64  // 折扣百分比（基数为 10000），当 DiscountType=2 时使用，例如 8000 表示 80% 折扣（即原价的 20%）
}
