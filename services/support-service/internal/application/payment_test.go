package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"seckill-support-service/internal/domain"
	paymentdomain "seckill-support-service/internal/domain/entity"
	supportstatus "seckill-support-service/internal/domain/status"
)

func TestPrepayCreatesPayment(t *testing.T) {
	orders := &fakeOrderGateway{order: paymentdomain.Order{OrderNo: "O1", UserID: 7, PayAmount: 9900, Status: supportstatus.OrderPending}}
	app := NewApp(orders, &fakePaymentGateway{}, nil, nil, nil)
	r, err := app.Prepay(context.Background(), 7, "O1", "wechat")
	if err != nil {
		t.Fatal(err)
	}
	if r.OrderNo != "O1" {
		t.Fatalf("orderNo=%q", r.OrderNo)
	}
}

func TestPrepayRejectsNonPending(t *testing.T) {
	orders := &fakeOrderGateway{order: paymentdomain.Order{OrderNo: "O1", UserID: 7, Status: supportstatus.OrderPaid}}
	app := NewApp(orders, &fakePaymentGateway{}, nil, nil, nil)
	_, err := app.Prepay(context.Background(), 7, "O1", "wechat")
	if err != domain.ErrOrderNotPayable {
		t.Fatalf("err=%v", err)
	}
}

func TestNotifyMarksPaid(t *testing.T) {
	orders := &fakeOrderGateway{order: paymentdomain.Order{OrderNo: "O1", UserID: 7, Status: supportstatus.OrderPending}}
	cards := &fakeFreeCardGateway{}
	sync := &fakeOrderSyncGateway{}
	app := NewApp(orders, &fakePaymentGateway{}, cards, sync, nil)
	if err := app.Notify(context.Background(), "wechat", map[string]string{"orderNo": "O1", "transactionNo": "T1"}); err != nil {
		t.Fatal(err)
	}
	if orders.order.Status != string(supportstatus.OrderPaid) {
		t.Fatalf("status=%s", orders.order.Status)
	}
	if sync.calls != 1 {
		t.Fatalf("sync=%d", sync.calls)
	}
	if cards.calls != 1 {
		t.Fatalf("cards=%d", cards.calls)
	}
}

func TestNotifySkipsLockBusy(t *testing.T) {
	orders := &fakeOrderGateway{order: paymentdomain.Order{OrderNo: "O1", UserID: 7, Status: supportstatus.OrderPending}}
	lock := &fakeLock{locked: false}
	app := NewApp(orders, &fakePaymentGateway{}, &fakeFreeCardGateway{}, &fakeOrderSyncGateway{}, nil, WithCallbackLock(lock, time.Second))
	app.Notify(context.Background(), "wechat", map[string]string{"orderNo": "O1", "transactionNo": "T1"})
	if orders.markPaidCalls != 0 {
		t.Fatalf("markPaid=%d", orders.markPaidCalls)
	}
}

func TestNotifyEnqueuesOnFailure(t *testing.T) {
	orders := &fakeOrderGateway{order: paymentdomain.Order{OrderNo: "O1", UserID: 7, PayAmount: 9900, Status: supportstatus.OrderPending}}
	cards := &fakeFreeCardGateway{err: errors.New("down")}
	sync := &fakeOrderSyncGateway{err: errors.New("down")}
	tasks := &fakePublisher{}
	app := NewApp(orders, &fakePaymentGateway{}, cards, sync, nil, WithPostPayTasks(tasks))
	app.Notify(context.Background(), "wechat", map[string]string{"orderNo": "O1", "transactionNo": "T1"})
	if len(tasks.tasks) != 2 {
		t.Fatalf("tasks=%d", len(tasks.tasks))
	}
}

func TestNotifyFallsBackInlineWhenPostPayPublishFails(t *testing.T) {
	orders := &fakeOrderGateway{order: paymentdomain.Order{OrderNo: "O1", UserID: 7, PayAmount: 9900, Status: supportstatus.OrderPending}}
	cards := &fakeFreeCardGateway{}
	sync := &fakeOrderSyncGateway{}
	tasks := &fakePublisher{err: errors.New("nats down")}
	app := NewApp(orders, &fakePaymentGateway{}, cards, sync, nil, WithPostPayTasks(tasks))
	if err := app.Notify(context.Background(), "wechat", map[string]string{"orderNo": "O1", "transactionNo": "T1"}); err != nil {
		t.Fatal(err)
	}
	if sync.calls != 1 {
		t.Fatalf("sync=%d", sync.calls)
	}
	if cards.calls != 1 {
		t.Fatalf("cards=%d", cards.calls)
	}
	if len(tasks.tasks) != 2 {
		t.Fatalf("publish attempts=%d", len(tasks.tasks))
	}
}

