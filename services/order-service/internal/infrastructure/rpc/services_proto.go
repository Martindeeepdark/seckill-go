// Package rpc 提供 gRPC 服务的 Protobuf 实现
package rpc

import (
	"context"
	"time"

	orderv1 "seckill-api/order/v1"

	"seckill-order-service/internal/domain/entity"
	"seckill-order-service/internal/domain/repository"
	"seckill-order-service/internal/infrastructure/persistence"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OrderPBService 订单 Protobuf 服务实现
type OrderPBService struct {
	orderv1.UnimplementedOrderServiceServer                       // 嵌入未实现的服务器接口
	repo                                    repository.OrderStore // 订单存储
}

// NewOrderPBService 创建订单 Protobuf 服务实例
// repo: 订单仓储
// 返回服务实例
func NewOrderPBService(repo repository.OrderStore) *OrderPBService {
	return &OrderPBService{repo: repo}
}

// RegisterOrderPBServer 注册订单 Protobuf 服务到 gRPC 服务器
// s: gRPC 服务注册器
// srv: 订单服务实例
func RegisterOrderPBServer(s grpc.ServiceRegistrar, srv orderv1.OrderServiceServer) {
	orderv1.RegisterOrderServiceServer(s, srv)
}

// CreateOrder 创建订单
// ctx: 上下文
// req: 订单创建请求
// 返回空响应和错误
func (s *OrderPBService) CreateOrder(ctx context.Context, req *orderv1.OrderResponse) (*emptypb.Empty, error) {
	pbOrder := req.GetOrder()
	if pbOrder == nil {
		return nil, toStatusError(persistence.ErrInvalidState)
	}
	// 将 Protobuf 消息转换为领域实体
	order := protoToEntity(pbOrder)
	if err := s.repo.CreateOrder(ctx, order); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// GetOrder 获取订单
// ctx: 上下文
// req: 订单号请求
// 返回订单响应和错误
func (s *OrderPBService) GetOrder(ctx context.Context, req *orderv1.OrderNoRequest) (*orderv1.OrderResponse, error) {
	order, err := s.repo.GetOrder(ctx, req.GetOrderNo())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &orderv1.OrderResponse{Order: entityToProto(order)}, nil
}

// GetOrderByUserAndTrace 根据 (user_id, trace_id) 组合查询订单
// 供 seckill-processor 在 CreateOrder 触发 DuplicateKey (23505) 后回查已存在订单
// ctx: 上下文
// req: 包含 user_id 和 trace_id 的请求
// 返回订单响应和错误:
//   - user_id == 0 → codes.InvalidArgument
//   - trace_id == "" → codes.InvalidArgument
//   - 未找到 → codes.NotFound
//   - 其他错误 → toStatusError
func (s *OrderPBService) GetOrderByUserAndTrace(ctx context.Context, req *orderv1.GetOrderByUserAndTraceRequest) (*orderv1.OrderResponse, error) {
	if req.GetUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GetTraceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	order, err := s.repo.GetByUserAndTrace(ctx, req.GetUserId(), req.GetTraceId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &orderv1.OrderResponse{Order: entityToProto(order)}, nil
}

// ListOrdersByActivity 根据活动编号列出订单
// ctx: 上下文
// req: 按活动列订单请求
// 返回订单列表响应和错误
func (s *OrderPBService) ListOrdersByActivity(ctx context.Context, req *orderv1.OrderListByActivityRequest) (*orderv1.OrderListResponse, error) {
	orders, err := s.repo.ListOrdersByActivity(ctx, req.GetActivityNo())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &orderv1.OrderListResponse{Orders: entityListToProto(orders)}, nil
}

// ListOrdersByActivities 根据多个活动编号批量列出订单
// ctx: 上下文
// req: 按多个活动列订单请求
// 返回按活动分组的订单列表响应和错误
func (s *OrderPBService) ListOrdersByActivities(ctx context.Context, req *orderv1.OrderListByActivitiesRequest) (*orderv1.OrderListByActivitiesResponse, error) {
	result, err := s.repo.ListOrdersByActivities(ctx, req.GetActivityNos())
	if err != nil {
		return nil, toStatusError(err)
	}
	// 转换为 Protobuf 消息格式
	items := make([]*orderv1.OrdersByActivity, 0, len(result))
	for activityNo, orders := range result {
		items = append(items, &orderv1.OrdersByActivity{
			ActivityNo: activityNo,
			Orders:     entityListToProto(orders),
		})
	}
	return &orderv1.OrderListByActivitiesResponse{Items: items}, nil
}

// ListOrdersByUser 根据用户ID列出订单
// ctx: 上下文
// req: 按用户列订单请求
// 返回订单列表响应和错误
func (s *OrderPBService) ListOrdersByUser(ctx context.Context, req *orderv1.OrderListByUserRequest) (*orderv1.OrderListResponse, error) {
	orders, err := s.repo.ListOrdersByUser(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &orderv1.OrderListResponse{Orders: entityListToProto(orders)}, nil
}

// MarkPaid 标记订单为已支付
// ctx: 上下文
// req: 标记订单已支付请求
// 返回空响应和错误
func (s *OrderPBService) MarkPaid(ctx context.Context, req *orderv1.MarkOrderPaidRequest) (*emptypb.Empty, error) {
	var paidAt time.Time
	if req.GetPaidAt() != nil {
		paidAt = req.GetPaidAt().AsTime()
	} else {
		paidAt = time.Now()
	}
	if err := s.repo.MarkOrderPaid(ctx, req.GetOrderNo(), req.GetTransactionNo(), paidAt); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// CloseOrder 关闭订单
// ctx: 上下文
// req: 订单号请求
// 返回空响应和错误
func (s *OrderPBService) CloseOrder(ctx context.Context, req *orderv1.OrderNoRequest) (*emptypb.Empty, error) {
	if err := s.repo.CloseOrder(ctx, req.GetOrderNo()); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// protoToEntity 将 Protobuf 订单消息转换为领域实体
func protoToEntity(pb *orderv1.Order) entity.Order {
	o := entity.Order{
		OrderNo:        pb.GetOrderNo(),
		UserID:         pb.GetUserId(),
		ActivityNo:     pb.GetActivityNo(),
		SKUNo:          pb.GetSkuNo(),
		Quantity:       pb.GetQuantity(),
		PayAmount:      pb.GetPayAmount(),
		Status:         pb.GetStatus(),
		TraceID:        pb.GetTraceId(),
		RequestTraceID: pb.GetRequestTraceId(),
		TransactionNo:  pb.GetTransactionNo(),
	}
	// 处理时间字段
	if pb.GetCreatedAt() != nil {
		o.CreatedAt = pb.GetCreatedAt().AsTime()
	}
	if pb.GetPaidAt() != nil {
		t := pb.GetPaidAt().AsTime()
		o.PaidAt = &t
	}
	if pb.GetClosedAt() != nil {
		t := pb.GetClosedAt().AsTime()
		o.ClosedAt = &t
	}
	return o
}

// entityToProto 将领域实体转换为 Protobuf 订单消息
func entityToProto(o entity.Order) *orderv1.Order {
	pb := &orderv1.Order{
		OrderNo:        o.OrderNo,
		UserId:         o.UserID,
		ActivityNo:     o.ActivityNo,
		SkuNo:          o.SKUNo,
		Quantity:       o.Quantity,
		PayAmount:      o.PayAmount,
		Status:         o.Status,
		TraceId:        o.TraceID,
		RequestTraceId: o.RequestTraceID,
		TransactionNo:  o.TransactionNo,
	}
	// 处理时间字段
	if !o.CreatedAt.IsZero() {
		pb.CreatedAt = timestamppb.New(o.CreatedAt)
	}
	if o.PaidAt != nil {
		pb.PaidAt = timestamppb.New(*o.PaidAt)
	}
	if o.ClosedAt != nil {
		pb.ClosedAt = timestamppb.New(*o.ClosedAt)
	}
	return pb
}

// entityListToProto 将领域实体列表转换为 Protobuf 消息列表
func entityListToProto(orders []entity.Order) []*orderv1.Order {
	result := make([]*orderv1.Order, 0, len(orders))
	for _, o := range orders {
		result = append(result, entityToProto(o))
	}
	return result
}
