// Package persistence 提供数据持久化层实现
package persistence

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	supportdomain "seckill-support-service/internal/domain/entity"
	supportstatus "seckill-support-service/internal/domain/status"
	"seckill-support-service/internal/infrastructure/persistence/sqlcgen"
	"seckill-support-service/internal/infrastructure/store"
)

// PostgresStore PostgreSQL存储实现
type PostgresStore struct {
	q *sqlcgen.Queries // sqlc生成的查询
}

// NewPostgresStore 创建PostgreSQL存储
func NewPostgresStore(conn *pgx.Conn) *PostgresStore {
	return &PostgresStore{q: sqlcgen.New(conn)}
}

// UpsertUser 创建或更新用户
func (s *PostgresStore) UpsertUser(ctx context.Context, user supportdomain.User) error {
	params := sqlcgen.CreateUserParams{
		Username:    user.Username,
		Phone:       pgText(user.Phone),
		Nickname:    pgText(user.Nickname),
		MemberLevel: safeInt16(user.MemberLevel),
		Status:      safeInt16(user.Status),
	}
	err := s.q.CreateUser(ctx, params)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUserByID 根据用户ID获取用户
func (s *PostgresStore) GetUserByID(ctx context.Context, id int64) (supportdomain.User, error) {
	user, err := s.q.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return supportdomain.User{}, store.ErrNotFound
		}
		return supportdomain.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return userFromDB(user), nil
}

// GetUserByPhone 根据手机号获取用户
func (s *PostgresStore) GetUserByPhone(ctx context.Context, phone string) (supportdomain.User, error) {
	// Since sqlc doesn't have a GetUserByPhone query, we need to add it or implement manually
	// For now, return not found
	return supportdomain.User{}, store.ErrNotFound
}

// GetMemberLevel 获取用户会员等级
func (s *PostgresStore) GetMemberLevel(ctx context.Context, id int64) (int64, error) {
	user, err := s.q.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get member level: %w", err)
	}
	return int64(user.MemberLevel), nil
}

// IssueCard 发放自由卡
func (s *PostgresStore) IssueCard(ctx context.Context, req supportdomain.IssueCardRequest) (string, error) {
	cardNo := generateCardNo()
	params := sqlcgen.CreateCardParams{
		CardNo:    cardNo,
		CardName:  pgText(req.CardName),
		FaceValue: req.FaceValue,
		ValidDays: safeInt32(req.ValidDays),
	}
	if err := s.q.CreateCard(ctx, params); err != nil {
		return "", fmt.Errorf("create card: %w", err)
	}
	return cardNo, nil
}

// GetCard 获取卡信息
func (s *PostgresStore) GetCard(ctx context.Context, cardNo string) (supportdomain.FreeCard, error) {
	card, err := s.q.GetCard(ctx, cardNo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return supportdomain.FreeCard{}, store.ErrNotFound
		}
		return supportdomain.FreeCard{}, fmt.Errorf("get card: %w", err)
	}
	return cardFromDB(card), nil
}

// ListCards 列出用户的卡
func (s *PostgresStore) ListCards(ctx context.Context, userID int64) ([]supportdomain.FreeCard, error) {
	cards, err := s.q.ListCardsByUser(ctx, pgInt8(userID))
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	result := make([]supportdomain.FreeCard, len(cards))
	for i, card := range cards {
		result[i] = cardFromDB(card)
	}
	return result, nil
}

// ActivateCard 激活卡
func (s *PostgresStore) ActivateCard(ctx context.Context, req supportdomain.ActivateCardRequest) error {
	params := sqlcgen.ActivateCardParams{
		CardNo:  req.CardNo,
		UserID:  pgInt8(req.UserID),
		OrderNo: pgText(req.OrderNo),
	}
	if err := s.q.ActivateCard(ctx, params); err != nil {
		return fmt.Errorf("activate card: %w", err)
	}
	return nil
}

// FreezeCard 冻结卡（待实现）
func (s *PostgresStore) FreezeCard(ctx context.Context, cardNo string) error {
	// TODO: 需要实现冻结逻辑 - 当前sqlc查询中没有
	return fmt.Errorf("freeze card not implemented in postgres")
}

// UnfreezeCard 解冻卡（待实现）
func (s *PostgresStore) UnfreezeCard(ctx context.Context, cardNo string) error {
	// TODO: 需要实现解冻逻辑 - 当前sqlc查询中没有
	return fmt.Errorf("unfreeze card not implemented in postgres")
}

// SyncOrder 同步订单（待实现）
func (s *PostgresStore) SyncOrder(ctx context.Context, req supportdomain.SyncOrderRequest) error {
	// TODO: 需要实现订单同步 - 当前sqlc查询中没有
	return fmt.Errorf("sync order not implemented in postgres")
}

// GetSyncedOrder 获取已同步订单（待实现）
func (s *PostgresStore) GetSyncedOrder(ctx context.Context, orderNo string) (supportdomain.SyncedOrder, error) {
	// TODO: 需要实现获取已同步订单 - 当前sqlc查询中没有
	return supportdomain.SyncedOrder{}, store.ErrNotFound
}

