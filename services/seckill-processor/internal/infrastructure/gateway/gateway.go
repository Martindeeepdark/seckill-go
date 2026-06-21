// Package gateway 提供 gRPC 客户端的实现
// 封装对各个微服务的调用，包括活动、库存、风控、订单、支付等
package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	activityv1 "seckill-api/activity/v1"
	freecardv1 "seckill-api/free_card/v1"
	orderv1 "seckill-api/order/v1"
	ordersyncv1 "seckill-api/order_sync/v1"
	paymentv1 "seckill-api/payment/v1"
	riskv1 "seckill-api/risk/v1"
	stockv1 "seckill-api/stock/v1"

	domainservice "seckill-processor-service/internal/domain/service"
	"seckill-processor-service/internal/domain/model"
)

// ==================== 活动网关 ====================

// ActivityClient 活动服务客户端
type ActivityClient struct {
	client activityv1.ActivityServiceClient
	logger *slog.Logger
}

// NewActivityClient 创建活动服务客户端
func NewActivityClient(client activityv1.ActivityServiceClient, logger *slog.Logger) *ActivityClient {
	return &ActivityClient{client: client, logger: logger}
}

// GetActivity 获取活动信息
func (c *ActivityClient) GetActivity(ctx context.Context, activityNo string) (model.ActivityInfo, error) {
	resp, err := c.client.GetActivity(ctx, &activityv1.ActivityNoRequest{ActivityNo: activityNo})
	if err != nil {
		return model.ActivityInfo{}, fmt.Errorf("get activity %s: %w", activityNo, err)
	}
	return toActivity(resp.GetActivity()), nil
}

// GetSKU 获取 SKU 信息
func (c *ActivityClient) GetSKU(ctx context.Context, activityNo, skuNo string) (model.SKUInfo, error) {
	resp, err := c.client.GetSKU(ctx, &activityv1.SKURequest{ActivityNo: activityNo, SkuNo: skuNo})
	if err != nil {
		return model.SKUInfo{}, fmt.Errorf("get sku %s/%s: %w", activityNo, skuNo, err)
	}
	return toSKU(resp.GetSku()), nil
}

// toActivity 将 protobuf 消息转换为应用层活动信息
func toActivity(a *activityv1.Activity) model.ActivityInfo {
	result := model.ActivityInfo{
		ActivityNo:    a.GetActivityNo(),
		Name:          a.GetName(),
		Status:        a.GetStatus(),
		PurchaseLimit: a.GetPurchaseLimit(),
	}
	if a.StartTime != nil {
		result.StartTime = a.StartTime.AsTime()
	}
	if a.EndTime != nil {
		result.EndTime = a.EndTime.AsTime()
	}
	return result
}

// toSKU 将 protobuf 消息转换为应用层 SKU 信息
func toSKU(s *activityv1.SKU) model.SKUInfo {
	return model.SKUInfo{
		ActivityNo:    s.GetActivityNo(),
		SKUNo:         s.GetSkuNo(),
		TotalStock:    s.GetTotalStock(),
		SeckillPrice:  s.GetSeckillPrice(),
		LimitQuantity: s.GetLimitQuantity(),
	}
}

// ==================== 库存网关 ====================

// StockClient 库存服务客户端
type StockClient struct {
	client stockv1.StockServiceClient
	logger *slog.Logger
}

// NewStockClient 创建库存服务客户端
func NewStockClient(client stockv1.StockServiceClient, logger *slog.Logger) *StockClient {
	return &StockClient{client: client, logger: logger}
}

// DeductStockWithLimit 扣减库存（带限购检查）
func (c *StockClient) DeductStockWithLimit(ctx context.Context, activityNo, skuNo string, userID int64, quantity, purchaseLimit int64, orderNo string) (bool, error) {
	resp, err := c.client.Deduct(ctx, &stockv1.DeductRequest{
		ActivityNo:    activityNo,
		SkuNo:         skuNo,
		UserId:        userID,
		Quantity:      quantity,
		PurchaseLimit: purchaseLimit,
		OrderNo:       orderNo,
	})
	if err != nil {
		return false, fmt.Errorf("deduct stock %s/%s: %w", activityNo, skuNo, err)
	}
	return resp.GetOk(), nil
}

// ReleaseStock 释放库存
func (c *StockClient) ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error {
	_, err := c.client.Release(ctx, &stockv1.ReleaseRequest{
		ActivityNo: activityNo,
		SkuNo:      skuNo,
		UserId:     userID,
		Quantity:   quantity,
		OrderNo:    orderNo,
	})
	if err != nil {
		return fmt.Errorf("release stock %s/%s: %w", activityNo, skuNo, err)
	}
	return nil
}

// ==================== 风控网关 ====================

