package service

import (
	"context"
	"time"

	"seckill-support-service/internal/domain/entity"
)

// OrderGateway 订单网关接口
type OrderGateway interface {
	GetOrder(ctx context.Context, orderNo string) (entity.Order, error)
	MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error
}

// PaymentGateway 支付网关接口
type PaymentGateway interface {
	CreatePayment(ctx context.Context, request entity.CreatePayRequest) (entity.PayResult, error)
	QueryPayment(ctx context.Context, orderNo string) (entity.PayQueryResult, error)
	ClosePayment(ctx context.Context, orderNo string) error
}

// FreeCardGateway 自由卡网关接口
type FreeCardGateway interface {
	IssueCard(ctx context.Context, request entity.IssueCardRequest) (string, error)
	GetCard(ctx context.Context, cardNo string) (entity.FreeCard, error)
	ListCards(ctx context.Context, userID int64) ([]entity.FreeCard, error)
	ActivateCard(ctx context.Context, request entity.ActivateCardRequest) error
	FreezeCard(ctx context.Context, cardNo string) error
	UnfreezeCard(ctx context.Context, cardNo string) error
}

// OrderSyncGateway 订单同步网关接口
type OrderSyncGateway interface {
	SyncOrder(ctx context.Context, request entity.SyncOrderRequest) error
	GetSyncedOrder(ctx context.Context, orderNo string) (entity.SyncedOrder, error)
	ListSyncedOrders(ctx context.Context, userID int64) ([]entity.SyncedOrder, error)
}

// MemberGateway 会员网关接口
type MemberGateway interface {
	GetUserByID(ctx context.Context, id int64) (entity.User, error)
	GetUserByPhone(ctx context.Context, phone string) (entity.User, error)
	GetMemberLevel(ctx context.Context, id int64) (int64, error)
}
