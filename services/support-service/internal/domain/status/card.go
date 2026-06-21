// Package status 提供状态机转换逻辑
package status

import (
	"seckill-support-service/internal/domain/entity"
	"slices"
	"time"
)

const (
	CardInactive = 0 // 卡未激活
	CardActive   = 1 // 卡已激活
	CardFrozen   = 2 // 卡已冻结
	CardExpired  = 3 // 卡已过期
)

// CardAllowedTransitions 卡状态允许的转换映射
var CardAllowedTransitions = map[int64][]int64{
	CardInactive: {CardActive, CardFrozen, CardExpired}, // 未激活 → 已激活/已冻结/已过期
	CardActive:   {CardFrozen, CardExpired},             // 已激活 → 已冻结/已过期
	CardFrozen:   {CardActive, CardExpired},             // 已冻结 → 已激活/已过期
	CardExpired:  {},                                    // 已过期 → 终态
}

// CanCardTransitionTo 检查卡是否可以从from状态转换到to状态
func CanCardTransitionTo(from int64, to int64) bool {
	targets, ok := CardAllowedTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(targets, to)
}

// TransitCardActive 将卡状态转换为已激活
func TransitCardActive(card entity.FreeCard, now time.Time) (entity.FreeCard, bool) {
	// 幂等处理：如果已经是激活状态，直接返回
	if card.Status == CardActive {
		return card, true
	}
	// 检查状态转换是否允许
	if !CanCardTransitionTo(card.Status, CardActive) {
		return card, false
	}
	// 更新状态
	card.Status = CardActive
	// 设置激活时间
	if card.ActivatedAt == nil {
		card.ActivatedAt = &now
	}
	// 设置有效天数（默认365天）
	vd := card.ValidDays
	if vd <= 0 {
		vd = 365
	}
	card.ValidDays = vd
	// 设置过期时间
	if card.ExpireAt == nil {
		ea := now.AddDate(0, 0, int(vd))
		card.ExpireAt = &ea
	}
	return card, true
}

// TransitCardFrozen 将卡状态转换为已冻结
func TransitCardFrozen(card entity.FreeCard) (entity.FreeCard, bool) {
	if card.Status == CardFrozen {
		return card, true
	}
	if !CanCardTransitionTo(card.Status, CardFrozen) {
		return card, false
	}
	card.Status = CardFrozen
	return card, true
}

// TransitCardExpired 将卡状态转换为已过期
func TransitCardExpired(card entity.FreeCard) (entity.FreeCard, bool) {
	if card.Status == CardExpired {
		return card, true
	}
	if !CanCardTransitionTo(card.Status, CardExpired) {
		return card, false
	}
	card.Status = CardExpired
	return card, true
}