// RiskClient 风控服务客户端
type RiskClient struct {
	client riskv1.RiskServiceClient
	logger *slog.Logger
}

// NewRiskClient 创建风控服务客户端
func NewRiskClient(client riskv1.RiskServiceClient, logger *slog.Logger) *RiskClient {
	return &RiskClient{client: client, logger: logger}
}

// Evaluate 评估用户风险
func (c *RiskClient) Evaluate(ctx context.Context, userID int64, requestIP string) (model.RiskResult, error) {
	resp, err := c.client.Evaluate(ctx, &riskv1.RiskEvaluateRequest{
		UserId:    userID,
		RequestIp: requestIP,
	})
	if err != nil {
		return model.RiskResult{}, fmt.Errorf("risk evaluate user %d: %w", userID, err)
	}
	eval := resp.GetEvaluation()
	return model.RiskResult{
		Risk:   eval.GetRisk(),
		Level:  eval.GetLevel(),
		Reason: eval.GetReason(),
	}, nil
}

// ==================== 订单网关 ====================

// OrderClient 订单服务客户端
type OrderClient struct {
	client orderv1.OrderServiceClient
	logger *slog.Logger
}

// NewOrderClient 创建订单服务客户端
func NewOrderClient(client orderv1.OrderServiceClient, logger *slog.Logger) *OrderClient {
	return &OrderClient{client: client, logger: logger}
}

// CreateOrder 创建订单
// order-service 在 (user_id, trace_id) 命中 UNIQUE INDEX 时返回 gRPC AlreadyExists
// (源自 persistence.ErrDuplicate,底层为 pgconn 23505)。
// 这里转换为领域 ErrDuplicateTraceID,供领域服务 Submit 在 DuplicateKey 路径上回查
func (c *OrderClient) CreateOrder(ctx context.Context, order model.OrderRequest) error {
	_, err := c.client.CreateOrder(ctx, &orderv1.OrderResponse{
		Order: &orderv1.Order{
			OrderNo:        order.OrderNo,
			UserId:         order.UserID,
			ActivityNo:     order.ActivityNo,
			SkuNo:          order.SKUNo,
			Quantity:       order.Quantity,
			PayAmount:      order.PayAmount,
			Status:         order.Status,
			TraceId:        order.TraceID,
			RequestTraceId: order.RequestTraceID,
		},
	})
	if err != nil {
		if isGRPCAlreadyExists(err) {
			return domainservice.ErrDuplicateTraceID
		}
		return fmt.Errorf("create order %s: %w", order.OrderNo, err)
	}
	return nil
}

// GetByUserAndTrace 根据 (userID, traceID) 双键查询订单
// 用于 DuplicateKey (23505) 回查场景。
// 底层调用 order-service 的 GetOrderByUserAndTrace RPC(双参数,契合
// sk_order partial UNIQUE INDEX uk_sk_order_user_trace 列序与 HASH(user_id) 分区)。
// gRPC NotFound → 领域 ErrOrderNotFound
func (c *OrderClient) GetByUserAndTrace(ctx context.Context, userID int64, traceID string) (model.OrderInfo, error) {
	resp, err := c.client.GetOrderByUserAndTrace(ctx, &orderv1.GetOrderByUserAndTraceRequest{
		UserId:  userID,
		TraceId: traceID,
	})
	if err != nil {
		if isGRPCNotFound(err) {
			return model.OrderInfo{}, domainservice.ErrOrderNotFound
		}
		return model.OrderInfo{}, fmt.Errorf("get order by user %d trace_id %s: %w", userID, traceID, err)
	}
	return toOrder(resp.GetOrder()), nil
}

// isGRPCAlreadyExists 判断 gRPC 错误是否为 AlreadyExists
func isGRPCAlreadyExists(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.AlreadyExists
}

// isGRPCNotFound 判断 gRPC 错误是否为 NotFound
func isGRPCNotFound(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.NotFound
}

