// Package persistence 提供风险控制的持久化层实现
package persistence

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"seckill-risk-service/internal/domain/entity"
	"seckill-risk-service/internal/infrastructure/persistence/sqlcgen"
)

// PostgresStore PostgreSQL 存储实现，主要用于持久化风险行为记录
// 不支持临时的黑名单管理（需要 Redis）
type PostgresStore struct {
	q *sqlcgen.Queries // SQLC 生成的查询执行器
}

// NewPostgresStore 创建 PostgreSQL 存储实例
// 参数：conn-PgSQL 连接对象
// 返回：PostgresStore 实例
func NewPostgresStore(conn *pgx.Conn) *PostgresStore {
	return &PostgresStore{q: sqlcgen.New(conn)}
}

// MarkRiskUser 标记用户为风险用户
// PostgreSQL 不支持此操作（需要 Redis 管理临时黑名单）
func (s *PostgresStore) MarkRiskUser(_ context.Context, _ int64, _ time.Duration) error {
	return fmt.Errorf("MarkRiskUser requires Redis; postgres store does not manage ephemeral blacklist")
}

// IsRiskUser 检查用户是否为风险用户
// PostgreSQL 不支持此操作（需要 Redis 管理临时黑名单）
func (s *PostgresStore) IsRiskUser(_ context.Context, _ int64) (bool, error) {
	return false, fmt.Errorf("IsRiskUser requires Redis; postgres store does not manage ephemeral blacklist")
}

// RecordRiskAction 持久化风险行为记录到 PostgreSQL
// 使用 SQLC 生成的查询执行器插入数据
func (s *PostgresStore) RecordRiskAction(ctx context.Context, record entity.RiskRecord) error {
	if record.RiskLevel < 0 || record.RiskLevel > math.MaxInt16 {
		return fmt.Errorf("risk level overflow: %d", record.RiskLevel)
	}
	err := s.q.CreateRiskRecord(ctx, sqlcgen.CreateRiskRecordParams{
		UserID:      record.UserID,
		ActionType:  record.ActionType,
		RiskLevel:   int16(record.RiskLevel),
		RequestIp:   pgText(record.RequestIP),
		RequestInfo: pgText(record.RequestInfo),
	})
	if err != nil {
		return fmt.Errorf("create risk record: %w", err)
	}
	return nil
}

// CountRecentRiskActions 统计指定时间以来特定类型的风险行为数量
// 通过 SQLC 查询数据库并返回计数
func (s *PostgresStore) CountRecentRiskActions(ctx context.Context, userID int64, actionType string, since time.Time) (int, error) {
	count, err := s.q.CountRecentActions(ctx, sqlcgen.CountRecentActionsParams{
		UserID:     userID,
		ActionType: actionType,
		CreatedAt:  since,
	})
	if err != nil {
		return 0, fmt.Errorf("count recent actions: %w", err)
	}
	return int(count), nil
}

// HasHighRiskRecord 检查指定时间以来是否存在高风险记录
// 查询风险等级 >= RiskLevelHigh 的记录
func (s *PostgresStore) HasHighRiskRecord(ctx context.Context, userID int64, since time.Time) (bool, error) {
	exists, err := s.q.HasHighRiskRecord(ctx, sqlcgen.HasHighRiskRecordParams{
		UserID:    userID,
		CreatedAt: since,
	})
	if err != nil {
		return false, fmt.Errorf("has high risk record: %w", err)
	}
	return exists, nil
}

// ListRiskRecords 列出用户的风险行为记录，最多返回 100 条
// 通过 SQLC 查询数据库并将结果转换为领域实体
func (s *PostgresStore) ListRiskRecords(ctx context.Context, userID int64) ([]entity.RiskRecord, error) {
	rows, err := s.q.ListRiskRecordsByUser(ctx, sqlcgen.ListRiskRecordsByUserParams{
		UserID: userID,
		Limit:  100,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("list risk records: %w", err)
	}
	result := make([]entity.RiskRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, pgRecordToEntity(row))
	}
	return result, nil
}

// CleanupExpiredRiskUsers 清理过期的风险用户
// PostgreSQL 不支持此操作（需要 Redis 管理临时黑名单）
func (s *PostgresStore) CleanupExpiredRiskUsers(_ context.Context) (int, error) {
	return 0, fmt.Errorf("CleanupExpiredRiskUsers requires Redis; postgres store does not manage ephemeral blacklist")
}

// pgRecordToEntity 将 PostgreSQL 数据库记录转换为领域实体
func pgRecordToEntity(row sqlcgen.SkRiskRecord) entity.RiskRecord {
	return entity.RiskRecord{
		UserID:      row.UserID,
		ActionType:  row.ActionType,
		RiskLevel:   int64(row.RiskLevel),
		RequestIP:   row.RequestIp.String,
		RequestInfo: row.RequestInfo.String,
		CreatedAt:   row.CreatedAt,
	}
}

// pgText 将字符串转换为 pgtype.Text 类型，处理空字符串情况
func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
