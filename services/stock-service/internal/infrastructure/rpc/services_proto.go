package rpc

import (
	"context"
	"errors"

	activityv1 "seckill-api/activity/v1"
	commonv1 "seckill-api/common/v1"
	stockv1 "seckill-api/stock/v1"

	"seckill-stock-service/internal/application"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// StockPBService 库存gRPC服务实现
type StockPBService struct {
	stockv1.UnimplementedStockServiceServer
	gateway    StockGateway
	appService *application.StockAppService
}

// NewStockPBService 创建库存gRPC服务（同时需要 Gateway 和 AppService）
// Gateway 用于 Peek 和 Cleanup 操作，AppService 用于 Deduct 和 Release 操作
func NewStockPBService(gateway StockGateway, appService *application.StockAppService) *StockPBService {
	return &StockPBService{
		gateway:    gateway,
		appService: appService,
	}
}

// RegisterStockPBServer 注册库存服务到gRPC服务器
func RegisterStockPBServer(s grpc.ServiceRegistrar, srv stockv1.StockServiceServer) {
	stockv1.RegisterStockServiceServer(s, srv)
}

// Peek 查询库存（直接从 gateway 查询，无需事件）
func (s *StockPBService) Peek(ctx context.Context, req *activityv1.SKURequest) (*stockv1.StockResponse, error) {
	stock, err := s.gateway.PeekStock(ctx, req.GetActivityNo(), req.GetSkuNo())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &stockv1.StockResponse{Stock: stock}, nil
}

// Deduct 扣减库存
func (s *StockPBService) Deduct(ctx context.Context, req *stockv1.DeductRequest) (*commonv1.BoolResponse, error) {
	cmd := application.ReserveStockCommand{
		ActivityNo:    req.GetActivityNo(),
		SKUNo:         req.GetSkuNo(),
		UserID:        req.GetUserId(),
		Quantity:      req.GetQuantity(),
		PurchaseLimit: req.GetPurchaseLimit(),
		OrderNo:       req.GetOrderNo(),
	}

	err := s.appService.ReserveStock(ctx, cmd)
	if err != nil {
		// 区分库存不足错误和其他错误
		if errors.Is(err, application.ErrStockInsufficient) {
			// 库存不足不是错误，返回 Ok: false
			return &commonv1.BoolResponse{Ok: false}, nil
		}
		// 其他错误需要转换为 gRPC status
		return &commonv1.BoolResponse{Ok: false}, toStatusError(err)
	}

	return &commonv1.BoolResponse{Ok: true}, nil
}

// Release 释放库存
func (s *StockPBService) Release(ctx context.Context, req *stockv1.ReleaseRequest) (*emptypb.Empty, error) {
	cmd := application.ReleaseStockCommand{
		ActivityNo: req.GetActivityNo(),
		SKUNo:      req.GetSkuNo(),
		UserID:     req.GetUserId(),
		Quantity:   req.GetQuantity(),
		OrderNo:    req.GetOrderNo(),
	}

	if err := s.appService.ReleaseStock(ctx, cmd); err != nil {
		return nil, toStatusError(err)
	}

	return &emptypb.Empty{}, nil
}

// CleanupActivity 清理活动库存数据（保持原有逻辑）
func (s *StockPBService) CleanupActivity(ctx context.Context, req *stockv1.StockCleanupRequest) (*stockv1.StockCleanupResponse, error) {
	deleted, err := s.gateway.CleanupActivityStock(ctx, req.GetActivityNo(), req.GetSkuNos())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &stockv1.StockCleanupResponse{Deleted: deleted}, nil
}

// CleanupActivityPurchases 清理活动购买记录（保持原有逻辑）
func (s *StockPBService) CleanupActivityPurchases(ctx context.Context, req *stockv1.ActivityPurchaseCleanupRequest) (*stockv1.StockCleanupResponse, error) {
	deleted, err := s.gateway.CleanupActivityPurchases(ctx, req.GetActivityNo())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &stockv1.StockCleanupResponse{Deleted: deleted}, nil
}
