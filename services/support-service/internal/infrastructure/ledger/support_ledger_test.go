package ledger

import (
	"context"
	"testing"
	"time"
	supportdomain "seckill-support-service/internal/domain/entity"
	supportstatus "seckill-support-service/internal/domain/status"
)

func TestSupportLedgerIssueCardIsIdempotent(t *testing.T) {
	l := NewSupportLedger(); ctx := context.Background()
	req := supportdomain.IssueCardRequest{UserID: 7, OrderNo: "O1", CardName: "秒杀自由卡", FaceValue: 9900, ValidDays: 365}
	a, _ := l.IssueCard(ctx, req); b, _ := l.IssueCard(ctx, req)
	if a != b { t.Fatalf("card no = %q then %q, want same", a, b) }
}

func TestSupportLedgerFreeCardLifecycle(t *testing.T) {
	l := NewSupportLedger(); ctx := context.Background()
	no, _ := l.IssueCard(ctx, supportdomain.IssueCardRequest{UserID: 7, OrderNo: "O1", CardName: "秒杀自由卡", FaceValue: 9900, ValidDays: 30})
	l.ActivateCard(ctx, supportdomain.ActivateCardRequest{CardNo: no, UserID: 7, OrderNo: "O1"})
	c, _ := l.GetCard(ctx, no)
	if c.Status != supportstatus.CardActive { t.Fatalf("status=%d, want active", c.Status) }
	l.FreezeCard(ctx, no)
	l.UnfreezeCard(ctx, no)
}

func TestSupportLedgerSyncOrderIsIdempotent(t *testing.T) {
	l := NewSupportLedger(); ctx := context.Background()
	req := supportdomain.SyncOrderRequest{OrderNo: "O1", UserID: 7, OrderSource: "SECKILL", TotalAmount: 9900, PayAmount: 9900, PaidAt: time.Now(), TransactionNo: "T1"}
	l.SyncOrder(ctx, req)
	req.TransactionNo = "T2"; l.SyncOrder(ctx, req)
	os, _ := l.ListSyncedOrders(ctx, 7)
	if len(os) != 1 { t.Fatalf("len=%d, want 1", len(os)) }
}
