package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"seckill-common/tracing"

	paymentdomain "seckill-support-service/internal/domain/entity"
)

func TestPostPayProcessorSyncAndIssue(t *testing.T) {
	cards := &fakeFreeCardGateway{}
	sync := &fakeOrderSyncGateway{}
	p := NewPostPayProcessor(cards, sync, nil, nil)
	err := p.handle(context.Background(), paymentdomain.PostPayTask{Type: paymentdomain.PostPayTaskSyncOrder, OrderNo: "O1", SyncOrder: &paymentdomain.SyncOrderRequest{OrderNo: "O1", UserID: 7, OrderSource: "SECKILL", PayAmount: 9900, PaidAt: time.Now(), TransactionNo: "T1"}})
	if err != nil {
		t.Fatal(err)
	}
	if sync.calls != 1 {
		t.Fatalf("sync=%d", sync.calls)
	}
	err = p.handle(context.Background(), paymentdomain.PostPayTask{Type: paymentdomain.PostPayTaskIssueCard, OrderNo: "O1", IssueCard: &paymentdomain.IssueCardRequest{UserID: 7, OrderNo: "O1", CardName: "秒杀自由卡", FaceValue: 9900, ValidDays: 365}})
	if err != nil {
		t.Fatal(err)
	}
	if cards.calls != 1 {
		t.Fatalf("cards=%d", cards.calls)
	}
}

func TestPostPayProcessorError(t *testing.T) {
	p := NewPostPayProcessor(&fakeFreeCardGateway{err: errors.New("down")}, &fakeOrderSyncGateway{}, nil, nil)
	err := p.handle(context.Background(), paymentdomain.PostPayTask{Type: paymentdomain.PostPayTaskIssueCard, IssueCard: &paymentdomain.IssueCardRequest{OrderNo: "O1", UserID: 7}})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestPostPayProcessorRunWithoutConsumerWaitsForContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewPostPayProcessor(&fakeFreeCardGateway{}, &fakeOrderSyncGateway{}, nil, nil)
	if err := p.Run(ctx, "support-postpay"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context canceled", err)
	}
}

func TestPostPayTraceID(t *testing.T) {
	const tid = "55555555555555555555555555555555"
	cards := &traceCards{}
	sync := &traceSync{}
	p := NewPostPayProcessor(cards, sync, nil, nil)
	p.handle(context.Background(), paymentdomain.PostPayTask{Type: paymentdomain.PostPayTaskSyncOrder, OrderNo: "O1", RequestTraceID: tid, SyncOrder: &paymentdomain.SyncOrderRequest{OrderNo: "O1", UserID: 7, OrderSource: "SECKILL", PayAmount: 9900, PaidAt: time.Now(), TransactionNo: "T1"}})
	if sync.tid != tid {
		t.Fatalf("sync tid=%q", sync.tid)
	}
	p.handle(context.Background(), paymentdomain.PostPayTask{Type: paymentdomain.PostPayTaskIssueCard, OrderNo: "O1", RequestTraceID: tid, IssueCard: &paymentdomain.IssueCardRequest{UserID: 7, OrderNo: "O1", FaceValue: 9900}})
	if cards.tid != tid {
		t.Fatalf("cards tid=%q", cards.tid)
	}
}

type traceCards struct{ tid string }

func (g *traceCards) IssueCard(ctx context.Context, _ paymentdomain.IssueCardRequest) (string, error) {
	g.tid = tracing.TraceID(ctx)
	return "FC1", nil
}
func (g *traceCards) GetCard(context.Context, string) (paymentdomain.FreeCard, error) {
	return paymentdomain.FreeCard{}, nil
}
func (g *traceCards) ListCards(context.Context, int64) ([]paymentdomain.FreeCard, error) {
	return nil, nil
}
func (g *traceCards) ActivateCard(context.Context, paymentdomain.ActivateCardRequest) error {
	return nil
}
func (g *traceCards) FreezeCard(context.Context, string) error   { return nil }
func (g *traceCards) UnfreezeCard(context.Context, string) error { return nil }

type traceSync struct{ tid string }

func (g *traceSync) SyncOrder(ctx context.Context, _ paymentdomain.SyncOrderRequest) error {
	g.tid = tracing.TraceID(ctx)
	return nil
}
func (g *traceSync) GetSyncedOrder(context.Context, string) (paymentdomain.SyncedOrder, error) {
	return paymentdomain.SyncedOrder{}, nil
}
func (g *traceSync) ListSyncedOrders(context.Context, int64) ([]paymentdomain.SyncedOrder, error) {
	return nil, nil
}
