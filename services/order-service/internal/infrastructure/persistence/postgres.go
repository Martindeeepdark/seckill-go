// Package persistence 提供 PostgreSQL 存储实现
package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/infrastructure/persistence/sqlcgen"
)

// DBTX interface satisfied by *pgxpool.Pool, *pgx.Conn, pgx.Tx etc.
type DBTX = sqlcgen.DBTX

// PostgresStore PostgreSQL 存储
type PostgresStore struct {
	q  *sqlcgen.Queries // SQLC 生成的查询接口
	db DBTX             // 原生 SQL 访问（用于 sqlcgen 未生成的查询，如 GetByUserAndTrace）
}

// NewPostgresStore 创建 PostgreSQL 存储实例
// conn: PostgreSQL 连接
func NewPostgresStore(db DBTX) *PostgresStore {
	return &PostgresStore{q: sqlcgen.New(db), db: db}
}

// CreateOrder 创建订单
// ctx: 上下文
// order: 订单实体
// 返回错误表示创建失败
func (s *PostgresStore) CreateOrder(ctx context.Context, order entity.Order) error {
	err := s.q.CreateOrder(ctx, sqlcgen.CreateOrderParams{
		OrderNo:     order.OrderNo,
		UserID:      order.UserID,
		ActivityNo:  order.ActivityNo,
		SkuNo:       order.SKUNo,
		Quantity:    order.Quantity,
		PayAmount:   order.PayAmount,
		OrderStatus: order.Status,
		TraceID:     text(order.TraceID),
		Remark:      "",
	})
	if err != nil {
		// 检查是否是重复键错误
		if isDuplicate(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

// GetOrder 获取订单
// ctx: 上下文
// orderNo: 订单号
// 返回订单实体和错误
func (s *PostgresStore) GetOrder(ctx context.Context, orderNo string) (entity.Order, error) {
	row, err := s.q.GetOrder(ctx, orderNo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Order{}, ErrNotFound
		}
		return entity.Order{}, fmt.Errorf("get order: %w", err)
	}
	return toEntity(row), nil
}

// GetByUserAndTrace 根据 (user_id, trace_id) 组合查询订单
// ctx: 上下文
// userID: 用户ID
// traceID: 链路追踪 ID
// 返回订单实体（未找到时返回 ErrNotFound）
// 使用 partial UNIQUE INDEX uk_sk_order_user_trace 加速查询
func (s *PostgresStore) GetByUserAndTrace(ctx context.Context, userID int64, traceID string) (entity.Order, error) {
	if traceID == "" {
		return entity.Order{}, ErrNotFound
	}
	// 走原生 SQL（sqlcgen 未生成按 user_id+trace_id 查询的 query）
	const sql = `
		SELECT id, order_no, user_id, activity_no, sku_no, quantity,
		       pay_amount, order_status, trace_id, transaction_no,
		       paid_at, closed_at, created_at
		FROM sk_order
		WHERE user_id = $1 AND trace_id = $2 AND is_deleted = 0
		LIMIT 1`
	row := s.db.QueryRow(ctx, sql, userID, traceID)

	var id int64
	var o entity.Order
	var traceIDText pgtype.Text
	var txnNo pgtype.Text
	var paidAt pgtype.Timestamp
	var closedAt pgtype.Timestamp

	if err := row.Scan(
		&id, &o.OrderNo, &o.UserID, &o.ActivityNo, &o.SKUNo, &o.Quantity,
		&o.PayAmount, &o.Status, &traceIDText, &txnNo,
		&paidAt, &closedAt, &o.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Order{}, ErrNotFound
		}
		return entity.Order{}, fmt.Errorf("get order by user_id=%d trace_id=%s: %w", userID, traceID, err)
	}
	if traceIDText.Valid {
		o.TraceID = traceIDText.String
	}
	if txnNo.Valid {
		o.TransactionNo = txnNo.String
	}
	if paidAt.Valid {
		t := paidAt.Time
		o.PaidAt = &t
	}
	if closedAt.Valid {
		t := closedAt.Time
		o.ClosedAt = &t
	}
	return o, nil
}

// ListOrdersByActivity 根据活动编号列出订单
// ctx: 上下文
// activityNo: 活动编号
// 返回订单列表和错误
func (s *PostgresStore) ListOrdersByActivity(ctx context.Context, activityNo string) ([]entity.Order, error) {
	rows, err := s.q.ListOrdersByActivity(ctx, sqlcgen.ListOrdersByActivityParams{
		ActivityNo: activityNo,
		Limit:      100,
		Offset:     0,
	})
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	return toEntities(rows), nil
}

// ListOrdersByActivities 根据多个活动编号批量列出订单
// ctx: 上下文
// activityNos: 活动编号列表
// 返回按活动编号分组的订单映射和错误
func (s *PostgresStore) ListOrdersByActivities(ctx context.Context, activityNos []string) (map[string][]entity.Order, error) {
	rows, err := s.q.ListOrdersByActivities(ctx, activityNos)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	// 按活动编号分组
	result := make(map[string][]entity.Order)
	for _, row := range rows {
		o := toEntity(row)
		result[o.ActivityNo] = append(result[o.ActivityNo], o)
	}
	return result, nil
}

// ListOrdersByUser 根据用户ID列出订单
// ctx: 上下文
// userID: 用户ID
// 返回订单列表和错误
func (s *PostgresStore) ListOrdersByUser(ctx context.Context, userID int64) ([]entity.Order, error) {
	rows, err := s.q.ListOrdersByUser(ctx, sqlcgen.ListOrdersByUserParams{
		UserID: userID,
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	return toEntities(rows), nil
}

// MarkOrderPaid 标记订单为已支付
// ctx: 上下文
// orderNo: 订单号
// transactionNo: 交易流水号
// paidAt: 支付时间
// 返回错误表示标记失败
func (s *PostgresStore) MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error {
	err := s.q.MarkOrderPaid(ctx, sqlcgen.MarkOrderPaidParams{
		OrderNo:       orderNo,
		TransactionNo: text(transactionNo),
	})
	if err != nil {
		return ErrInvalidState
	}
	return nil
}

// CloseOrder 关闭订单
// ctx: 上下文
// orderNo: 订单号
// 返回错误表示关闭失败
func (s *PostgresStore) CloseOrder(ctx context.Context, orderNo string) error {
	err := s.q.CloseOrder(ctx, orderNo)
	if err != nil {
		return ErrInvalidState
	}
	return nil
}

// toEntity 将数据库行转换为订单实体
func toEntity(row sqlcgen.SkOrder) entity.Order {
	var paidAt *time.Time
	if row.PaidAt.Valid {
		paidAt = &row.PaidAt.Time
	}
	var closedAt *time.Time
	if row.ClosedAt.Valid {
		closedAt = &row.ClosedAt.Time
	}
	return entity.Order{
		OrderNo:       row.OrderNo,
		UserID:        row.UserID,
		ActivityNo:    row.ActivityNo,
		SKUNo:         row.SkuNo,
		Quantity:      row.Quantity,
		PayAmount:     row.PayAmount,
		Status:        row.OrderStatus,
		TraceID:       row.TraceID.String,
		TransactionNo: row.TransactionNo.String,
		PaidAt:        paidAt,
		ClosedAt:      closedAt,
		CreatedAt:     row.CreatedAt,
	}
}

// toEntities 将数据库行列表转换为订单实体列表
func toEntities(rows []sqlcgen.SkOrder) []entity.Order {
	result := make([]entity.Order, len(rows))
	for i, row := range rows {
		result[i] = toEntity(row)
	}
	return result
}

// text 将字符串转换为 pgtype.Text
func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// isDuplicate 检查错误是否是 PostgreSQL unique_violation (23505)
// 使用 errors.As 提取 *pgconn.PgError，按 SQLSTATE code 精确判断，
// 不依赖错误消息字符串匹配（避免误判业务层 "duplicate" 文案）。
// 注意：pgx.ErrNoRows 不再误判为 duplicate（旧实现 bug）。
func isDuplicate(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}
