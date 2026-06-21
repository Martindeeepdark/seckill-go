// Package entity 定义秒杀活动领域的核心实体。
package entity

import (
	"errors"
	"fmt"
	"time"

	"seckill-activity-service/internal/domain/event"
	"seckill-common/domain"
)

// Activity status constants.
const (
	ActivityPending int64 = 0 // 待开始
	ActivityOpen    int64 = 1 // 进行中
	ActivityPaused  int64 = 2 // 已暂停
	ActivityEnded   int64 = 3 // 已结束
)

// Activity 是秒杀活动聚合根。
type Activity struct {
	*domain.AggregateRoot           // 嵌入聚合根基类
	ActivityNo            string    // 活动编号
	Name                  string    // 活动名称
	StartTime             time.Time // 开始时间
	EndTime               time.Time // 结束时间
	Status                int64     // 活动状态
	PurchaseLimit         int64     // 限购数量
	Remark                string    // 备注
	SKUs                  []SKU     // 商品列表
	CreatedAt             time.Time // 创建时间
	UpdatedAt             time.Time // 更新时间
}

// Start 开始活动
func (a *Activity) Start(now time.Time) error {
	// 前置校验：状态检查
	if a.Status == ActivityEnded {
		return errors.New("activity has ended, cannot start")
	}
	if a.Status == ActivityOpen {
		return errors.New("activity is already open")
	}
	if a.Status == ActivityPaused {
		return errors.New("activity is paused, cannot start")
	}

	// 前置校验：时间检查
	if now.Before(a.StartTime) {
		return errors.New("activity cannot start before scheduled time")
	}

	// 执行状态变更
	a.Status = ActivityOpen

	// 记录领域事件
	a.RecordEvent(event.NewActivityStartedEvent(a.ActivityNo, now))

	return nil
}

// End 结束活动
func (a *Activity) End(reason string, now time.Time) error {
	// 前置校验：状态检查
	if a.Status == ActivityEnded {
		return errors.New("activity already ended")
	}

	// 执行状态变更
	a.Status = ActivityEnded

	// 记录领域事件
	a.RecordEvent(event.NewActivityEndedEvent(a.ActivityNo, reason, now))

	return nil
}

// AddSKU 添加商品
func (a *Activity) AddSKU(sku SKU) error {
	// 前置校验：状态检查
	if a.Status == ActivityEnded {
		return errors.New("activity has ended, cannot add SKU")
	}

	// 前置校验：重复检查
	if a.HasSKU(sku.SKUNo) {
		return errors.New("SKU already exists in activity")
	}

	// 执行添加
	a.SKUs = append(a.SKUs, sku)

	// 记录领域事件
	a.RecordEvent(event.NewSKUAddedEvent(a.ActivityNo, sku.SKUNo, sku.TotalStock, sku.SeckillPrice))

	return nil
}

// RemoveSKU 移除商品
func (a *Activity) RemoveSKU(skuNo string, reason string) error {
	// 前置校验：状态检查
	if a.Status == ActivityOpen {
		return errors.New("cannot remove SKU from open activity")
	}
	if a.Status == ActivityEnded {
		return errors.New("activity has ended, cannot remove SKU")
	}

	// 前置校验：存在性检查
	if !a.HasSKU(skuNo) {
		return errors.New("SKU not found in activity")
	}

	// 执行移除
	for i, sku := range a.SKUs {
		if sku.SKUNo == skuNo {
			a.SKUs = append(a.SKUs[:i], a.SKUs[i+1:]...)
			break
		}
	}

	// 记录领域事件
	a.RecordEvent(event.NewSKURemovedEvent(a.ActivityNo, skuNo, reason))

	return nil
}

// IsOpen 检查活动是否正在进行
func (a *Activity) IsOpen(now time.Time) bool {
	return a.Status == ActivityOpen && !now.Before(a.StartTime) && now.Before(a.EndTime)
}

// IsEnded 检查活动是否已结束
func (a *Activity) IsEnded() bool {
	return a.Status == ActivityEnded
}

// HasSKU 检查商品是否存在
func (a *Activity) HasSKU(skuNo string) bool {
	return a.FindSKU(skuNo) != nil
}

// FindSKU 查找商品
func (a *Activity) FindSKU(skuNo string) *SKU {
	for i := range a.SKUs {
		if a.SKUs[i].SKUNo == skuNo {
			return &a.SKUs[i]
		}
	}
	return nil
}

// CanModify 检查活动是否可修改
func (a *Activity) CanModify() error {
	if a.Status == ActivityEnded {
		return errors.New("activity has ended, cannot be modified")
	}
	return nil
}

// CanEnd 检查活动是否可结束
func (a *Activity) CanEnd() error {
	if a.Status == ActivityEnded {
		return errors.New("activity already ended")
	}
	return nil
}

// CanAddProducts 检查是否可添加商品
func (a *Activity) CanAddProducts() error {
	if a.Status == ActivityEnded {
		return errors.New("activity has ended, cannot add products")
	}
	return nil
}

// CanRemoveProducts 检查是否可移除商品
func (a *Activity) CanRemoveProducts() error {
	if a.Status == ActivityOpen {
		return errors.New("cannot remove products from an open activity")
	}
	return nil
}

// String 实现 Stringer 接口
func (a *Activity) String() string {
	return fmt.Sprintf("Activity{No: %s, Name: %s, Status: %d}", a.ActivityNo, a.Name, a.Status)
}
