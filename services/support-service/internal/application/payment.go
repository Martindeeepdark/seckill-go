// Package application 提供支持服务的应用层逻辑
// 包括支付、会员、卡券、订单同步等核心业务流程
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	commonlogs "github.com/Martindeeepdark/go-common/logs"

	"seckill-common/tracing"

	"seckill-support-service/internal/domain"
	paymentdomain "seckill-support-service/internal/domain/entity"
	"seckill-support-service/internal/domain/service"
	"seckill-support-service/internal/domain/status"
)

// 领域层 Gateway 接口的类型别名
type (
	OrderGateway    = service.OrderGateway
	PaymentGateway  = service.PaymentGateway
	FreeCardGateway = service.FreeCardGateway
	OrderSyncGateway = service.OrderSyncGateway
	MemberGateway   = service.MemberGateway
)

var (
	// ErrInvalidRequest 无效的支付请求
	ErrInvalidRequest = errors.New("invalid payment request")
	// ErrServiceUnavailable 服务暂时不可用
	ErrServiceUnavailable = errors.New("order service temporarily unavailable")
)

// PostPayTaskPublisher 支付后任务发布器接口
type PostPayTaskPublisher interface {
	PublishPostPayTask(ctx context.Context, task paymentdomain.PostPayTask) error
}

// PrepayCache 预支付缓存接口
type PrepayCache interface {
	GetPrepay(ctx context.Context, orderNo string) (paymentdomain.PayResult, bool, error)
	SetPrepay(ctx context.Context, orderNo string, result paymentdomain.PayResult, ttl time.Duration) error
	DeletePrepay(ctx context.Context, orderNo string) error
}

// PaymentCallbackLock 支付回调锁接口
type PaymentCallbackLock interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string) error
}

// AppOption 应用配置选项函数
type AppOption func(*App)

// DegradeConfig 降级配置
type DegradeConfig struct {
	SkipOrderSync bool // 跳过订单同步
	SkipCardIssue bool // 跳过发卡
}

// App 支持服务应用
type App struct {
	orders          OrderGateway         // 订单网关
	payments        PaymentGateway       // 支付网关
	cards           FreeCardGateway      // 自由卡网关
	sync            OrderSyncGateway     // 订单同步网关
	prepayCache     PrepayCache          // 预支付缓存
	prepayCacheTTL  time.Duration        // 预支付缓存过期时间
	postPayTasks    PostPayTaskPublisher // 支付后任务发布器
	callbackLock    PaymentCallbackLock  // 支付回调锁
	callbackLockTTL time.Duration        // 回调锁过期时间
	degrade         DegradeConfig        // 降级配置
	degradeProvider func() DegradeConfig // 降级配置提供者
}

// NewApp 创建支持服务应用实例
func NewApp(orders OrderGateway, payments PaymentGateway, cards FreeCardGateway, sync OrderSyncGateway, _ any, opts ...AppOption) *App {
	a := &App{orders: orders, payments: payments, cards: cards, sync: sync, prepayCacheTTL: 5 * time.Minute, callbackLockTTL: 10 * time.Second}
	for _, o := range opts {
		o(a)
	}
	return a
}

// WithPostPayTasks 设置支付后任务发布器
func WithPostPayTasks(t PostPayTaskPublisher) AppOption { return func(a *App) { a.postPayTasks = t } }

// WithPrepayCache 设置预支付缓存
func WithPrepayCache(c PrepayCache, ttl time.Duration) AppOption {
	return func(a *App) {
		a.prepayCache = c
		if ttl > 0 {
			a.prepayCacheTTL = ttl
		}
	}
}

// WithCallbackLock 设置支付回调锁
func WithCallbackLock(l PaymentCallbackLock, ttl time.Duration) AppOption {
	return func(a *App) {
		a.callbackLock = l
		if ttl > 0 {
			a.callbackLockTTL = ttl
		}
	}
}

// WithDegrade 设置降级配置
func WithDegrade(d DegradeConfig) AppOption { return func(a *App) { a.degrade = d } }

// WithDegradeProvider 设置降级配置提供者
func WithDegradeProvider(p func() DegradeConfig) AppOption {
	return func(a *App) { a.degradeProvider = p }
}

// Prepay 创建预支付订单
func (a *App) Prepay(ctx context.Context, userID int64, orderNo string, payChannel string) (paymentdomain.PayResult, error) {
	orderNo = strings.TrimSpace(orderNo)
	payChannel = strings.TrimSpace(payChannel)
	if orderNo == "" {
		return paymentdomain.PayResult{}, ErrInvalidRequest
	}
	if payChannel == "" {
		payChannel = "mock"
	}
	order, err := a.orders.GetOrder(ctx, orderNo)
	if err != nil {
		if errors.Is(err, domain.ErrCircuitOpen) {
			return paymentdomain.PayResult{}, ErrServiceUnavailable
		}
		return paymentdomain.PayResult{}, domain.ErrOrderNotFound
	}
	if order.UserID != userID {
		return paymentdomain.PayResult{}, domain.ErrForbidden
	}
	if order.Status != status.OrderPending {
		return paymentdomain.PayResult{}, domain.ErrOrderNotPayable
	}
	if a.prepayCache != nil {
		r, ok, err := a.prepayCache.GetPrepay(ctx, orderNo)
		if err != nil {
			commonlogs.CtxWarnf(ctx, "get prepay cache failed orderNo=%s error=%v", orderNo, err)
		}
		if ok {
			return r, nil
		}
	}
	r, err := a.payments.CreatePayment(ctx, paymentdomain.CreatePayRequest{OrderNo: order.OrderNo, UserID: order.UserID, PayAmount: order.PayAmount, PayChannel: payChannel, Subject: "秒杀订单-" + order.OrderNo, ExpireAt: time.Now().Add(310 * time.Second)})
	if err != nil {
		return paymentdomain.PayResult{}, fmt.Errorf("create payment: %w", err)
	}
	if a.prepayCache != nil {
		if err := a.prepayCache.SetPrepay(ctx, orderNo, r, a.prepayCacheTTL); err != nil {
			commonlogs.CtxWarnf(ctx, "set prepay cache failed orderNo=%s error=%v", orderNo, err)
		}
	}
	return r, nil
}

