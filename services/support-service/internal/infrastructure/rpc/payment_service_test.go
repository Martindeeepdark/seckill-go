package rpc

import (
	"context"
	"testing"
	"time"

	paymentv1 "seckill-api/payment/v1"
	supportapp "seckill-support-service/internal/application"
	"seckill-support-service/internal/domain"
	paymentdomain "seckill-support-service/internal/domain/entity"
	supportstatus "seckill-support-service/internal/domain/status"
)

func TestPaymentPBServicePrepayUsesSupportApp(t *testing.T) {
	orders := &fakeRPCOrderGateway{order: paymentdomain.Order{OrderNo: "O1", UserID: 7, PayAmount: 9900, Status: string(supportstatus.OrderPending)}}
	payments := &fakeRPCPaymentGateway{}
	app := supportapp.NewApp(orders, payments, nil, nil, nil)
	service := NewPaymentPBService(payments, app)

	reply, err := service.Prepay(context.Background(), &paymentv1.PrepayRequest{
		UserId:     7,
		OrderNo:    "O1",
		PayChannel: "mock",
	})
	if err != nil {
		t.Fatalf("Prepay: %v", err)
	}
	if reply.GetResult().GetPrepayId() != "prepay-O1" {
		t.Fatalf("prepay id = %q, want prepay-O1", reply.GetResult().GetPrepayId())
	}
}

func TestPaymentPBServiceNotifyMarksOrderPaid(t *testing.T) {
	orders := &fakeRPCOrderGateway{order: paymentdomain.Order{OrderNo: "O1", UserID: 7, PayAmount: 9900, Status: string(supportstatus.OrderPending)}}
	payments := &fakeRPCPaymentGateway{}
	app := supportapp.NewApp(orders, payments, nil, nil, nil)
	service := NewPaymentPBService(payments, app)

	_, err := service.NotifyPayment(context.Background(), &paymentv1.PaymentNotifyRequest{
		Channel:       "mock",
		OrderNo:       "O1",
		TransactionNo: "T1",
	})
	if err != nil {
		t.Fatalf("NotifyPayment: %v", err)
	}
	if orders.markPaidOrderNo != "O1" || orders.markPaidTransactionNo != "T1" {
		t.Fatalf("mark paid args = %s/%s, want O1/T1", orders.markPaidOrderNo, orders.markPaidTransactionNo)
	}
}

type fakeRPCOrderGateway struct {
	order                 paymentdomain.Order
	markPaidOrderNo       string
	markPaidTransactionNo string
}

func (g *fakeRPCOrderGateway) GetOrder(_ context.Context, orderNo string) (paymentdomain.Order, error) {
	if orderNo != g.order.OrderNo {
		return paymentdomain.Order{}, domain.ErrOrderNotFound
	}
	return g.order, nil
}

func (g *fakeRPCOrderGateway) MarkOrderPaid(_ context.Context, orderNo string, transactionNo string, paidAt time.Time) error {
	g.markPaidOrderNo = orderNo
	g.markPaidTransactionNo = transactionNo
	g.order.Status = string(supportstatus.OrderPaid)
	g.order.TransactionNo = transactionNo
	g.order.PaidAt = &paidAt
	return nil
}

type fakeRPCPaymentGateway struct{}

func (g *fakeRPCPaymentGateway) CreatePayment(_ context.Context, request paymentdomain.CreatePayRequest) (paymentdomain.PayResult, error) {
	return paymentdomain.PayResult{
		OrderNo:    request.OrderNo,
		PayChannel: request.PayChannel,
		PrepayID:   "prepay-" + request.OrderNo,
	}, nil
}

func (g *fakeRPCPaymentGateway) QueryPayment(_ context.Context, orderNo string) (paymentdomain.PayQueryResult, error) {
	return paymentdomain.PayQueryResult{OrderNo: orderNo}, nil
}

func (g *fakeRPCPaymentGateway) QueryPayments(_ context.Context, orderNos []string) (map[string]paymentdomain.PayQueryResult, error) {
	results := make(map[string]paymentdomain.PayQueryResult, len(orderNos))
	for _, orderNo := range orderNos {
		results[orderNo] = paymentdomain.PayQueryResult{OrderNo: orderNo}
	}
	return results, nil
}

func (g *fakeRPCPaymentGateway) ClosePayment(context.Context, string) error {
	return nil
}
