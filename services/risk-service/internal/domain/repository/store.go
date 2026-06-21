// Package repository 定义风险控制相关的仓储接口
package repository

import (
	"context"
	"errors"
	"time"

	"seckill-risk-service/internal/domain/entity"
)

var (
	ErrNotFound = errors.New("not found") // 数据未找到错误
)

// RiskRepository 定义风险用户的持久化操作接口
type RiskRepository interface {
	// MarkRiskUser 标记用户为风险用户，ttl 指定过期时长
	MarkRiskUser(ctx context.Context, userID int64, ttl time.Duration) error
	// IsRiskUser 检查用户是否为风险用户
	IsRiskUser(ctx context.Context, userID int64) (bool, error)
	// RecordRiskAction 记录风险行为
	RecordRiskAction(ctx context.Context, record entity.RiskRecord) error
	// CountRecentRiskActions 统计指定时间以来的风险行为数量
	CountRecentRiskActions(ctx context.Context, userID int64, actionType string, since time.Time) (int, error)
	// HasHighRiskRecord 检查是否存在高风险记录
	HasHighRiskRecord(ctx context.Context, userID int64, since time.Time) (bool, error)
	// ListRiskRecords 列出用户的风险行为记录
	ListRiskRecords(ctx context.Context, userID int64) ([]entity.RiskRecord, error)
	// CleanupExpiredRiskUsers 清理过期的风险用户，返回清理数量
	CleanupExpiredRiskUsers(ctx context.Context) (int, error)
}