// Notify 处理支付回调通知
func (a *App) Notify(ctx context.Context, channel string, params map[string]string) error {
	_ = channel
	orderNo := strings.TrimSpace(params["orderNo"])
	txn := strings.TrimSpace(params["transactionNo"])
	if orderNo == "" || txn == "" {
		return ErrInvalidRequest
	}
	if a.callbackLock != nil {
		key := "seckill:payment:update:" + orderNo
		locked, _ := a.callbackLock.TryLock(ctx, key, a.callbackLockTTL) //nolint:errcheck // best-effort lock
		if !locked {
			return nil
		}
		defer a.callbackLock.Unlock(context.Background(), key) //nolint:errcheck // best-effort cleanup
	}
	order, err := a.orders.GetOrder(ctx, orderNo)
	if err != nil {
		if errors.Is(err, domain.ErrCircuitOpen) {
			return ErrServiceUnavailable
		}
		return nil
	}
	if order.Status != status.OrderPending {
		return nil
	}
	paidAt := time.Now()
	if err := a.orders.MarkOrderPaid(ctx, orderNo, txn, paidAt); err != nil {
		return fmt.Errorf("mark order paid: %w", err)
	}
	if a.postPayTasks != nil {
		syncQueued, cardQueued := a.publishPostPayTasks(ctx, order, txn, paidAt)
		if !syncQueued {
			a.syncPaidOrder(ctx, order, txn, paidAt)
		}
		if !cardQueued {
			a.issueCardAfterPay(ctx, order)
		}
		return nil
	}
	a.syncPaidOrder(ctx, order, txn, paidAt)
	a.issueCardAfterPay(ctx, order)
	return nil
}

// syncPaidOrder 同步已支付订单
func (a *App) syncPaidOrder(ctx context.Context, order paymentdomain.Order, txn string, paidAt time.Time) {
	if a.currentDegrade().SkipOrderSync || a.sync == nil {
		return
	}
	req := paymentdomain.SyncOrderRequest{OrderNo: order.OrderNo, UserID: order.UserID, OrderSource: "SECKILL", TotalAmount: order.PayAmount, PayAmount: order.PayAmount, PaidAt: paidAt, TransactionNo: txn}
	if err := a.sync.SyncOrder(ctx, req); err != nil {
		commonlogs.CtxWarnf(ctx, "sync paid order failed orderNo=%s error=%v", order.OrderNo, err)
	}
}

// issueCardAfterPay 支付后发卡
func (a *App) issueCardAfterPay(ctx context.Context, order paymentdomain.Order) {
	if a.currentDegrade().SkipCardIssue || a.cards == nil {
		return
	}
	req := paymentdomain.IssueCardRequest{UserID: order.UserID, OrderNo: order.OrderNo, CardName: "秒杀自由卡", FaceValue: order.PayAmount, ValidDays: 365}
	if _, err := a.cards.IssueCard(ctx, req); err != nil {
		commonlogs.CtxWarnf(ctx, "issue card after pay failed orderNo=%s error=%v", order.OrderNo, err)
	}
}

// publishPostPayTasks 发布支付后任务
func (a *App) publishPostPayTasks(ctx context.Context, order paymentdomain.Order, txn string, paidAt time.Time) (bool, bool) {
	traceID := tracing.TraceID(ctx)
	syncQueued := true
	syncReq := paymentdomain.SyncOrderRequest{OrderNo: order.OrderNo, UserID: order.UserID, OrderSource: "SECKILL", TotalAmount: order.PayAmount, PayAmount: order.PayAmount, PaidAt: paidAt, TransactionNo: txn}
	if err := a.postPayTasks.PublishPostPayTask(ctx, paymentdomain.PostPayTask{Type: paymentdomain.PostPayTaskSyncOrder, OrderNo: order.OrderNo, SyncOrder: &syncReq, RequestTraceID: traceID}); err != nil {
		syncQueued = false
		commonlogs.CtxWarnf(ctx, "publish sync-order post-pay task failed orderNo=%s error=%v", order.OrderNo, err)
	}
	cardQueued := true
	cardReq := paymentdomain.IssueCardRequest{UserID: order.UserID, OrderNo: order.OrderNo, CardName: "秒杀自由卡", FaceValue: order.PayAmount, ValidDays: 365}
	if err := a.postPayTasks.PublishPostPayTask(ctx, paymentdomain.PostPayTask{Type: paymentdomain.PostPayTaskIssueCard, OrderNo: order.OrderNo, IssueCard: &cardReq, RequestTraceID: traceID}); err != nil {
		cardQueued = false
		commonlogs.CtxWarnf(ctx, "publish issue-card post-pay task failed orderNo=%s error=%v", order.OrderNo, err)
	}
	return syncQueued, cardQueued
}

// currentDegrade 获取当前降级配置
func (a *App) currentDegrade() DegradeConfig {
	if a.degradeProvider != nil {
		return a.degradeProvider()
	}
	return a.degrade
}
