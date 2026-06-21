// Package gateway 提供 gRPC 客户端的适配器
// 将外部 gRPC 服务适配为应用层接口
package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	activityv1 "seckill-api/activity/v1"
	commonv1 "seckill-api/common/v1"
	orderv1 "seckill-api/order/v1"
	paymentv1 "seckill-api/payment/v1"
	riskv1 "seckill-api/risk/v1"
	stockv1 "seckill-api/stock/v1"

	"seckill-gateway-service/internal/application"
)

// 确保所有适配器都满足应用接口
var (
	_ application.ActivityGateway = (*ActivityAdapter)(nil)
	_ application.StockGateway    = (*StockAdapter)(nil)
	_ application.RiskGateway     = (*RiskAdapter)(nil)
	_ application.OrderGateway    = (*OrderAdapter)(nil)
	_ application.PaymentGateway  = (*PaymentAdapter)(nil)
)

// ActivityAdapter 封装 gRPC 活动服务客户端
type ActivityAdapter struct {
	client activityv1.ActivityServiceClient
}

// NewActivityAdapter 创建新的活动适配器
func NewActivityAdapter(client activityv1.ActivityServiceClient) *ActivityAdapter {
	return &ActivityAdapter{client: client}
}

// ListActivities 列出所有活动
func (a *ActivityAdapter) ListActivities(ctx context.Context) (application.ActivityList, error) {
	reply, err := a.client.ListActivities(ctx, &emptypb.Empty{})
	if err != nil {
		return application.ActivityList{}, fmt.Errorf("list activities: %w", err)
	}
	items := make([]application.ActivityItem, 0, len(reply.GetActivities()))
	for _, act := range reply.GetActivities() {
		if act == nil {
			continue
		}
		items = append(items, application.ActivityItem{
			ActivityNo:     act.GetActivityNo(),
			ActivityName:   act.GetName(),
			StartTime:      timeFromPB(act.GetStartTime()),
			EndTime:        timeFromPB(act.GetEndTime()),
			ActivityStatus: int(act.GetStatus()),
		})
	}
	return application.ActivityList{Activities: items}, nil
}

// GetActivity 获取单个活动详情
func (a *ActivityAdapter) GetActivity(ctx context.Context, activityNo string) (*application.ActivityDetail, error) {
	reply, err := a.client.GetActivity(ctx, &activityv1.ActivityNoRequest{ActivityNo: activityNo})
	if err != nil {
		return nil, fmt.Errorf("get activity %s: %w", activityNo, err)
	}
	act := reply.GetActivity()
	if act == nil {
		return nil, fmt.Errorf("activity not found")
	}
	return activityDetailFromPB(act), nil
}

