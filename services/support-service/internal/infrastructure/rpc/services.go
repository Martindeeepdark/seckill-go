// Package rpc 提供gRPC服务实现
package rpc

import (
	"context"

	commonv1 "seckill-api/common/v1"
	free_cardv1 "seckill-api/free_card/v1"
	memberv1 "seckill-api/member/v1"
	orderv1 "seckill-api/order/v1"
	order_syncv1 "seckill-api/order_sync/v1"
	paymentv1 "seckill-api/payment/v1"

	supportapp "seckill-support-service/internal/application"
	paymentdomain "seckill-support-service/internal/domain/entity"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// PaymentServiceGateway 支付服务网关接口
type PaymentServiceGateway interface {
	supportapp.PaymentGateway
	QueryPayments(ctx context.Context, orderNos []string) (map[string]paymentdomain.PayQueryResult, error)
}

// PaymentPBService 支付服务protobuf实现
type PaymentPBService struct {
	paymentv1.UnimplementedPaymentServiceServer
	gateway PaymentServiceGateway // 网关
	app     *supportapp.App       // 应用
}

// NewPaymentPBService 创建支付服务
func NewPaymentPBService(g PaymentServiceGateway, app *supportapp.App) *PaymentPBService {
	return &PaymentPBService{gateway: g, app: app}
}

// RegisterPaymentPBServer 注册支付服务
func RegisterPaymentPBServer(s grpc.ServiceRegistrar, srv paymentv1.PaymentServiceServer) {
	paymentv1.RegisterPaymentServiceServer(s, srv)
}
// Prepay 创建预支付
func (s *PaymentPBService) Prepay(ctx context.Context, req *paymentv1.PrepayRequest) (*paymentv1.PayResultResponse, error) {
	if s.app == nil {
		return nil, toStatusError(supportapp.ErrInvalidRequest)
	}
	r, err := s.app.Prepay(ctx, req.GetUserId(), req.GetOrderNo(), req.GetPayChannel())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &paymentv1.PayResultResponse{Result: payResultToPB(r)}, nil
}

// NotifyPayment 处理支付通知
func (s *PaymentPBService) NotifyPayment(ctx context.Context, req *paymentv1.PaymentNotifyRequest) (*emptypb.Empty, error) {
	if s.app == nil {
		return nil, toStatusError(supportapp.ErrInvalidRequest)
	}
	params := make(map[string]string, len(req.GetParams())+2)
	for k, v := range req.GetParams() {
		params[k] = v
	}
	if req.GetOrderNo() != "" {
		params["orderNo"] = req.GetOrderNo()
	}
	if req.GetTransactionNo() != "" {
		params["transactionNo"] = req.GetTransactionNo()
	}
	if err := s.app.Notify(ctx, req.GetChannel(), params); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}
// CreatePayment 创建支付
func (s *PaymentPBService) CreatePayment(ctx context.Context, req *paymentv1.CreatePaymentRequest) (*paymentv1.PayResultResponse, error) {
	r, err := s.gateway.CreatePayment(ctx, createPayFromPB(req.GetRequest()))
	if err != nil {
		return nil, toStatusError(err)
	}
	return &paymentv1.PayResultResponse{Result: payResultToPB(r)}, nil
}

// QueryPayment 查询支付状态
func (s *PaymentPBService) QueryPayment(ctx context.Context, req *orderv1.OrderNoRequest) (*paymentv1.PayQueryResponse, error) {
	r, err := s.gateway.QueryPayment(ctx, req.GetOrderNo())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &paymentv1.PayQueryResponse{Result: payQueryResultToPB(r)}, nil
}

// QueryPayments 批量查询支付状态
func (s *PaymentPBService) QueryPayments(ctx context.Context, req *orderv1.OrderNosRequest) (*paymentv1.PayQueryListResponse, error) {
	nos := uniqueNonEmptyStrings(req.GetOrderNos())
	results, err := s.gateway.QueryPayments(ctx, nos)
	if err != nil {
		return nil, toStatusError(err)
	}
	return &paymentv1.PayQueryListResponse{Results: payQueryResultsToPB(results, nos)}, nil
}
// ClosePayment 关闭支付
func (s *PaymentPBService) ClosePayment(ctx context.Context, req *orderv1.OrderNoRequest) (*emptypb.Empty, error) {
	if err := s.gateway.ClosePayment(ctx, req.GetOrderNo()); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// FreeCardPBService 自由卡服务protobuf实现
type FreeCardPBService struct {
	free_cardv1.UnimplementedFreeCardServiceServer
	gateway supportapp.FreeCardGateway // 网关
}

// NewFreeCardPBService 创建自由卡服务
func NewFreeCardPBService(g supportapp.FreeCardGateway) *FreeCardPBService {
	return &FreeCardPBService{gateway: g}
}

// RegisterFreeCardPBServer 注册自由卡服务
func RegisterFreeCardPBServer(s grpc.ServiceRegistrar, srv free_cardv1.FreeCardServiceServer) {
	free_cardv1.RegisterFreeCardServiceServer(s, srv)
}
// IssueCard 发放自由卡
func (s *FreeCardPBService) IssueCard(ctx context.Context, req *free_cardv1.IssueCardRequest) (*free_cardv1.CardNoResponse, error) {
	no, err := s.gateway.IssueCard(ctx, issueCardFromPB(req.GetRequest()))
	if err != nil {
		return nil, toStatusError(err)
	}
	return &free_cardv1.CardNoResponse{CardNo: no}, nil
}

// GetCard 获取自由卡信息
func (s *FreeCardPBService) GetCard(ctx context.Context, req *free_cardv1.CardNoRequest) (*free_cardv1.FreeCardResponse, error) {
	c, err := s.gateway.GetCard(ctx, req.GetCardNo())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &free_cardv1.FreeCardResponse{Card: freeCardToPB(c)}, nil
}

// ListCards 列出用户的自由卡
func (s *FreeCardPBService) ListCards(ctx context.Context, req *commonv1.UserIDRequest) (*free_cardv1.FreeCardListResponse, error) {
	cards, err := s.gateway.ListCards(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &free_cardv1.FreeCardListResponse{Cards: freeCardsToPB(cards)}, nil
}
// ActivateCard 激活自由卡
func (s *FreeCardPBService) ActivateCard(ctx context.Context, req *free_cardv1.ActivateCardRequest) (*emptypb.Empty, error) {
	if err := s.gateway.ActivateCard(ctx, activateCardFromPB(req)); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// FreezeCard 冻结自由卡
func (s *FreeCardPBService) FreezeCard(ctx context.Context, req *free_cardv1.CardNoRequest) (*emptypb.Empty, error) {
	if err := s.gateway.FreezeCard(ctx, req.GetCardNo()); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// UnfreezeCard 解冻自由卡
func (s *FreeCardPBService) UnfreezeCard(ctx context.Context, req *free_cardv1.CardNoRequest) (*emptypb.Empty, error) {
	if err := s.gateway.UnfreezeCard(ctx, req.GetCardNo()); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// OrderSyncServiceGateway 订单同步服务网关接口
type OrderSyncServiceGateway interface {
	supportapp.OrderSyncGateway
	ListSyncedOrdersByOrderNos(ctx context.Context, orderNos []string) (map[string]paymentdomain.SyncedOrder, error)
}

// OrderSyncPBService 订单同步服务protobuf实现
type OrderSyncPBService struct {
	order_syncv1.UnimplementedOrderSyncServiceServer
	gateway OrderSyncServiceGateway // 网关
}

// NewOrderSyncPBService 创建订单同步服务
func NewOrderSyncPBService(g OrderSyncServiceGateway) *OrderSyncPBService {
	return &OrderSyncPBService{gateway: g}
}

// RegisterOrderSyncPBServer 注册订单同步服务
func RegisterOrderSyncPBServer(s grpc.ServiceRegistrar, srv order_syncv1.OrderSyncServiceServer) {
	order_syncv1.RegisterOrderSyncServiceServer(s, srv)
}
// SyncOrder 同步订单
func (s *OrderSyncPBService) SyncOrder(ctx context.Context, req *order_syncv1.SyncOrderRequest) (*emptypb.Empty, error) {
	if err := s.gateway.SyncOrder(ctx, syncOrderFromPB(req.GetRequest())); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// GetSyncedOrder 获取已同步订单
func (s *OrderSyncPBService) GetSyncedOrder(ctx context.Context, req *orderv1.OrderNoRequest) (*order_syncv1.SyncedOrderResponse, error) {
	o, err := s.gateway.GetSyncedOrder(ctx, req.GetOrderNo())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &order_syncv1.SyncedOrderResponse{Order: syncedOrderToPB(o)}, nil
}

// ListSyncedOrdersByOrderNos 批量获取已同步订单
func (s *OrderSyncPBService) ListSyncedOrdersByOrderNos(ctx context.Context, req *orderv1.OrderNosRequest) (*order_syncv1.SyncedOrderListResponse, error) {
	nos := uniqueNonEmptyStrings(req.GetOrderNos())
	orders, err := s.gateway.ListSyncedOrdersByOrderNos(ctx, nos)
	if err != nil {
		return nil, toStatusError(err)
	}
	return &order_syncv1.SyncedOrderListResponse{Orders: syncedOrdersByOrderNoToPB(orders, nos)}, nil
}
// ListSyncedOrders 列出用户的已同步订单
func (s *OrderSyncPBService) ListSyncedOrders(ctx context.Context, req *commonv1.UserIDRequest) (*order_syncv1.SyncedOrderListResponse, error) {
	orders, err := s.gateway.ListSyncedOrders(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &order_syncv1.SyncedOrderListResponse{Orders: syncedOrdersToPB(orders)}, nil
}

// MemberPBService 会员服务protobuf实现
type MemberPBService struct {
	memberv1.UnimplementedMemberServiceServer
	gateway supportapp.MemberGateway // 网关
}

// NewMemberPBService 创建会员服务
func NewMemberPBService(g supportapp.MemberGateway) *MemberPBService {
	return &MemberPBService{gateway: g}
}

// RegisterMemberPBServer 注册会员服务
func RegisterMemberPBServer(s grpc.ServiceRegistrar, srv memberv1.MemberServiceServer) {
	memberv1.RegisterMemberServiceServer(s, srv)
}
// GetUserByID 根据用户ID获取用户
func (s *MemberPBService) GetUserByID(ctx context.Context, req *commonv1.UserIDRequest) (*memberv1.UserResponse, error) {
	u, err := s.gateway.GetUserByID(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &memberv1.UserResponse{User: userToPB(u)}, nil
}

// GetUserByPhone 根据手机号获取用户
func (s *MemberPBService) GetUserByPhone(ctx context.Context, req *memberv1.PhoneRequest) (*memberv1.UserResponse, error) {
	u, err := s.gateway.GetUserByPhone(ctx, req.GetPhone())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &memberv1.UserResponse{User: userToPB(u)}, nil
}

// GetMemberLevel 获取用户会员等级
func (s *MemberPBService) GetMemberLevel(ctx context.Context, req *commonv1.UserIDRequest) (*memberv1.MemberLevelResponse, error) {
	l, err := s.gateway.GetMemberLevel(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &memberv1.MemberLevelResponse{MemberLevel: l}, nil
}