// ListSyncedOrders 列出已同步订单（待实现）
func (s *PostgresStore) ListSyncedOrders(ctx context.Context, userID int64) ([]supportdomain.SyncedOrder, error) {
	// TODO: 需要实现列出已同步订单 - 当前sqlc查询中没有
	return nil, nil
}

// ListSyncedOrdersByOrderNos 批量获取已同步订单（待实现）
func (s *PostgresStore) ListSyncedOrdersByOrderNos(ctx context.Context, orderNos []string) (map[string]supportdomain.SyncedOrder, error) {
	// TODO: 需要实现批量获取已同步订单 - 当前sqlc查询中没有
	return nil, store.ErrNotFound
}

// Payment operations
// CreatePayment 创建支付记录
func (s *PostgresStore) CreatePayment(ctx context.Context, req supportdomain.CreatePayRequest) error {
	params := sqlcgen.CreatePaymentParams{
		PaymentNo:  generatePaymentNo(),
		OrderNo:    req.OrderNo,
		UserID:     req.UserID,
		PayAmount:  req.PayAmount,
		PayChannel: req.PayChannel,
		PayStatus:  int16(supportstatus.PayStatusPending),
	}
	if err := s.q.CreatePayment(ctx, params); err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

// GetPayment 获取支付记录
func (s *PostgresStore) GetPayment(ctx context.Context, orderNo string) (supportdomain.Payment, error) {
	payment, err := s.q.GetPayment(ctx, orderNo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return supportdomain.Payment{}, store.ErrNotFound
		}
		return supportdomain.Payment{}, fmt.Errorf("get payment: %w", err)
	}
	return paymentFromDB(payment), nil
}

// UpdatePaymentStatus 更新支付状态
func (s *PostgresStore) UpdatePaymentStatus(ctx context.Context, paymentNo string, status int64, transactionNo string, paidAt *time.Time) error {
	params := sqlcgen.UpdatePaymentStatusParams{
		PaymentNo:     paymentNo,
		PayStatus:     safeInt16(status),
		TransactionNo: pgText(transactionNo),
		PaidAt:        pgTimestamptz(paidAt),
	}
	if err := s.q.UpdatePaymentStatus(ctx, params); err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}
	return nil
}

// Helper functions for pgtype conversions（pgtype类型转换辅助函数）
func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func pgInt8(i int64) pgtype.Int8 {
	return pgtype.Int8{Int64: i, Valid: true}
}

func pgTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func textToString(pg pgtype.Text) string {
	if !pg.Valid {
		return ""
	}
	return pg.String
}

func int8ToInt64(pg pgtype.Int8) int64 {
	if !pg.Valid {
		return 0
	}
	return pg.Int64
}

func timestamptzToTimePtr(pg pgtype.Timestamptz) *time.Time {
	if !pg.Valid {
		return nil
	}
	return &pg.Time
}

// Conversion functions（数据库类型转换函数）
func userFromDB(u sqlcgen.TUser) supportdomain.User {
	return supportdomain.User{
		ID:          u.ID,
		Username:    u.Username,
		Phone:       textToString(u.Phone),
		Nickname:    textToString(u.Nickname),
		MemberLevel: int64(u.MemberLevel),
		Status:      int64(u.Status),
	}
}

func cardFromDB(c sqlcgen.TFreeCard) supportdomain.FreeCard {
	return supportdomain.FreeCard{
		CardNo:      c.CardNo,
		CardName:    textToString(c.CardName),
		FaceValue:   c.FaceValue,
		UserID:      int8ToInt64(c.UserID),
		OrderNo:     textToString(c.OrderNo),
		Status:      int64(c.Status),
		ValidDays:   int64(c.ValidDays),
		ActivatedAt: timestamptzToTimePtr(c.ActivatedAt),
		ExpireAt:    timestamptzToTimePtr(c.ExpireAt),
		CreatedAt:   c.CreatedAt,
	}
}

func paymentFromDB(p sqlcgen.TPayment) supportdomain.Payment {
	return supportdomain.Payment{
		Request: supportdomain.CreatePayRequest{
			OrderNo:    p.OrderNo,
			UserID:     p.UserID,
			PayAmount:  p.PayAmount,
			PayChannel: p.PayChannel,
		},
		Status:        int64(p.PayStatus),
		TransactionNo: textToString(p.TransactionNo),
		PaidAt:        timestamptzToTimePtr(p.PaidAt),
		CreatedAt:     p.CreatedAt,
	}
}

// generateCardNo 生成卡号（简单实现）
func generateCardNo() string {
	return "FC" + time.Now().Format("20060102150405") + "000"
}

// generatePaymentNo 生成支付号（简单实现）
func generatePaymentNo() string {
	return "PAY" + time.Now().Format("20060102150405") + "000"
}

func safeInt16(v int64) int16 {
	if v > math.MaxInt16 || v < math.MinInt16 {
		panic(fmt.Sprintf("value %d overflows int16", v))
	}
	return int16(v)
}

func safeInt32(v int64) int32 {
	if v > math.MaxInt32 || v < math.MinInt32 {
		panic(fmt.Sprintf("value %d overflows int32", v))
	}
	return int32(v)
}
