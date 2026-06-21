package handler

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"seckill-gateway-service/internal/application"
)

func TestPaymentNotifyAcceptsJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payments := &fakePaymentGateway{}
	app := application.NewPaymentApp(payments, slog.Default())
	router := gin.New()
	NewPaymentHandler(app).Register(router)

	req := httptest.NewRequest(http.MethodPost, "/api/pay/notify/mock", bytes.NewBufferString(`{"orderNo":"O1","transactionNo":"T1"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s, want 200", resp.Code, resp.Body.String())
	}
	if resp.Body.String() != "SUCCESS" {
		t.Fatalf("body = %q, want SUCCESS", resp.Body.String())
	}
	if !payments.notifyCalled {
		t.Fatal("Notify was not called")
	}
	if payments.notifyReq.OrderNo != "O1" || payments.notifyReq.TransactionNo != "T1" || payments.notifyReq.Channel != "mock" {
		t.Fatalf("Notify args = %+v, want O1/T1/mock", payments.notifyReq)
	}
}

func TestPaymentNotifyRejectsMissingTransactionNo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := application.NewPaymentApp(&fakePaymentGateway{}, slog.Default())
	router := gin.New()
	NewPaymentHandler(app).Register(router)

	req := httptest.NewRequest(http.MethodPost, "/api/pay/notify/mock", bytes.NewBufferString(`{"orderNo":"O1"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s, want 400", resp.Code, resp.Body.String())
	}
	if resp.Body.String() != "MISSING_TRANSACTION_NO" {
		t.Fatalf("body = %q, want MISSING_TRANSACTION_NO", resp.Body.String())
	}
}

type fakePaymentGateway struct {
	notifyCalled bool
	notifyReq    application.PaymentNotifyRequest
}

func (g *fakePaymentGateway) Prepay(_ context.Context, req application.PrepayPaymentRequest) (*application.PayResult, error) {
	return &application.PayResult{
		OrderNo:    req.OrderNo,
		PayChannel: req.PayChannel,
		PrepayID:   "prepay-1",
	}, nil
}

func (g *fakePaymentGateway) Notify(_ context.Context, req application.PaymentNotifyRequest) error {
	g.notifyCalled = true
	g.notifyReq = req
	return nil
}

func (g *fakePaymentGateway) QueryPayment(context.Context, string) (*application.PayQueryResult, error) {
	return nil, nil
}

func (g *fakePaymentGateway) ClosePayment(context.Context, string) error {
	return nil
}
