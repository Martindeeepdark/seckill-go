// Package persistence 提供基于 PostgreSQL 的仓储实现。
package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	domain "seckill-activity-service/internal/domain/entity"
	"seckill-activity-service/internal/infrastructure/persistence/sqlcgen"
)

// PostgresStore 是基于 PostgreSQL 的仓储实现。
type PostgresStore struct {
	q *sqlcgen.Queries
}

// NewPostgresStore 创建 PostgreSQL 仓储实例。
func NewPostgresStore(conn *pgx.Conn) *PostgresStore {
	return &PostgresStore{q: sqlcgen.New(conn)}
}

// AddActivity 将活动及其 SKU 写入 PostgreSQL，重复活动编号返回 ErrDuplicate。
func (s *PostgresStore) AddActivity(ctx context.Context, activity domain.Activity) error {
	err := s.q.CreateActivity(ctx, sqlcgen.CreateActivityParams{
		ActivityNo:     activity.ActivityNo,
		ActivityName:   activity.Name,
		StartTime:      activity.StartTime,
		EndTime:        activity.EndTime,
		EffectiveType:  0,
		EffectiveDays:  pgtype.Text{Valid: false},
		EffectiveStart: pgtype.Text{Valid: false},
		EffectiveEnd:   pgtype.Text{Valid: false},
		ActivityStatus: int16(activity.Status),   //nolint:gosec // G115: safe narrow conversion, value bounded by domain constraints
		PurchaseLimit:  int32(activity.PurchaseLimit), //nolint:gosec // G115: safe narrow conversion, value bounded by domain constraints
		Remark:         pgText(activity.Remark),
	})
	if err != nil {
		if isDuplicatePG(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("create activity: %w", err)
	}
	for _, sku := range activity.SKUs {
		if err := s.upsertProductAndSKU(ctx, activity.ActivityNo, sku); err != nil {
			return fmt.Errorf("upsert sku %s: %w", sku.SKUNo, err)
		}
	}
	return nil
}

// UpdateActivity 更新活动基本信息（名称、时间、状态、限购、备注）。
func (s *PostgresStore) UpdateActivity(ctx context.Context, activity domain.Activity) error {
	err := s.q.UpdateActivity(ctx, sqlcgen.UpdateActivityParams{
		ActivityNo:     activity.ActivityNo,
		ActivityName:   activity.Name,
		StartTime:      activity.StartTime,
		EndTime:        activity.EndTime,
		ActivityStatus: int16(activity.Status),   //nolint:gosec // G115: safe narrow conversion, value bounded by domain constraints
		PurchaseLimit:  int32(activity.PurchaseLimit), //nolint:gosec // G115: safe narrow conversion, value bounded by domain constraints
		Remark:         pgText(activity.Remark),
	})
	if err != nil {
		return fmt.Errorf("update activity: %w", err)
	}
	return nil
}

// UpdateActivityStatus 更新指定活动的状态。
func (s *PostgresStore) UpdateActivityStatus(ctx context.Context, activityNo string, status int64) error {
	err := s.q.UpdateActivityStatus(ctx, sqlcgen.UpdateActivityStatusParams{
		ActivityNo:     activityNo,
		ActivityStatus: int16(status), //nolint:gosec // G115: safe narrow conversion, value bounded by domain constraints
	})
	if err != nil {
		return fmt.Errorf("update activity status: %w", err)
	}
	return nil
}

// AddActivitySKU 向活动追加 SKU，幂等 upsert 商品与 SKU 记录。
func (s *PostgresStore) AddActivitySKU(ctx context.Context, activityNo string, sku domain.SKU) error {
	return s.upsertProductAndSKU(ctx, activityNo, sku)
}

// RemoveActivitySKU PostgreSQL 仓储暂未实现 SKU 移除，调用即返回错误。
func (s *PostgresStore) RemoveActivitySKU(ctx context.Context, activityNo, skuNo string) error {
	return fmt.Errorf("RemoveActivitySKU not yet implemented for postgres store")
}

// ListActivities 列出活动及其 SKU 列表（上限 100 条）。
func (s *PostgresStore) ListActivities(ctx context.Context) ([]domain.Activity, error) {
	rows, err := s.q.ListActivities(ctx, sqlcgen.ListActivitiesParams{
		Limit:          100,
		Offset:         0,
		ActivityStatus: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	result := make([]domain.Activity, 0, len(rows))
	for _, row := range rows {
		activity := pgActivityToEntity(row)
		skus, err := s.listSKUsForActivity(ctx, row.ActivityNo)
		if err != nil {
			return nil, err
		}
		activity.SKUs = skus
		result = append(result, activity)
	}
	return result, nil
}

// GetActivity 根据活动编号加载活动聚合（含 SKU），不存在返回 ErrNotFound。
func (s *PostgresStore) GetActivity(ctx context.Context, activityNo string) (domain.Activity, error) {
	row, err := s.q.GetActivity(ctx, activityNo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Activity{}, ErrNotFound
		}
		return domain.Activity{}, fmt.Errorf("get activity: %w", err)
	}
	activity := pgActivityToEntity(row)
	skus, err := s.listSKUsForActivity(ctx, activityNo)
	if err != nil {
		return domain.Activity{}, err
	}
	activity.SKUs = skus
	return activity, nil
}

// GetSKU 根据 SKU 编号加载商品信息，不存在返回 ErrNotFound。
func (s *PostgresStore) GetSKU(ctx context.Context, activityNo, skuNo string) (domain.SKU, error) {
	row, err := s.q.GetSKU(ctx, skuNo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SKU{}, ErrNotFound
		}
		return domain.SKU{}, fmt.Errorf("get sku: %w", err)
	}
	return pgSKUToEntity(row, ""), nil
}

// PeekStock 实时库存查询需经 Redis，PostgreSQL 仓储不支持，调用即返回错误。
func (s *PostgresStore) PeekStock(_ context.Context, _, _ string) (int64, error) {
	return 0, fmt.Errorf("PeekStock requires Redis; postgres store does not manage real-time stock")
}

// DeductStockWithLimit 原子扣减库存需经 Redis，PostgreSQL 仓储不支持，调用即返回错误。
func (s *PostgresStore) DeductStockWithLimit(_ context.Context, _, _ string, _ int64, _ int64, _ int64) (bool, error) {
	return false, fmt.Errorf("DeductStockWithLimit requires Redis; postgres store does not manage real-time stock")
}

// ReleaseStock 回滚库存需经 Redis，PostgreSQL 仓储不支持，调用即返回错误。
func (s *PostgresStore) ReleaseStock(_ context.Context, _, _ string, _ int64, _ int64) error {
	return fmt.Errorf("ReleaseStock requires Redis; postgres store does not manage real-time stock")
}

// CleanupActivityStock 清理活动库存键需经 Redis，PostgreSQL 仓储不支持，调用即返回错误。
func (s *PostgresStore) CleanupActivityStock(_ context.Context, _ string, _ []string) (int64, error) {
	return 0, fmt.Errorf("CleanupActivityStock requires Redis; postgres store does not manage real-time stock")
}

// CleanupActivityPurchases 清理活动购买记录需经 Redis，PostgreSQL 仓储不支持，调用即返回错误。
func (s *PostgresStore) CleanupActivityPurchases(_ context.Context, _ string) (int64, error) {
	return 0, fmt.Errorf("CleanupActivityPurchases requires Redis; postgres store does not manage real-time purchases")
}

// TryStartTrace 链路追踪需经 Redis，PostgreSQL 仓储不支持，调用即返回错误。
func (s *PostgresStore) TryStartTrace(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return false, fmt.Errorf("TryStartTrace requires Redis; postgres store does not manage distributed traces")
}

// MarkTraceSuccess 标记链路成功需经 Redis，PostgreSQL 仓储不支持，调用即返回错误。
func (s *PostgresStore) MarkTraceSuccess(_ context.Context, _, _ string, _ time.Duration) error {
	return fmt.Errorf("MarkTraceSuccess requires Redis; postgres store does not manage distributed traces")
}

// MarkTraceFail 标记链路失败需经 Redis，PostgreSQL 仓储不支持，调用即返回错误。
func (s *PostgresStore) MarkTraceFail(_ context.Context, _, _ string, _ time.Duration) error {
	return fmt.Errorf("MarkTraceFail requires Redis; postgres store does not manage distributed traces")
}

// GetTraceResult 读取链路结果需经 Redis，PostgreSQL 仓储不支持，调用即返回错误。
func (s *PostgresStore) GetTraceResult(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("GetTraceResult requires Redis; postgres store does not manage distributed traces")
}

// DeleteTrace 删除链路结果需经 Redis，PostgreSQL 仓储不支持，调用即返回错误。
func (s *PostgresStore) DeleteTrace(_ context.Context, _ string) error {
	return fmt.Errorf("DeleteTrace requires Redis; postgres store does not manage distributed traces")
}

func (s *PostgresStore) upsertProductAndSKU(ctx context.Context, activityNo string, sku domain.SKU) error {
	if err := s.q.CreateProduct(ctx, sqlcgen.CreateProductParams{
		ActivityNo:    activityNo,
		ProductName:   sku.ProductName,
		ProductImage:  pgText(sku.ProductImage),
		OriginalPrice: sku.OriginalPrice,
		DiscountType:  int16(sku.DiscountType),
		DiscountPrice: pgInt8(sku.DiscountPrice),
		SortOrder:     0,
	}); err != nil {
		if !isDuplicatePG(err) {
			return fmt.Errorf("create product: %w", err)
		}
	}
	if err := s.q.CreateSKU(ctx, sqlcgen.CreateSKUParams{
		ActivityNo:      activityNo,
		ProductID:       0,
		SkuNo:           sku.SKUNo,
		ActivityStock:   int32(sku.TotalStock), //nolint:gosec // G115: safe narrow conversion, value bounded by domain constraints
		DiscountType:    int16(sku.DiscountType),
		DiscountPercent: pgInt4(sku.DiscountPct),
		DiscountPrice:   pgInt8(sku.DiscountPrice),
	}); err != nil {
		if !isDuplicatePG(err) {
			return fmt.Errorf("create sku: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) listSKUsForActivity(ctx context.Context, activityNo string) ([]domain.SKU, error) {
	rows, err := s.q.ListSKUsByActivity(ctx, activityNo)
	if err != nil {
		return nil, fmt.Errorf("list skus: %w", err)
	}
	skus := make([]domain.SKU, 0, len(rows))
	for _, row := range rows {
		skus = append(skus, pgSKUToEntity(row, activityNo))
	}
	return skus, nil
}

func pgActivityToEntity(row sqlcgen.SkActivity) domain.Activity {
	return domain.Activity{
		ActivityNo:    row.ActivityNo,
		Name:          row.ActivityName,
		StartTime:     row.StartTime,
		EndTime:       row.EndTime,
		Status:        int64(row.ActivityStatus),
		PurchaseLimit: int64(row.PurchaseLimit),
		Remark:        row.Remark.String,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func pgSKUToEntity(row sqlcgen.SkActivityProductSku, activityNo string) domain.SKU {
	return domain.SKU{
		ActivityNo:    coalesceStr(activityNo, row.ActivityNo),
		SKUNo:         row.SkuNo,
		TotalStock:    int64(row.ActivityStock),
		DiscountType:  int64(row.DiscountType),
		DiscountPct:   pgInt4Value(row.DiscountPercent),
		DiscountPrice: pgInt8Value(row.DiscountPrice),
	}
}

func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func pgInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: true}
}

func pgInt8Value(v pgtype.Int8) int64 {
	if v.Valid {
		return v.Int64
	}
	return 0
}

func pgInt4(v int64) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func pgInt4Value(v pgtype.Int4) int64 {
	if v.Valid {
		return int64(v.Int32)
	}
	return 0
}

func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func isDuplicatePG(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsSub(msg, "duplicate") || containsSub(msg, "23505")
}

func containsSub(s, sub string) bool {
	return len(s) >= len(sub) && findSub(s, sub)
}

func findSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
