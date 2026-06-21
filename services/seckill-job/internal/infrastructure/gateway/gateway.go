// Package gateway 提供各微服务的RPC客户端网关，封装远程调用并集成熔断器保护
package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	activityv1 "seckill-api/activity/v1"
	orderv1 "seckill-api/order/v1"
	ordersyncv1 "seckill-api/order_sync/v1"
	paymentv1 "seckill-api/payment/v1"
	riskv1 "seckill-api/risk/v1"
	stockv1 "seckill-api/stock/v1"

	"seckill-job-service/internal/domain/entity"
)

// -- 活动服务网关 --

// ActivityClient 活动服务网关客户端，内含熔断器保护
type ActivityClient struct {
	client activityv1.ActivityServiceClient // RPC客户端
	logger *slog.Logger                     // 日志记录器
	cb     *CircuitBreaker                  // 熔断器
}

// NewActivityClient 创建带熔断器的活动服务客户端
func NewActivityClient(client activityv1.ActivityServiceClient, logger *slog.Logger, cb *CircuitBreaker) *ActivityClient {
	return &ActivityClient{client: client, logger: logger, cb: cb}
}

// ListActivities 查询所有活动，受熔断器保护
func (c *ActivityClient) ListActivities(ctx context.Context) ([]entity.Activity, error) {
	var activities []entity.Activity
	err := c.cb.Execute(func() error {
		var execErr error
		activities, execErr = c.listActivities(ctx)
		return execErr
	})
	return activities, err
}

