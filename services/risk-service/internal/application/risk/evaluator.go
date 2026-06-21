// Package risk 提供风控评估逻辑
package risk

import (
	"context"
	"fmt"
	"time"

	"seckill-risk-service/internal/domain/entity"
	"seckill-risk-service/internal/domain/repository"
)

// Evaluator 风控评估器
type Evaluator struct {
	Repo   repository.RiskRepository // 风控仓储
	Config BlackListConfig           // 黑名单配置
	Risk   RiskConfig                // 风控配置
	Clock  func() time.Time          // 时间函数（可注入）
}

// MarkSuspicious 标记可疑用户（活动开始前探测）
func (e *Evaluator) MarkSuspicious(ctx context.Context, activity entity.Activity, userID int64, requestIP string) error {
	cfg := e.normalizedBlackList()
	if !cfg.Enabled {
		return nil
	}
	now := e.now()
	windowStart := activity.StartTime.Add(-cfg.MarkStartBefore)
	windowEnd := activity.StartTime.Add(-cfg.MarkEndBefore)
	// 检查是否在探测窗口内
	if now.After(windowStart) && now.Before(windowEnd) {
		if err := e.Repo.MarkRiskUser(ctx, userID, cfg.ExpireAfter); err != nil {
			return fmt.Errorf("mark risk user: %w", err)
		}
		return e.RecordAction(ctx, entity.RiskRecord{
			UserID:      userID,
			ActionType:  entity.RiskActionPreCheck,
			RiskLevel:   entity.RiskLevelSuspicious,
			RequestIP:   requestIP,
			RequestInfo: "活动开始前探测",
			CreatedAt:   now,
		})
	}
	return nil
}

// Evaluate 评估用户风险
func (e *Evaluator) Evaluate(ctx context.Context, userID int64, requestIP string) (entity.RiskEvaluation, error) {
	// 检查黑名单
	inBlackList, err := e.Repo.IsRiskUser(ctx, userID)
	if err != nil {
		return entity.RiskEvaluation{}, fmt.Errorf("is risk user: %w", err)
	}
	if inBlackList {
		return entity.RiskEvaluation{Risk: true, Level: entity.RiskLevelHigh, Reason: "BLACKLIST"}, nil
	}

	now := e.now()
	risk := e.normalizedRisk()
	// 检查是否有高风险记录
	hasHighRisk, err := e.Repo.HasHighRiskRecord(ctx, userID, now.Add(-risk.HighRiskWindow))
	if err != nil {
		return entity.RiskEvaluation{}, fmt.Errorf("has high risk record: %w", err)
	}
	if hasHighRisk {
		if err := e.Repo.MarkRiskUser(ctx, userID, risk.RiskUserTTL); err != nil {
			return entity.RiskEvaluation{}, fmt.Errorf("mark risk user: %w", err)
		}
		return entity.RiskEvaluation{Risk: true, Level: entity.RiskLevelHigh, Reason: "HIGH_RISK_RECORD"}, nil
	}

	// 检查近期秒杀次数
	recentCount, err := e.Repo.CountRecentRiskActions(ctx, userID, entity.RiskActionSeckill, now.Add(-risk.RecentWindow))
	if err != nil {
		return entity.RiskEvaluation{}, fmt.Errorf("count recent risk actions: %w", err)
	}
	if recentCount >= risk.HighRiskThreshold {
		record := entity.RiskRecord{
			UserID:      userID,
			ActionType:  entity.RiskActionRateLimitHit,
			RiskLevel:   entity.RiskLevelHigh,
			RequestIP:   requestIP,
			RequestInfo: "1小时内秒杀次数过高",
			CreatedAt:   now,
		}
		if err := e.Repo.MarkRiskUser(ctx, userID, risk.RiskUserTTL); err != nil {
			return entity.RiskEvaluation{}, fmt.Errorf("mark risk user: %w", err)
		}
		if err := e.RecordAction(ctx, record); err != nil {
			return entity.RiskEvaluation{}, fmt.Errorf("record action: %w", err)
		}
		return entity.RiskEvaluation{Risk: true, Level: entity.RiskLevelHigh, Reason: "RECENT_SECKILL_THRESHOLD"}, nil
	}
	return entity.RiskEvaluation{Risk: false, Level: entity.RiskLevelNormal}, nil
}

// RecordAction 记录风控操作
func (e *Evaluator) RecordAction(ctx context.Context, record entity.RiskRecord) error {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = e.now()
	}
	if err := e.Repo.RecordRiskAction(ctx, record); err != nil {
		return fmt.Errorf("record risk action: %w", err)
	}
	return nil
}

// IsRiskUser 检查是否为风险用户
func (e *Evaluator) IsRiskUser(ctx context.Context, userID int64) (bool, error) {
	evaluation, err := e.Evaluate(ctx, userID, "")
	return evaluation.Risk, err
}

// CleanupExpiredRiskUsers 清理过期风险用户
func (e *Evaluator) CleanupExpiredRiskUsers(ctx context.Context) (int, error) {
	n, err := e.Repo.CleanupExpiredRiskUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired risk users: %w", err)
	}
	return n, nil
}

// now 获取当前时间
func (e *Evaluator) now() time.Time {
	if e.Clock != nil {
		return e.Clock()
	}
	return time.Now()
}

// normalizedRisk 归一化风控配置
func (e *Evaluator) normalizedRisk() RiskConfig {
	cfg := e.Risk
	if cfg.HighRiskThreshold <= 0 {
		cfg.HighRiskThreshold = 10
	}
	if cfg.RiskUserTTL <= 0 {
		cfg.RiskUserTTL = 24 * time.Hour
	}
	if cfg.RecentWindow <= 0 {
		cfg.RecentWindow = time.Hour
	}
	if cfg.HighRiskWindow <= 0 {
		cfg.HighRiskWindow = 24 * time.Hour
	}
	return cfg
}

// normalizedBlackList 归一化黑名单配置
func (e *Evaluator) normalizedBlackList() BlackListConfig {
	cfg := e.Config
	if cfg.MarkStartBefore <= 0 {
		cfg.MarkStartBefore = 300 * time.Second
	}
	if cfg.MarkEndBefore <= 0 {
		cfg.MarkEndBefore = 10 * time.Second
	}
	if cfg.ExpireAfter <= 0 {
		cfg.ExpireAfter = 300 * time.Second
	}
	return cfg
}