// GetOrder 获取订单信息
func (c *OrderClient) GetOrder(ctx context.Context, orderNo string) (model.OrderInfo, error) {
	resp, err := c.client.GetOrder(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
	if err != nil {
		return model.OrderInfo{}, fmt.Errorf("get order %s: %w", orderNo, err)
	}
	return toOrder(resp.GetOrder()), nil
}

// MarkOrderPaid 标记订单为已支付
func (c *OrderClient) MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error {
	_, err := c.client.MarkPaid(ctx, &orderv1.MarkOrderPaidRequest{
		OrderNo:       orderNo,
		TransactionNo: transactionNo,
		PaidAt:        timestamppb.New(paidAt),
	})
	if err != nil {
		return fmt.Errorf("mark order paid %s: %w", orderNo, err)
	}
	return nil
}

// CloseOrder 关闭订单
func (c *OrderClient) CloseOrder(ctx context.Context, orderNo string) error {
	_, err := c.client.CloseOrder(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
	if err != nil {
		return fmt.Errorf("close order %s: %w", orderNo, err)
	}
	return nil
}

// toOrder 将 protobuf 消息转换为应用层订单信息
func toOrder(o *orderv1.Order) model.OrderInfo {
	result := model.OrderInfo{
		OrderNo:       o.GetOrderNo(),
		UserID:        o.GetUserId(),
		ActivityNo:    o.GetActivityNo(),
		SKUNo:         o.GetSkuNo(),
		Quantity:      o.GetQuantity(),
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

// ==================== 支付网关 ====================

// PaymentClient 支付服务客户端
type PaymentClient struct {
	client paymentv1.PaymentServiceClient
	logger *slog.Logger
}

// NewPaymentClient 创建支付服务客户端
func NewPaymentClient(client paymentv1.PaymentServiceClient, logger *slog.Logger) *PaymentClient {
	return &PaymentClient{client: client, logger: logger}
}

// QueryPayment 查询支付状态
func (c *PaymentClient) QueryPayment(ctx context.Context, orderNo string) (model.PayQueryResult, error) {
	resp, err := c.client.QueryPayment(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
	if err != nil {
		return model.PayQueryResult{}, fmt.Errorf("query payment %s: %w", orderNo, err)
	}
	return toPayQueryResult(resp.GetResult()), nil
}

// ClosePayment 关闭支付
func (c *PaymentClient) ClosePayment(ctx context.Context, orderNo string) error {
	_, err := c.client.ClosePayment(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
	if err != nil {
		return fmt.Errorf("close payment %s: %w", orderNo, err)
	}
	return nil
}

// toPayQueryResult 将 protobuf 消息转换为应用层支付查询结果
func toPayQueryResult(r *paymentv1.PayQueryResult) model.PayQueryResult {
	result := model.PayQueryResult{
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

// ==================== 自由卡网关 ====================

// FreeCardClient 自由卡服务客户端
type FreeCardClient struct {
	client freecardv1.FreeCardServiceClient
	logger *slog.Logger
}

// NewFreeCardClient 创建自由卡服务客户端
func NewFreeCardClient(client freecardv1.FreeCardServiceClient, logger *slog.Logger) *FreeCardClient {
	return &FreeCardClient{client: client, logger: logger}
}

// IssueCard 发放自由卡
func (c *FreeCardClient) IssueCard(ctx context.Context, payload model.IssueCardPayload) (string, error) {
	resp, err := c.client.IssueCard(ctx, &freecardv1.IssueCardRequest{
		Request: &freecardv1.IssueCard{
			UserId:    payload.UserID,
			OrderNo:   payload.OrderNo,
			CardName:  payload.CardName,
			FaceValue: payload.FaceValue,
			ValidDays: payload.ValidDays,
		},
	})
	if err != nil {
		return "", fmt.Errorf("issue card for order %s: %w", payload.OrderNo, err)
	}
	return resp.GetCardNo(), nil
}

// ==================== 订单同步网关 ====================

// OrderSyncClient 订单同步服务客户端
type OrderSyncClient struct {
	client ordersyncv1.OrderSyncServiceClient
	logger *slog.Logger
}

// NewOrderSyncClient 创建订单同步服务客户端
func NewOrderSyncClient(client ordersyncv1.OrderSyncServiceClient, logger *slog.Logger) *OrderSyncClient {
	return &OrderSyncClient{client: client, logger: logger}
}

// SyncOrder 同步订单信息
func (c *OrderSyncClient) SyncOrder(ctx context.Context, payload model.SyncOrderPayload) error {
	_, err := c.client.SyncOrder(ctx, &ordersyncv1.SyncOrderRequest{
		Request: &ordersyncv1.SyncOrder{
			OrderNo:        payload.OrderNo,
			UserId:         payload.UserID,
			OrderSource:    payload.OrderSource,
			TotalAmount:    payload.TotalAmount,
			DiscountAmount: payload.DiscountAmount,
			PayAmount:      payload.PayAmount,
			PaidAt:         timestamppb.New(payload.PaidAt),
			TransactionNo:  payload.TransactionNo,
		},
	})
	if err != nil {
		return fmt.Errorf("sync order %s: %w", payload.OrderNo, err)
	}
	return nil
}

// ==================== 辅助类型 ====================

// 确保 noopCloser 满足 kratos Discovery 接口的编译时检查
var _ interface{ Close() error } = (*noopCloser)(nil)

// noopCloser 空操作的关闭器
type noopCloser struct{}

// Close 空操作关闭方法
func (noopCloser) Close() error { return nil }

// 确保查询响应类型匹配
var _ = (*emptypb.Empty)(nil)
