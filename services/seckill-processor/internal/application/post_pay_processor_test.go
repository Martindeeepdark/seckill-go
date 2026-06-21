package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"seckill-processor-service/internal/application/usecase"
	"seckill-processor-service/internal/domain"
	"seckill-processor-service/internal/domain/model"
	"seckill-processor-service/internal/domain/status"

	commonerrors "seckill-common/errors"
)

// newTestPostPayProcessor 创建测试用的 PostPayProcessor
func newTestPostPayProcessor(
	cards *postPayFakeCards,
	sync *postPayFakeSync,
) *PostPayProcessor {
	uc := usecase.NewHandlePostPay(cards, sync, slog.Default())
	return NewPostPayProcessor(uc, nil, slog.Default())
}

// ---------------------------------------------------------------------------
// PostPayProcessor table-driven tests
// ---------------------------------------------------------------------------

func TestPostPayHandle(t *testing.T) {
	tests := []struct {
		name      string
		cards     *postPayFakeCards
		sync      *postPayFakeSync
		task      model.PostPayTask
		wantErr   bool
		wantTerm  bool // true => error should be terminal
	}{
		{
			name:    "sync order success",
			cards:   nil,
			sync:    &postPayFakeSync{},
			task: model.PostPayTask{
				Type:    status.PostPayTaskSyncOrder,
				OrderNo: "O1",
				SyncOrder: &model.SyncOrderPayload{
					OrderNo:   "O1",
					UserID:    7,
					PayAmount: 9900,
				},
			},
			wantErr: false,
		},
		{
			name:    "sync order temporary error returns error for retry",
			cards:   nil,
			sync:    &postPayFakeSync{err: grpcstatus.Error(codes.Unavailable, "sync svc down")},
			task: model.PostPayTask{
				Type:    status.PostPayTaskSyncOrder,
				OrderNo: "O1",
				SyncOrder: &model.SyncOrderPayload{OrderNo: "O1"},
			},
			wantErr: true,
			wantTerm: false,
		},
		{
			name:    "sync order permanent error returns error",
			cards:   nil,
			sync:    &postPayFakeSync{err: commonerrors.WrapTerminal(errors.New("permanent sync failure"))},
			task: model.PostPayTask{
				Type:    status.PostPayTaskSyncOrder,
				OrderNo: "O1",
				SyncOrder: &model.SyncOrderPayload{OrderNo: "O1"},
			},
			wantErr: true,
			wantTerm: true,
		},
		{
			name:  "issue card success",
			cards: &postPayFakeCards{cardNo: "CARD-1"},
			sync:  nil,
			task: model.PostPayTask{
				Type:    status.PostPayTaskIssueCard,
				OrderNo: "O1",
				IssueCard: &model.IssueCardPayload{
					UserID:  7,
					OrderNo: "O1",
				},
			},
			wantErr: false,
		},
		{
			name:  "issue card error returns error",
			cards: &postPayFakeCards{err: errors.New("card service error")},
			sync:  nil,
			task: model.PostPayTask{
				Type:    status.PostPayTaskIssueCard,
				OrderNo: "O1",
				IssueCard: &model.IssueCardPayload{OrderNo: "O1"},
			},
			wantErr: true,
		},
		{
			name:  "unknown task type returns terminal error",
			cards: nil,
			sync:  nil,
			task: model.PostPayTask{
				Type:    "UNKNOWN",
				OrderNo: "O1",
			},
			wantErr: true,
			wantTerm: true,
		},
		{
			name:  "missing sync order payload returns terminal error",
			cards: nil,
			sync:  &postPayFakeSync{},
			task: model.PostPayTask{
				Type:    status.PostPayTaskSyncOrder,
				OrderNo: "O1",
			},
			wantErr: true,
			wantTerm: true,
		},
		{
			name:  "missing issue card payload returns terminal error",
			cards: &postPayFakeCards{},
			sync:  nil,
			task: model.PostPayTask{
				Type:    status.PostPayTaskIssueCard,
				OrderNo: "O1",
			},
			wantErr: true,
			wantTerm: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestPostPayProcessor(tt.cards, tt.sync)
			err := p.handle(context.Background(), tt.task)

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantTerm && err != nil {
				if !commonerrors.IsTerminal(err) {
					t.Fatalf("expected terminal error, got non-terminal: %v", err)
				}
			}
			if tt.wantErr && !tt.wantTerm && err != nil {
				// Should NOT be terminal
				if commonerrors.IsTerminal(err) {
					t.Fatalf("expected non-terminal (retriable) error, got terminal: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Existing individual tests
// ---------------------------------------------------------------------------

func TestPostPayHandle_SyncOrder(t *testing.T) {
	sync := &postPayFakeSync{}
	p := newTestPostPayProcessor(nil, sync)
	err := p.handle(context.Background(), model.PostPayTask{
		Type:    status.PostPayTaskSyncOrder,
		OrderNo: "O1",
		SyncOrder: &model.SyncOrderPayload{
			OrderNo:   "O1",
			UserID:    7,
			PayAmount: 9900,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sync.synced {
		t.Fatal("expected sync to be called")
	}
}

func TestPostPayHandle_SyncOrderError(t *testing.T) {
	sync := &postPayFakeSync{err: grpcstatus.Error(codes.Unavailable, "sync svc down")}
	p := newTestPostPayProcessor(nil, sync)
	err := p.handle(context.Background(), model.PostPayTask{
		Type:    status.PostPayTaskSyncOrder,
		OrderNo: "O1",
		SyncOrder: &model.SyncOrderPayload{OrderNo: "O1"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPostPayHandle_IssueCard(t *testing.T) {
	cards := &postPayFakeCards{cardNo: "CARD-1"}
	p := newTestPostPayProcessor(cards, nil)
	err := p.handle(context.Background(), model.PostPayTask{
		Type:    status.PostPayTaskIssueCard,
		OrderNo: "O1",
		IssueCard: &model.IssueCardPayload{
			UserID:  7,
			OrderNo: "O1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cards.issued {
		t.Fatal("expected card to be issued")
	}
}

func TestPostPayHandle_IssueCardError(t *testing.T) {
	cards := &postPayFakeCards{err: errors.New("card service error")}
	p := newTestPostPayProcessor(cards, nil)
	err := p.handle(context.Background(), model.PostPayTask{
		Type:    status.PostPayTaskIssueCard,
		OrderNo: "O1",
		IssueCard: &model.IssueCardPayload{OrderNo: "O1"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPostPayHandle_UnknownTaskType(t *testing.T) {
	p := newTestPostPayProcessor(nil, nil)
	err := p.handle(context.Background(), model.PostPayTask{
		Type:    "UNKNOWN",
		OrderNo: "O1",
	})
	if err == nil {
		t.Fatal("expected error for unknown task type")
	}
	if !errors.Is(err, domain.ErrUnknownPostPayTask) {
		t.Fatalf("expected ErrUnknownPostPayTask, got: %v", err)
	}
	if !commonerrors.IsTerminal(err) {
		t.Fatal("expected unknown task type error to be terminal")
	}
}

func TestPostPayHandle_MissingSyncOrderPayload(t *testing.T) {
	p := newTestPostPayProcessor(nil, &postPayFakeSync{})
	err := p.handle(context.Background(), model.PostPayTask{
		Type:    status.PostPayTaskSyncOrder,
		OrderNo: "O1",
	})
	if err == nil {
		t.Fatal("expected error for missing payload")
	}
	if !commonerrors.IsTerminal(err) {
		t.Fatal("expected missing payload error to be terminal")
	}
}

func TestPostPayHandle_MissingIssueCardPayload(t *testing.T) {
	p := newTestPostPayProcessor(&postPayFakeCards{}, nil)
	err := p.handle(context.Background(), model.PostPayTask{
		Type:    status.PostPayTaskIssueCard,
		OrderNo: "O1",
	})
	if err == nil {
		t.Fatal("expected error for missing payload")
	}
	if !commonerrors.IsTerminal(err) {
		t.Fatal("expected missing payload error to be terminal")
	}
}

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type postPayFakeSync struct {
	synced bool
	err    error
}

func (s *postPayFakeSync) SyncOrder(context.Context, model.SyncOrderPayload) error {
	s.synced = true
	return s.err
}

type postPayFakeCards struct {
	cardNo string
	issued bool
	err    error
}

func (c *postPayFakeCards) IssueCard(context.Context, model.IssueCardPayload) (string, error) {
	c.issued = true
	return c.cardNo, c.err
}