type fakeOrderGateway struct {
	order         paymentdomain.Order
	markPaidCalls int
}

func (g *fakeOrderGateway) GetOrder(_ context.Context, no string) (paymentdomain.Order, error) {
	if g.order.OrderNo != no {
		return paymentdomain.Order{}, domain.ErrOrderNotFound
	}
	return g.order, nil
}
func (g *fakeOrderGateway) MarkOrderPaid(_ context.Context, _ string, txn string, at time.Time) error {
	g.markPaidCalls++
	g.order.Status = string(supportstatus.OrderPaid)
	g.order.TransactionNo = txn
	g.order.PaidAt = &at
	return nil
}

type fakePaymentGateway struct{}

func (g *fakePaymentGateway) CreatePayment(_ context.Context, r paymentdomain.CreatePayRequest) (paymentdomain.PayResult, error) {
	return paymentdomain.PayResult{OrderNo: r.OrderNo, PayChannel: r.PayChannel, PrepayID: "p1"}, nil
}
func (g *fakePaymentGateway) QueryPayment(_ context.Context, no string) (paymentdomain.PayQueryResult, error) {
	return paymentdomain.PayQueryResult{OrderNo: no, PayStatus: supportstatus.PayStatusPending}, nil
}
func (g *fakePaymentGateway) ClosePayment(_ context.Context, _ string) error { return nil }

type fakeFreeCardGateway struct {
	calls int
	err   error
}

func (g *fakeFreeCardGateway) IssueCard(_ context.Context, _ paymentdomain.IssueCardRequest) (string, error) {
	g.calls++
	if g.err != nil {
		return "", g.err
	}
	return "FC1", nil
}
func (g *fakeFreeCardGateway) GetCard(_ context.Context, no string) (paymentdomain.FreeCard, error) {
	return paymentdomain.FreeCard{CardNo: no}, nil
}
func (g *fakeFreeCardGateway) ListCards(_ context.Context, _ int64) ([]paymentdomain.FreeCard, error) {
	return nil, nil
}
func (g *fakeFreeCardGateway) ActivateCard(_ context.Context, _ paymentdomain.ActivateCardRequest) error {
	return nil
}
func (g *fakeFreeCardGateway) FreezeCard(_ context.Context, _ string) error   { return nil }
func (g *fakeFreeCardGateway) UnfreezeCard(_ context.Context, _ string) error { return nil }

type fakeOrderSyncGateway struct {
	calls   int
	err     error
	lastReq paymentdomain.SyncOrderRequest
}

func (g *fakeOrderSyncGateway) SyncOrder(_ context.Context, r paymentdomain.SyncOrderRequest) error {
	g.calls++
	g.lastReq = r
	if g.err != nil {
		return g.err
	}
	return nil
}
func (g *fakeOrderSyncGateway) GetSyncedOrder(_ context.Context, no string) (paymentdomain.SyncedOrder, error) {
	return paymentdomain.SyncedOrder{OrderNo: no}, nil
}
func (g *fakeOrderSyncGateway) ListSyncedOrders(_ context.Context, _ int64) ([]paymentdomain.SyncedOrder, error) {
	return nil, nil
}

type fakePublisher struct {
	tasks []paymentdomain.PostPayTask
	err   error
}

func (p *fakePublisher) PublishPostPayTask(_ context.Context, t paymentdomain.PostPayTask) error {
	p.tasks = append(p.tasks, t)
	return p.err
}

type fakeLock struct {
	locked      bool
	tryCalls    int
	unlockCalls int
}

func (l *fakeLock) TryLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	l.tryCalls++
	return l.locked, nil
}
func (l *fakeLock) Unlock(_ context.Context, _ string) error { l.unlockCalls++; return nil }