// CreateActivity 创建活动
func (a *ActivityAdapter) CreateActivity(ctx context.Context, req application.CreateActivityRequest) (*application.ActivityDetail, error) {
	startTs := timestampFromRFC3339(req.StartTime)
	endTs := timestampFromRFC3339(req.EndTime)
	reply, err := a.client.CreateActivity(ctx, &activityv1.ActivityResponse{
		Activity: &activityv1.Activity{
			Name:          req.ActivityName,
			StartTime:     startTs,
			EndTime:       endTs,
			PurchaseLimit: int64(req.PurchaseLimit),
			Remark:        req.Remark,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create activity: %w", err)
	}
	act := reply.GetActivity()
	if act == nil {
		return nil, fmt.Errorf("create activity: empty response")
	}
	return activityDetailFromPB(act), nil
}

// UpdateActivity 更新活动
func (a *ActivityAdapter) UpdateActivity(ctx context.Context, req application.UpdateActivityRequest) error {
	startTs := timestampFromRFC3339(req.StartTime)
	endTs := timestampFromRFC3339(req.EndTime)
	_, err := a.client.UpdateActivity(ctx, &activityv1.ActivityResponse{
		Activity: &activityv1.Activity{
			ActivityNo:    req.ActivityNo,
			Name:          req.ActivityName,
			StartTime:     startTs,
			EndTime:       endTs,
			PurchaseLimit: int64(req.PurchaseLimit),
			Remark:        req.Remark,
		},
	})
	if err != nil {
		return fmt.Errorf("update activity: %w", err)
	}
	return nil
}

// EndActivity 结束活动
func (a *ActivityAdapter) EndActivity(ctx context.Context, activityNo string) error {
	_, err := a.client.UpdateActivityStatus(ctx, &activityv1.ActivityStatusRequest{
		ActivityNo: activityNo,
		Status:     3, // ActivityEnded
	})
	if err != nil {
		return fmt.Errorf("update activity status: %w", err)
	}
	return nil
}

// AddProduct 添加商品
func (a *ActivityAdapter) AddProduct(ctx context.Context, req application.AddProductRequest) error {
	_, err := a.client.AddActivitySKU(ctx, &activityv1.ActivitySKURequest{
		ActivityNo: req.ActivityNo,
		Sku: &activityv1.SKU{
			ActivityNo:      req.ActivityNo,
			SkuNo:           req.SKUNo,
			ProductName:     req.ProductName,
			ProductImage:    req.ProductImage,
			OriginalPrice:   req.OriginalPrice,
			SeckillPrice:    req.SeckillPrice,
			TotalStock:      req.ActivityStock,
			LimitQuantity:   req.LimitQuantity,
			DiscountType:    int64(req.DiscountType),
			DiscountPrice:   req.DiscountPrice,
			DiscountPercent: req.DiscountPercent,
		},
	})
	if err != nil {
		return fmt.Errorf("add activity sku: %w", err)
	}
	return nil
}

// RemoveProduct 移除商品
func (a *ActivityAdapter) RemoveProduct(ctx context.Context, activityNo, skuNo string) error {
	_, err := a.client.RemoveActivitySKU(ctx, &activityv1.SKURequest{
		ActivityNo: activityNo,
		SkuNo:      skuNo,
	})
	if err != nil {
		return fmt.Errorf("remove activity sku: %w", err)
	}
	return nil
}

// activityDetailFromPB 将 protobuf 活动对象转换为应用层活动详情
func activityDetailFromPB(act *activityv1.Activity) *application.ActivityDetail {
	now := time.Now()
	start := timeFromPB(act.GetStartTime())
	end := timeFromPB(act.GetEndTime())
	startTime, _ := time.Parse(time.RFC3339, start) //nolint:errcheck // zero time on parse failure is safe: activity will not appear open
	endTime, _ := time.Parse(time.RFC3339, end)     //nolint:errcheck // zero time on parse failure is safe: activity will not appear open
	products := make([]application.ProductDetail, 0, len(act.GetSkus()))
	for i, sku := range act.GetSkus() {
		if sku == nil {
			continue
		}
		products = append(products, application.ProductDetail{
			SKUNo:           sku.GetSkuNo(),
			ProductName:     sku.GetProductName(),
			ProductImage:    sku.GetProductImage(),
			OriginalPrice:   sku.GetOriginalPrice(),
			SeckillPrice:    sku.GetSeckillPrice(),
			ActivityStock:   sku.GetTotalStock(),
			DiscountType:    int(sku.GetDiscountType()),
			DiscountPrice:   sku.GetDiscountPrice(),
			DiscountPercent: sku.GetDiscountPercent(),
			SortOrder:       i + 1,
		})
	}
	return &application.ActivityDetail{
		ActivityNo:     act.GetActivityNo(),
		ActivityName:   act.GetName(),
		StartTime:      start,
		EndTime:        end,
		ActivityStatus: int(act.GetStatus()),
		PurchaseLimit:  int(act.GetPurchaseLimit()),
		ActivityOpen:   act.GetStatus() == 1 && !now.Before(startTime) && now.Before(endTime),
		Products:       products,
	}
}

// ========== StockAdapter ==========

// StockAdapter 封装 gRPC 库存服务客户端
type StockAdapter struct {
	client stockv1.StockServiceClient
}

// NewStockAdapter 创建新的库存适配器
func NewStockAdapter(client stockv1.StockServiceClient) *StockAdapter {
	return &StockAdapter{client: client}
}

// Peek 查看库存数量
func (s *StockAdapter) Peek(ctx context.Context, activityNo, skuNo string) (int64, error) {
	reply, err := s.client.Peek(ctx, &activityv1.SKURequest{ActivityNo: activityNo, SkuNo: skuNo})
	if err != nil {
		return 0, fmt.Errorf("peek stock: %w", err)
	}
	return reply.GetStock(), nil
}

// Deduct 扣减库存
func (s *StockAdapter) Deduct(ctx context.Context, req application.DeductRequest) (bool, error) {
	reply, err := s.client.Deduct(ctx, &stockv1.DeductRequest{
		ActivityNo:    req.ActivityNo,
		SkuNo:         req.SkuNo,
		UserId:        req.UserID,
		Quantity:      int64(req.Quantity),
		PurchaseLimit: int64(req.PurchaseLimit),
	})
	if err != nil {
		return false, fmt.Errorf("deduct stock: %w", err)
	}
	return reply.GetOk(), nil
}

// Release 释放库存
func (s *StockAdapter) Release(ctx context.Context, activityNo, skuNo, userID string, quantity int) error {
	_, err := s.client.Release(ctx, &stockv1.ReleaseRequest{
		ActivityNo: activityNo,
		SkuNo:      skuNo,
		UserId:     mustParseInt64(userID),
		Quantity:   int64(quantity),
	})
	if err != nil {
		return fmt.Errorf("release stock: %w", err)
	}
	return nil
}

// ========== RiskAdapter ==========

// RiskAdapter 封装 gRPC 风险服务客户端
type RiskAdapter struct {
	client riskv1.RiskServiceClient
}

// NewRiskAdapter 创建新的风险适配器
func NewRiskAdapter(client riskv1.RiskServiceClient) *RiskAdapter {
	return &RiskAdapter{client: client}
}

// Evaluate 评估用户风险
func (r *RiskAdapter) Evaluate(ctx context.Context, userID int64, requestIP string) (*application.RiskEvaluation, error) {
	reply, err := r.client.Evaluate(ctx, &riskv1.RiskEvaluateRequest{
		UserId:    userID,
		RequestIp: requestIP,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluate risk: %w", err)
	}
	eval := reply.GetEvaluation()
	if eval == nil {
		return &application.RiskEvaluation{}, nil
	}
	return &application.RiskEvaluation{
		Risk:   eval.GetRisk(),
		Level:  int(eval.GetLevel()),
		Reason: eval.GetReason(),
	}, nil
}

// IsRiskUser 检查是否为风险用户
func (r *RiskAdapter) IsRiskUser(ctx context.Context, userID int64) (bool, error) {
	reply, err := r.client.IsRiskUser(ctx, &riskv1.RiskUserRequest{UserId: userID})
	if err != nil {
		return false, fmt.Errorf("check risk user: %w", err)
	}
	return reply.GetOk(), nil
}

// MarkSuspicious 标记可疑用户
func (r *RiskAdapter) MarkSuspicious(ctx context.Context, activity *application.ActivityDetail, userID int64, requestIP string) error {
	_, err := r.client.MarkSuspicious(ctx, &riskv1.RiskMarkRequest{
		Activity:  activityToRiskPB(activity),
		UserId:    userID,
		RequestIp: requestIP,
	})
	if err != nil {
		return fmt.Errorf("mark suspicious: %w", err)
	}
	return nil
}

// RecordAction 记录风险操作
func (r *RiskAdapter) RecordAction(ctx context.Context, record application.RiskRecord) error {
	var createdAt *timestamppb.Timestamp
	if !record.CreatedAt.IsZero() {
		createdAt = timestamppb.New(record.CreatedAt)
	}
	_, err := r.client.RecordAction(ctx, &riskv1.RiskRecordRequest{
		Record: &riskv1.RiskRecord{
			UserId:      record.UserID,
			ActionType:  record.ActionType,
			RiskLevel:   record.RiskLevel,
			RequestIp:   record.RequestIP,
			RequestInfo: record.RequestInfo,
			CreatedAt:   createdAt,
		},
	})
	if err != nil {
		return fmt.Errorf("record risk action: %w", err)
	}
	return nil
}

// activityToRiskPB 将应用层活动转换为 protobuf 风险活动对象
func activityToRiskPB(activity *application.ActivityDetail) *activityv1.Activity {
	if activity == nil {
		return nil
	}
	return &activityv1.Activity{
		ActivityNo:    activity.ActivityNo,
		Name:          activity.ActivityName,
		StartTime:     timestampFromRFC3339(activity.StartTime),
		EndTime:       timestampFromRFC3339(activity.EndTime),
		Status:        int64(activity.ActivityStatus),
		PurchaseLimit: int64(activity.PurchaseLimit),
	}
}

// ========== OrderAdapter ==========

// OrderAdapter 封装 gRPC 订单服务客户端
type OrderAdapter struct {
	client orderv1.OrderServiceClient
}

// NewOrderAdapter 创建新的订单适配器
func NewOrderAdapter(client orderv1.OrderServiceClient) *OrderAdapter {
	return &OrderAdapter{client: client}
}

// GetOrder 获取订单详情
func (o *OrderAdapter) GetOrder(ctx context.Context, orderNo string) (*application.OrderDetail, error) {
	reply, err := o.client.GetOrder(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	return orderDetailFromPB(reply.GetOrder()), nil
}

// ListOrdersByUser 列出用户的订单
func (o *OrderAdapter) ListOrdersByUser(ctx context.Context, userID int64) ([]application.OrderDetail, error) {
	reply, err := o.client.ListOrdersByUser(ctx, &orderv1.OrderListByUserRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("list orders by user: %w", err)
	}
	return orderListFromPB(reply.GetOrders()), nil
}

// ListOrdersByActivity 列出活动的订单
func (o *OrderAdapter) ListOrdersByActivity(ctx context.Context, activityNo string) ([]application.OrderDetail, error) {
	reply, err := o.client.ListOrdersByActivity(ctx, &orderv1.OrderListByActivityRequest{ActivityNo: activityNo})
	if err != nil {
		return nil, fmt.Errorf("list orders by activity: %w", err)
	}
	return orderListFromPB(reply.GetOrders()), nil
}

// CreateOrder 创建订单
func (o *OrderAdapter) CreateOrder(ctx context.Context, req application.CreateOrderRequest) error {
	_, err := o.client.CreateOrder(ctx, &orderv1.OrderResponse{
		Order: &orderv1.Order{
			OrderNo:        req.OrderNo,
			UserId:         req.UserID,
			ActivityNo:     req.ActivityNo,
			SkuNo:          req.SKUNo,
			Quantity:       int64(req.Quantity),
			PayAmount:      req.PayAmount,
			Status:         req.Status,
			TraceId:        req.TraceID,
			RequestTraceId: req.RequestTraceID,
		},
	})
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

// MarkPaid 标记订单为已支付
func (o *OrderAdapter) MarkPaid(ctx context.Context, orderNo, transactionNo string, paidAt time.Time) error {
	_, err := o.client.MarkPaid(ctx, &orderv1.MarkOrderPaidRequest{
		OrderNo:       orderNo,
		TransactionNo: transactionNo,
		PaidAt:        timestamppb.New(paidAt),
	})
	if err != nil {
		return fmt.Errorf("mark order paid: %w", err)
	}
	return nil
}

// CloseOrder 关闭订单
func (o *OrderAdapter) CloseOrder(ctx context.Context, orderNo string) error {
	_, err := o.client.CloseOrder(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
	if err != nil {
		return fmt.Errorf("close order: %w", err)
	}
	return nil
}

// orderDetailFromPB 将 protobuf 订单对象转换为应用层订单详情
func orderDetailFromPB(order *orderv1.Order) *application.OrderDetail {
	if order == nil {
		return nil
	}
	return &application.OrderDetail{
		OrderNo:        order.GetOrderNo(),
		UserID:         order.GetUserId(),
		ActivityNo:     order.GetActivityNo(),
		SKUNo:          order.GetSkuNo(),
		Quantity:       int(order.GetQuantity()),
		PayAmount:      order.GetPayAmount(),
		Status:         order.GetStatus(),
		TraceID:        order.GetTraceId(),
		RequestTraceID: order.GetRequestTraceId(),
		TransactionNo:  order.GetTransactionNo(),
		PaidAt:         timeFromPB(order.GetPaidAt()),
		ClosedAt:       timeFromPB(order.GetClosedAt()),
		CreatedAt:      timeFromPB(order.GetCreatedAt()),
	}
}

// orderListFromPB 将 protobuf 订单列表转换为应用层订单列表
func orderListFromPB(orders []*orderv1.Order) []application.OrderDetail {
	result := make([]application.OrderDetail, 0, len(orders))
	for _, order := range orders {
		if detail := orderDetailFromPB(order); detail != nil {
			result = append(result, *detail)
		}
	}
	return result
}

// ========== PaymentAdapter ==========

// PaymentAdapter 封装 gRPC 支付服务客户端
type PaymentAdapter struct {
	client paymentv1.PaymentServiceClient
}

// NewPaymentAdapter 创建新的支付适配器
func NewPaymentAdapter(client paymentv1.PaymentServiceClient) *PaymentAdapter {
	return &PaymentAdapter{client: client}
}

// Prepay 发起预支付
func (p *PaymentAdapter) Prepay(ctx context.Context, req application.PrepayPaymentRequest) (*application.PayResult, error) {
	reply, err := p.client.Prepay(ctx, &paymentv1.PrepayRequest{
		UserId:     req.UserID,
		OrderNo:    req.OrderNo,
		PayChannel: req.PayChannel,
	})
	if err != nil {
		return nil, fmt.Errorf("prepay: %w", err)
	}
	return payResultFromPB(reply.GetResult()), nil
}

// Notify 通知支付结果
func (p *PaymentAdapter) Notify(ctx context.Context, req application.PaymentNotifyRequest) error {
	_, err := p.client.NotifyPayment(ctx, &paymentv1.PaymentNotifyRequest{
		Channel:       req.Channel,
		OrderNo:       req.OrderNo,
		TransactionNo: req.TransactionNo,
		Params:        req.Params,
	})
	if err != nil {
		return fmt.Errorf("notify payment: %w", err)
	}
	return nil
}

// CreatePayment 创建支付
func (p *PaymentAdapter) CreatePayment(ctx context.Context, req application.CreatePaymentRequest) (*application.PayResult, error) {
	reply, err := p.client.CreatePayment(ctx, &paymentv1.CreatePaymentRequest{
		Request: &paymentv1.CreatePay{
			OrderNo:    req.OrderNo,
			UserId:     req.UserID,
			PayAmount:  req.PayAmount,
			PayChannel: req.PayChannel,
			Subject:    req.Subject,
			ExpireAt:   timestamppb.New(time.Now().Add(application.PaymentExpiry())),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}
	return payResultFromPB(reply.GetResult()), nil
}

// QueryPayment 查询支付状态
func (p *PaymentAdapter) QueryPayment(ctx context.Context, orderNo string) (*application.PayQueryResult, error) {
	reply, err := p.client.QueryPayment(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
	if err != nil {
		return nil, fmt.Errorf("query payment: %w", err)
	}
	result := reply.GetResult()
	if result == nil {
		return &application.PayQueryResult{}, nil
	}
	return &application.PayQueryResult{
		OrderNo:       result.GetOrderNo(),
		PayStatus:     int32(result.GetPayStatus()), //nolint:gosec // G115: safe narrow conversion, value bounded by domain constraints
		TransactionNo: result.GetTransactionNo(),
		PaidAt:        timeFromPB(result.GetPaidAt()),
	}, nil
}

// ClosePayment 关闭支付
func (p *PaymentAdapter) ClosePayment(ctx context.Context, orderNo string) error {
	_, err := p.client.ClosePayment(ctx, &orderv1.OrderNoRequest{OrderNo: orderNo})
	if err != nil {
		return fmt.Errorf("close payment: %w", err)
	}
	return nil
}

// ========== 辅助函数 ==========

// payResultFromPB 将 protobuf 支付结果转换为应用层支付结果
func payResultFromPB(result *paymentv1.PayResult) *application.PayResult {
	if result == nil {
		return &application.PayResult{}
	}
	return &application.PayResult{
		OrderNo:    result.GetOrderNo(),
		PayChannel: result.GetPayChannel(),
		PrepayID:   result.GetPrepayId(),
		NonceStr:   result.GetNonceStr(),
		TimeStamp:  result.GetTimeStamp(),
		Sign:       result.GetSign(),
	}
}

// timeFromPB 将 protobuf 时间戳转换为 RFC3339 字符串
func timeFromPB(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.AsTime().Format(time.RFC3339)
}

// timestampFromRFC3339 将 RFC3339 字符串转换为 protobuf 时间戳
func timestampFromRFC3339(value string) *timestamppb.Timestamp {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return nil
	}
	return timestamppb.New(parsed)
}

// mustParseInt64 解析 int64 字符串
func mustParseInt64(s string) int64 {
	var v int64
	fmt.Sscanf(strings.TrimSpace(s), "%d", &v) //nolint:errcheck // input validated upstream
	return v
}

// 确保引用 commonv1.BoolResponse 以便编译
var _ = (*commonv1.BoolResponse)(nil)
