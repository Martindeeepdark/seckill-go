// Package rpc 提供 gRPC 服务相关实现
package rpc

import (
	"context"

	"seckill-risk-service/internal/domain/entity"
)

// RiskGateway 定义风险控制网关接口
// 提供风险评估、用户标记、行为记录等核心功能
type RiskGateway interface {
	// MarkSuspicious 标记可疑用户为风险用户
	// 参数：ctx-上下文，activity-活动信息，userID-用户ID，requestIP-请求IP地址
	MarkSuspicious(ctx context.Context, activity entity.Activity, userID int64, requestIP string) error
	// Evaluate 评估用户风险等级
	// 参数：ctx-上下文，userID-用户ID，requestIP-请求IP地址
	// 返回：风险评估结果和可能的错误
	Evaluate(ctx context.Context, userID int64, requestIP string) (entity.RiskEvaluation, error)
	// RecordAction 记录风险行为到持久化存储
	// 参数：ctx-上下文，record-风险记录实体
	RecordAction(ctx context.Context, record entity.RiskRecord) error
	// IsRiskUser 检查用户是否为风险用户
	// 参数：ctx-上下文，userID-用户ID
	IsRiskUser(ctx context.Context, userID int64) (bool, error)
	// CleanupExpiredRiskUsers 清理过期的风险用户
	// 参数：ctx-上下文
	// 返回：清理的记录数量和可能的错误
	CleanupExpiredRiskUsers(ctx context.Context) (int, error)
}