// listActivities 实际执行活动列表查询（内部方法）
func (c *ActivityClient) listActivities(ctx context.Context) ([]entity.Activity, error) {
	resp, err := c.client.ListActivities(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	activities := make([]entity.Activity, 0, len(resp.Activities))
	for _, a := range resp.Activities {
		activities = append(activities, toActivity(a))
	}
	return activities, nil
}

// UpdateActivityStatus 更新活动状态，受熔断器保护
func (c *ActivityClient) UpdateActivityStatus(ctx context.Context, activityNo string, status int) error {
	return c.cb.Execute(func() error {
		_, err := c.client.UpdateActivityStatus(ctx, &activityv1.ActivityStatusRequest{
			ActivityNo: activityNo,
			Status:     int64(status),
		})
		if err != nil {
			return fmt.Errorf("update activity status %s: %w", activityNo, err)
		}
		return nil
	})
}

// toActivity 将protobuf活动对象转换为领域实体
func toActivity(a *activityv1.Activity) entity.Activity {
	result := entity.Activity{
		ActivityNo:    a.GetActivityNo(),
		Name:          a.GetName(),
		Status:        int(a.GetStatus()),
		PurchaseLimit: int(a.GetPurchaseLimit()),
		Remark:        a.GetRemark(),
	}
	if a.StartTime != nil {
		result.StartTime = a.StartTime.AsTime()
	}
	if a.EndTime != nil {
		result.EndTime = a.EndTime.AsTime()
	}
	if a.CreatedAt != nil {
		result.CreatedAt = a.CreatedAt.AsTime()
	}
	if a.UpdatedAt != nil {
		result.UpdatedAt = a.UpdatedAt.AsTime()
	}
	for _, sku := range a.GetSkus() {
		result.SKUs = append(result.SKUs, entity.SKU{
			ActivityNo: sku.GetActivityNo(),
			SKUNo:      sku.GetSkuNo(),
			TotalStock: sku.GetTotalStock(),
		})
	}
	return result
}

// -- 订单服务网关 --

// OrderClient 订单服务网关客户端，内含熔断器保护
type OrderClient struct {
	client orderv1.OrderServiceClient // RPC客户端
	logger *slog.Logger               // 日志记录器
	cb     *CircuitBreaker            // 熔断器
}

// NewOrderClient 创建带熔断器的订单服务客户端
func NewOrderClient(client orderv1.OrderServiceClient, logger *slog.Logger, cb *CircuitBreaker) *OrderClient {
	return &OrderClient{client: client, logger: logger, cb: cb}
}

// ListOrdersByActivities 按活动编号查询订单，受熔断器保护
func (c *OrderClient) ListOrdersByActivities(ctx context.Context, activityNos []string) (map[string][]entity.Order, error) {
	var result map[string][]entity.Order
	err := c.cb.Execute(func() error {
		var execErr error
		result, execErr = c.listOrdersByActivities(ctx, activityNos)
		return execErr
	})
	return result, err
}

// listOrdersByActivities 实际执行订单查询（内部方法）
func (c *OrderClient) listOrdersByActivities(ctx context.Context, activityNos []string) (map[string][]entity.Order, error) {
	resp, err := c.client.ListOrdersByActivities(ctx, &orderv1.OrderListByActivitiesRequest{
		ActivityNos: activityNos,
	})
	if err != nil {
		return nil, fmt.Errorf("list orders by activities: %w", err)
	}
	result := make(map[string][]entity.Order, len(resp.Items))
	for _, item := range resp.Items {
		result[item.ActivityNo] = toOrders(item.Orders)
	}
	return result, nil
}

// CloseOrder 关闭订单，受熔断器保护
func (c *OrderClient) CloseOrder(ctx context.Context, orderNo string) error {
	return c.cb.Execute(func() error {
		_, err := c.client.CloseOrder(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
		if err != nil {
			return fmt.Errorf("close order %s: %w", orderNo, err)
		}
		return nil
	})
}

// MarkOrderPaid 标记订单已支付，受熔断器保护
func (c *OrderClient) MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error {
	return c.cb.Execute(func() error {
		_, err := c.client.MarkPaid(ctx, &orderv1.MarkOrderPaidRequest{
			OrderNo:       orderNo,
			TransactionNo: transactionNo,
			PaidAt:        timestamppb.New(paidAt),
		})
		if err != nil {
			return fmt.Errorf("mark order paid %s: %w", orderNo, err)
		}
		return nil
	})
}

// toOrders 将protobuf订单列表转换为领域实体列表
func toOrders(orders []*orderv1.Order) []entity.Order {
	result := make([]entity.Order, 0, len(orders))
	for _, o := range orders {
		result = append(result, toOrder(o))
	}
	return result
}

// toOrder 将protobuf订单对象转换为领域实体
func toOrder(o *orderv1.Order) entity.Order {
	result := entity.Order{
		OrderNo:       o.GetOrderNo(),
		UserID:        o.GetUserId(),
		ActivityNo:    o.GetActivityNo(),
		SKUNo:         o.GetSkuNo(),
		Quantity:      int(o.GetQuantity()),
		PayAmount:     o.GetPayAmount(),
		Status:        o.GetStatus(),
		TransactionNo: o.GetTransactionNo(),
	}
	if o.PaidAt != nil {
		t := o.PaidAt.AsTime()
		result.PaidAt = &t
	}
	if o.CreatedAt != nil {
		result.CreatedAt = o.CreatedAt.AsTime()
	}
	return result
}

// -- 库存服务网关 --

// StockClient 库存服务网关客户端，内含熔断器保护
type StockClient struct {
	client stockv1.StockServiceClient // RPC客户端
	logger *slog.Logger               // 日志记录器
	cb     *CircuitBreaker            // 熔断器
}

// NewStockClient 创建带熔断器的库存服务客户端
func NewStockClient(client stockv1.StockServiceClient, logger *slog.Logger, cb *CircuitBreaker) *StockClient {
	return &StockClient{client: client, logger: logger, cb: cb}
}

// ReleaseStock 释放库存，受熔断器保护
func (c *StockClient) ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int, orderNo string) error {
	return c.cb.Execute(func() error {
		_, err := c.client.Release(ctx, &stockv1.ReleaseRequest{
			ActivityNo: activityNo,
			SkuNo:      skuNo,
			UserId:     userID,
			Quantity:   int64(quantity),
			OrderNo:    orderNo,
		})
		if err != nil {
			return fmt.Errorf("release stock %s/%s: %w", activityNo, skuNo, err)
		}
		return nil
	})
}

// CleanupActivityStock 清理活动库存缓存，受熔断器保护
func (c *StockClient) CleanupActivityStock(ctx context.Context, activityNo string, skuNos []string) (int, error) {
	var deleted int
	err := c.cb.Execute(func() error {
		resp, execErr := c.client.CleanupActivity(ctx, &stockv1.StockCleanupRequest{
			ActivityNo: activityNo,
			SkuNos:     skuNos,
		})
		if execErr != nil {
			return fmt.Errorf("cleanup activity stock %s: %w", activityNo, execErr)
		}
		deleted = int(resp.GetDeleted())
		return nil
	})
	return deleted, err
}

// CleanupActivityPurchases 清理活动购买记录，受熔断器保护
func (c *StockClient) CleanupActivityPurchases(ctx context.Context, activityNo string) (int, error) {
	var deleted int
	err := c.cb.Execute(func() error {
		resp, execErr := c.client.CleanupActivityPurchases(ctx, &stockv1.ActivityPurchaseCleanupRequest{
			ActivityNo: activityNo,
		})
		if execErr != nil {
			return fmt.Errorf("cleanup activity purchases %s: %w", activityNo, execErr)
		}
		deleted = int(resp.GetDeleted())
		return nil
	})
	return deleted, err
}

// -- 支付服务网关 --

// PaymentClient 支付服务网关客户端，内含熔断器保护
type PaymentClient struct {
	client paymentv1.PaymentServiceClient // RPC客户端
	logger *slog.Logger                   // 日志记录器
	cb     *CircuitBreaker                // 熔断器
}

// NewPaymentClient 创建带熔断器的支付服务客户端
func NewPaymentClient(client paymentv1.PaymentServiceClient, logger *slog.Logger, cb *CircuitBreaker) *PaymentClient {
	return &PaymentClient{client: client, logger: logger, cb: cb}
}

// QueryPayments 查询支付结果，受熔断器保护
func (c *PaymentClient) QueryPayments(ctx context.Context, orderNos []string) (map[string]entity.PayQueryResult, error) {
	var result map[string]entity.PayQueryResult
	err := c.cb.Execute(func() error {
		resp, execErr := c.client.QueryPayments(ctx, &orderv1.OrderNosRequest{OrderNos: orderNos})
		if execErr != nil {
			return fmt.Errorf("query payments: %w", execErr)
		}
		result = make(map[string]entity.PayQueryResult, len(resp.Results))
		for _, r := range resp.GetResults() {
			result[r.GetOrderNo()] = toPayQueryResult(r)
		}
		return nil
	})
	return result, err
}

// ClosePayment 关闭支付，受熔断器保护
func (c *PaymentClient) ClosePayment(ctx context.Context, orderNo string) error {
	return c.cb.Execute(func() error {
		_, err := c.client.ClosePayment(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
		if err != nil {
			return fmt.Errorf("close payment %s: %w", orderNo, err)
		}
		return nil
	})
}

// toPayQueryResult 将protobuf支付查询结果转换为领域实体
func toPayQueryResult(r *paymentv1.PayQueryResult) entity.PayQueryResult {
	result := entity.PayQueryResult{
		OrderNo:       r.GetOrderNo(),
		PayStatus:     int(r.GetPayStatus()),
		TransactionNo: r.GetTransactionNo(),
	}
	if r.PaidAt != nil {
		t := r.PaidAt.AsTime()
		result.PaidAt = &t
	}
	return result
}

// -- 订单同步服务网关 --

// OrderSyncClient 订单同步服务网关客户端，内含熔断器保护
type OrderSyncClient struct {
	client ordersyncv1.OrderSyncServiceClient // RPC客户端
	logger *slog.Logger                       // 日志记录器
	cb     *CircuitBreaker                    // 熔断器
}

// NewOrderSyncClient 创建带熔断器的订单同步服务客户端
func NewOrderSyncClient(client ordersyncv1.OrderSyncServiceClient, logger *slog.Logger, cb *CircuitBreaker) *OrderSyncClient {
	return &OrderSyncClient{client: client, logger: logger, cb: cb}
}

// SyncOrder 同步订单，受熔断器保护
func (c *OrderSyncClient) SyncOrder(ctx context.Context, request entity.SyncOrderRequest) error {
	return c.cb.Execute(func() error {
		_, err := c.client.SyncOrder(ctx, &ordersyncv1.SyncOrderRequest{
			Request: &ordersyncv1.SyncOrder{
				OrderNo:        request.OrderNo,
				UserId:         request.UserID,
				OrderSource:    request.OrderSource,
				TotalAmount:    request.TotalAmount,
				DiscountAmount: request.DiscountAmount,
				PayAmount:      request.PayAmount,
				PaidAt:         timestamppb.New(request.PaidAt),
				TransactionNo:  request.TransactionNo,
			},
		})
		if err != nil {
			return fmt.Errorf("sync order %s: %w", request.OrderNo, err)
		}
		return nil
	})
}

// ListSyncedOrdersByOrderNos 按订单号查询已同步订单，受熔断器保护
func (c *OrderSyncClient) ListSyncedOrdersByOrderNos(ctx context.Context, orderNos []string) (map[string]entity.SyncedOrder, error) {
	var result map[string]entity.SyncedOrder
	err := c.cb.Execute(func() error {
		resp, execErr := c.client.ListSyncedOrdersByOrderNos(ctx, &orderv1.OrderNosRequest{OrderNos: orderNos})
		if execErr != nil {
			return fmt.Errorf("list synced orders by order nos: %w", execErr)
		}
		result = make(map[string]entity.SyncedOrder, len(resp.GetOrders()))
		for _, o := range resp.GetOrders() {
			result[o.GetOrderNo()] = entity.SyncedOrder{OrderNo: o.GetOrderNo()}
		}
		return nil
	})
	return result, err
}

// -- 风控服务网关 --

// RiskClient 风控服务网关客户端，内含熔断器保护
type RiskClient struct {
	client riskv1.RiskServiceClient // RPC客户端
	logger *slog.Logger             // 日志记录器
	cb     *CircuitBreaker          // 熔断器
}

// NewRiskClient 创建带熔断器的风控服务客户端
func NewRiskClient(client riskv1.RiskServiceClient, logger *slog.Logger, cb *CircuitBreaker) *RiskClient {
	return &RiskClient{client: client, logger: logger, cb: cb}
}

// CleanupExpiredRiskUsers 清理过期风控用户，受熔断器保护
func (c *RiskClient) CleanupExpiredRiskUsers(ctx context.Context) (int, error) {
	var deleted int
	err := c.cb.Execute(func() error {
		resp, execErr := c.client.CleanupExpiredRiskUsers(ctx, &emptypb.Empty{})
		if execErr != nil {
			return fmt.Errorf("cleanup expired risk users: %w", execErr)
		}
		deleted = int(resp.GetDeleted())
		return nil
	})
	return deleted, err
}
