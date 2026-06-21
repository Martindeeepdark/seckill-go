// Package infrastructure 提供风险控制服务的基础设施层实现
package infrastructure

import (
	"context"
	"fmt"

	"seckill-risk-service/internal/domain/entity"
	"seckill-risk-service/internal/domain/repository"
	"seckill-risk-service/internal/infrastructure/rpc"
)

// LocalRiskGateway 本地风险控制网关实现
type LocalRiskGateway struct {
	Store repository.RiskRepository // 风险用户存储
}

var _ rpc.RiskGateway = (*LocalRiskGateway)(nil)

// MarkSuspicious 标记可疑用户
// 参数：ctx-上下文，activity-活动信息，userID-用户ID，requestIP-请求IP地址
func (g LocalRiskGateway) MarkSuspicious(ctx context.Context, activity entity.Activity, userID int64, requestIP string) error {
	if err := g.Store.MarkRiskUser(ctx, userID, 0); err != nil {
		return fmt.Errorf("mark risk user: %w", err)
	}
	return nil
}

// Evaluate 评估用户风险等级
// 参数：ctx-上下文，userID-用户ID，requestIP-请求IP地址
// 返回：风险评估结果和可能的错误
func (g LocalRiskGateway) Evaluate(ctx context.Context, userID int64, requestIP string) (entity.RiskEvaluation, error) {
	isRisk, err := g.Store.IsRiskUser(ctx, userID)
	if err != nil {
		return entity.RiskEvaluation{}, fmt.Errorf("is risk user: %w", err)
	}
	if isRisk {
		return entity.RiskEvaluation{Risk: true, Level: entity.RiskLevelHigh, Reason: "BLACKLIST"}, nil
	}
	return entity.RiskEvaluation{Risk: false, Level: entity.RiskLevelNormal}, nil
}

// RecordAction 记录风险行为到持久化存储
// 参数：ctx-上下文，record-风险记录实体
func (g LocalRiskGateway) RecordAction(ctx context.Context, record entity.RiskRecord) error {
	if err := g.Store.RecordRiskAction(ctx, record); err != nil {
		return fmt.Errorf("record risk action: %w", err)
	}
	return nil
}

// IsRiskUser 检查用户是否在风险用户黑名单中
// 参数：ctx-上下文，userID-用户ID
// 返回：是否为风险用户和可能的错误
func (g LocalRiskGateway) IsRiskUser(ctx context.Context, userID int64) (bool, error) {
	isRisk, err := g.Store.IsRiskUser(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("is risk user: %w", err)
	}
	return isRisk, nil
}

// CleanupExpiredRiskUsers 清理已过期的风险用户记录
// 参数：ctx-上下文
// 返回：清理的记录数量和可能的错误
func (g LocalRiskGateway) CleanupExpiredRiskUsers(ctx context.Context) (int, error) {
	count, err := g.Store.CleanupExpiredRiskUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired risk users: %w", err)
	}
	return count, nil
}
