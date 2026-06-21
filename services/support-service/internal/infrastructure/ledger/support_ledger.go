// Package ledger 提供内存账本实现（用于开发和测试）
package ledger

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"

	supportdomain "seckill-support-service/internal/domain/entity"
	supportstatus "seckill-support-service/internal/domain/status"
	"seckill-support-service/internal/infrastructure/store"

	"github.com/Martindeeepdark/go-common/snowflake"
)

// SupportLedger 支持服务内存账本
type SupportLedger struct {
	mu            sync.Mutex                                   // 互斥锁
	usersByID     map[int64]supportdomain.User                // 用户ID到用户的映射
	userIDByPhone map[string]int64                            // 手机号到用户ID的映射
	cardsByNo     map[string]supportdomain.FreeCard           // 卡号到卡的映射
	cardNoByOrder map[string]string                           // 订单号到卡号的映射
	orders        map[string]supportdomain.SyncedOrder         // 订单号到同步订单的映射
}

// NewSupportLedger 创建支持服务账本
func NewSupportLedger() *SupportLedger {
	return &SupportLedger{
		usersByID: map[int64]supportdomain.User{}, userIDByPhone: map[string]int64{},
		cardsByNo: map[string]supportdomain.FreeCard{}, cardNoByOrder: map[string]string{},
		orders: map[string]supportdomain.SyncedOrder{},
	}
}

// UpsertUser 创建或更新用户
func (l *SupportLedger) UpsertUser(_ context.Context, user supportdomain.User) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if user.ID == 0 {
		return store.ErrInvalidState
	}
	// 更新手机号索引：如果手机号发生变化，删除旧索引
	if old, ok := l.usersByID[user.ID]; ok && old.Phone != "" && old.Phone != user.Phone {
		delete(l.userIDByPhone, old.Phone)
	}
	// 更新用户数据
	l.usersByID[user.ID] = user
	if user.Phone != "" {
		l.userIDByPhone[user.Phone] = user.ID
	}
	return nil
}

// GetUserByID 根据用户ID获取用户
func (l *SupportLedger) GetUserByID(_ context.Context, id int64) (supportdomain.User, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	u, ok := l.usersByID[id]
	if !ok {
		return supportdomain.User{}, store.ErrNotFound
	}
	return u, nil
}

// GetUserByPhone 根据手机号获取用户
func (l *SupportLedger) GetUserByPhone(_ context.Context, phone string) (supportdomain.User, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id, ok := l.userIDByPhone[phone]
	if !ok {
		return supportdomain.User{}, store.ErrNotFound
	}
	return l.usersByID[id], nil
}

// GetMemberLevel 获取用户会员等级
func (l *SupportLedger) GetMemberLevel(_ context.Context, id int64) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	u, ok := l.usersByID[id]
	if !ok {
		return 0, nil
	}
	return u.MemberLevel, nil
}

// IssueCard 发放自由卡
func (l *SupportLedger) IssueCard(_ context.Context, req supportdomain.IssueCardRequest) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if no := l.cardNoByOrder[req.OrderNo]; no != "" {
		return no, nil
	}
	no := "FC" + strconv.FormatInt(snowflake.NewID(), 10)
	vd := req.ValidDays
	if vd <= 0 {
		vd = 365
	}
	l.cardsByNo[no] = supportdomain.FreeCard{CardNo: no, CardName: req.CardName, FaceValue: req.FaceValue, UserID: req.UserID, OrderNo: req.OrderNo, Status: supportstatus.CardInactive, ValidDays: vd, CreatedAt: time.Now()}
	l.cardNoByOrder[req.OrderNo] = no
	return no, nil
}

// GetCard 获取卡信息
func (l *SupportLedger) GetCard(_ context.Context, no string) (supportdomain.FreeCard, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.cardsByNo[no]
	if !ok {
		return supportdomain.FreeCard{}, store.ErrNotFound
	}
	return c, nil
}

// ListCards 列出用户的卡
func (l *SupportLedger) ListCards(_ context.Context, uid int64) ([]supportdomain.FreeCard, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []supportdomain.FreeCard
	for _, c := range l.cardsByNo {
		if c.UserID == uid {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// ActivateCard 激活卡
func (l *SupportLedger) ActivateCard(_ context.Context, req supportdomain.ActivateCardRequest) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.cardsByNo[req.CardNo]
	if !ok {
		return store.ErrNotFound
	}
	if req.UserID != 0 && c.UserID != req.UserID {
		return store.ErrInvalidState
	}
	if req.OrderNo != "" && c.OrderNo != req.OrderNo {
		return store.ErrInvalidState
	}
	u, ok := supportstatus.TransitCardActive(c, time.Now())
	if !ok {
		return store.ErrInvalidState
	}
	l.cardsByNo[u.CardNo] = u
	return nil
}

// FreezeCard 冻结卡
func (l *SupportLedger) FreezeCard(_ context.Context, no string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.cardsByNo[no]
	if !ok {
		return store.ErrNotFound
	}
	u, ok := supportstatus.TransitCardFrozen(c)
	if !ok {
		return store.ErrInvalidState
	}
	l.cardsByNo[u.CardNo] = u
	return nil
}

// UnfreezeCard 解冻卡
func (l *SupportLedger) UnfreezeCard(_ context.Context, no string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.cardsByNo[no]
	if !ok {
		return store.ErrNotFound
	}
	if c.Status != supportstatus.CardFrozen && c.Status != supportstatus.CardActive {
		return store.ErrInvalidState
	}
	u, ok := supportstatus.TransitCardActive(c, time.Now())
	if !ok {
		return store.ErrInvalidState
	}
	l.cardsByNo[u.CardNo] = u
	return nil
}

// SyncOrder 同步订单
func (l *SupportLedger) SyncOrder(_ context.Context, req supportdomain.SyncOrderRequest) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.orders[req.OrderNo]; ok {
		return nil
	}
	l.orders[req.OrderNo] = supportdomain.SyncedOrder{OrderNo: req.OrderNo, UserID: req.UserID, OrderSource: req.OrderSource, TotalAmount: req.TotalAmount, DiscountAmount: req.DiscountAmount, PayAmount: req.PayAmount, OrderStatus: 1, PaidAt: req.PaidAt, TransactionNo: req.TransactionNo, CreatedAt: time.Now()}
	return nil
}

// GetSyncedOrder 获取已同步订单
func (l *SupportLedger) GetSyncedOrder(_ context.Context, no string) (supportdomain.SyncedOrder, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	o, ok := l.orders[no]
	if !ok {
		return supportdomain.SyncedOrder{}, store.ErrNotFound
	}
	return o, nil
}

// ListSyncedOrdersByOrderNos 根据订单号列表批量获取已同步订单
func (l *SupportLedger) ListSyncedOrdersByOrderNos(_ context.Context, nos []string) (map[string]supportdomain.SyncedOrder, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r := make(map[string]supportdomain.SyncedOrder, len(nos))
	for _, no := range nos {
		if o, ok := l.orders[no]; ok {
			r[no] = o
		}
	}
	return r, nil
}

// ListSyncedOrders 列出用户的已同步订单
func (l *SupportLedger) ListSyncedOrders(_ context.Context, uid int64) ([]supportdomain.SyncedOrder, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []supportdomain.SyncedOrder
	for _, o := range l.orders {
		if o.UserID == uid {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
