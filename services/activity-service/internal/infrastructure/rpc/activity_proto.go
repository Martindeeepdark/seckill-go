// Package rpc 提供秒杀活动的 gRPC 服务实现。
package rpc

import (
	"context"

	domain "seckill-activity-service/internal/domain/entity"
	activityv1 "seckill-api/activity/v1"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ActivityGateway 定义 RPC 服务所需的活动网关接口。
type ActivityGateway interface {
	ListActivities(ctx context.Context) ([]domain.Activity, error)
	GetActivity(ctx context.Context, activityNo string) (domain.Activity, error)
	GetSKU(ctx context.Context, activityNo, skuNo string) (domain.SKU, error)
	CreateActivity(ctx context.Context, activity domain.Activity) (domain.Activity, error)
	UpdateActivity(ctx context.Context, activity domain.Activity) error
	UpdateActivityStatus(ctx context.Context, activityNo string, status int64) error
	AddActivitySKU(ctx context.Context, activityNo string, sku domain.SKU) error
	RemoveActivitySKU(ctx context.Context, activityNo, skuNo string) error
}

// ActivityPBService 实现活动 gRPC 服务。
type ActivityPBService struct {
	activityv1.UnimplementedActivityServiceServer
	gateway ActivityGateway
}

// NewActivityPBService 创建活动 gRPC 服务实例。
func NewActivityPBService(gateway ActivityGateway) *ActivityPBService {
	return &ActivityPBService{gateway: gateway}
}

// RegisterActivityPBServer 注册活动服务到 gRPC 服务器。
func RegisterActivityPBServer(s grpc.ServiceRegistrar, srv activityv1.ActivityServiceServer) {
	activityv1.RegisterActivityServiceServer(s, srv)
}

// ListActivities 列出所有活动。
func (s *ActivityPBService) ListActivities(ctx context.Context, _ *emptypb.Empty) (*activityv1.ActivityListResponse, error) {
	activities, err := s.gateway.ListActivities(ctx)
	if err != nil {
		return nil, toStatusError(err)
	}
	out := make([]*activityv1.Activity, 0, len(activities))
	for _, a := range activities {
		out = append(out, activityToPB(a))
	}
	return &activityv1.ActivityListResponse{Activities: out}, nil
}

// GetActivity 获取单个活动详情。
func (s *ActivityPBService) GetActivity(ctx context.Context, req *activityv1.ActivityNoRequest) (*activityv1.ActivityResponse, error) {
	a, err := s.gateway.GetActivity(ctx, req.GetActivityNo())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &activityv1.ActivityResponse{Activity: activityToPB(a)}, nil
}

// GetSKU 获取活动商品。
func (s *ActivityPBService) GetSKU(ctx context.Context, req *activityv1.SKURequest) (*activityv1.SKUResponse, error) {
	sku, err := s.gateway.GetSKU(ctx, req.GetActivityNo(), req.GetSkuNo())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &activityv1.SKUResponse{Sku: skuToPB(sku)}, nil
}

// CreateActivity 创建新活动。
func (s *ActivityPBService) CreateActivity(ctx context.Context, req *activityv1.ActivityResponse) (*activityv1.ActivityResponse, error) {
	a, err := s.gateway.CreateActivity(ctx, activityFromPB(req.GetActivity()))
	if err != nil {
		return nil, toStatusError(err)
	}
	return &activityv1.ActivityResponse{Activity: activityToPB(a)}, nil
}

// UpdateActivity 更新活动信息。
func (s *ActivityPBService) UpdateActivity(ctx context.Context, req *activityv1.ActivityResponse) (*emptypb.Empty, error) {
	if err := s.gateway.UpdateActivity(ctx, activityFromPB(req.GetActivity())); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// UpdateActivityStatus 更新活动状态。
func (s *ActivityPBService) UpdateActivityStatus(ctx context.Context, req *activityv1.ActivityStatusRequest) (*emptypb.Empty, error) {
	if err := s.gateway.UpdateActivityStatus(ctx, req.GetActivityNo(), req.GetStatus()); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// AddActivitySKU 向活动添加商品。
func (s *ActivityPBService) AddActivitySKU(ctx context.Context, req *activityv1.ActivitySKURequest) (*emptypb.Empty, error) {
	if err := s.gateway.AddActivitySKU(ctx, req.GetActivityNo(), skuFromPB(req.GetSku())); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// RemoveActivitySKU 从活动移除商品。
func (s *ActivityPBService) RemoveActivitySKU(ctx context.Context, req *activityv1.SKURequest) (*emptypb.Empty, error) {
	if err := s.gateway.RemoveActivitySKU(ctx, req.GetActivityNo(), req.GetSkuNo()); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}
