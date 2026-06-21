// Package gateway 提供外部服务的网关实现
package gateway

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	orderv1 "seckill-api/order/v1"
	"seckill-support-service/internal/domain"
	"seckill-support-service/internal/domain/entity"
)

// OrderGateway 订单服务网关
type OrderGateway struct {
	client orderv1.OrderServiceClient // gRPC客户端
	cb     *CircuitBreaker            // 熔断器
}

// NewOrderGateway 创建订单网关
func NewOrderGateway(client orderv1.OrderServiceClient, cb *CircuitBreaker) *OrderGateway {
	return &OrderGateway{client: client, cb: cb}
}

// GetOrder 获取订单信息
func (g *OrderGateway) GetOrder(ctx context.Context, orderNo string) (entity.Order, error) {
	var order entity.Order
	err := g.cb.Execute(func() error {
		var err error
		order, err = g.getOrder(ctx, orderNo)
		return err
	})
	return order, err
}

// getOrder 实际获取订单的方法
func (g *OrderGateway) getOrder(ctx context.Context, orderNo string) (entity.Order, error) {
	reply, err := g.client.GetOrder(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
	if err != nil {
		return entity.Order{}, fmt.Errorf("get order via grpc: %w", err)
	}
	order := reply.GetOrder()
	if order == nil {
		return entity.Order{}, domain.ErrOrderNotFound
	}
	result := entity.Order{
		OrderNo:       order.GetOrderNo(),
		UserID:        order.GetUserId(),
		PayAmount:     order.GetPayAmount(),
		Status:        order.GetStatus(),
		TransactionNo: order.GetTransactionNo(),
	}
	if order.GetPaidAt() != nil {
		paidAt := order.GetPaidAt().AsTime()
		result.PaidAt = &paidAt
	}
	return result, nil
}

// MarkOrderPaid 标记订单为已支付
func (g *OrderGateway) MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error {
	return g.cb.Execute(func() error {
		_, err := g.client.MarkPaid(ctx, &orderv1.MarkOrderPaidRequest{
			OrderNo:       orderNo,
			TransactionNo: transactionNo,
			PaidAt:        timestamppb.New(paidAt),
		})
		if err != nil {
			return fmt.Errorf("mark order paid via grpc: %w", err)
		}
		return nil
	})
}
