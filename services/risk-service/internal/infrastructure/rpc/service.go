// Package rpc 提供 gRPC 服务相关实现
package rpc

import (
	"context"

	commonv1 "seckill-api/common/v1"
	riskv1 "seckill-api/risk/v1"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// RiskPBService 风险控制 gRPC 服务实现
// 嵌入 UnimplementedRiskServiceServer 以确保向前兼容
type RiskPBService struct {
	riskv1.UnimplementedRiskServiceServer             // 嵌入未实现的服务器，用于向前兼容
	gateway                               RiskGateway // 风险控制网关，处理业务逻辑
}

// NewRiskPBService 创建风险控制 gRPC 服务实例
// 参数：gateway-风险控制网关
// 返回：RiskPBService 实例
func NewRiskPBService(gateway RiskGateway) *RiskPBService {
	return &RiskPBService{gateway: gateway}
}

// RegisterRiskPBServer 注册风险控制服务到 gRPC 服务器
// 参数：s-gRPC 服务注册器，srv-风险控制服务实例
func RegisterRiskPBServer(s grpc.ServiceRegistrar, srv riskv1.RiskServiceServer) {
	riskv1.RegisterRiskServiceServer(s, srv)
}

// MarkSuspicious 实现 gRPC 方法：标记可疑用户
// 将 protobuf 请求转换为领域实体并调用网关处理
func (s *RiskPBService) MarkSuspicious(ctx context.Context, req *riskv1.RiskMarkRequest) (*emptypb.Empty, error) {
	if err := s.gateway.MarkSuspicious(ctx, activityFromPB(req.GetActivity()), req.GetUserId(), req.GetRequestIp()); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// Evaluate 实现 gRPC 方法：评估用户风险等级
// 调用网关进行评估并将结果转换为 protobuf 响应
func (s *RiskPBService) Evaluate(ctx context.Context, req *riskv1.RiskEvaluateRequest) (*riskv1.RiskEvaluationResponse, error) {
	evaluation, err := s.gateway.Evaluate(ctx, req.GetUserId(), req.GetRequestIp())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &riskv1.RiskEvaluationResponse{Evaluation: riskEvaluationToPB(evaluation)}, nil
}

// RecordAction 实现 gRPC 方法：记录风险行为
// 将 protobuf 请求转换为领域实体并持久化
func (s *RiskPBService) RecordAction(ctx context.Context, req *riskv1.RiskRecordRequest) (*emptypb.Empty, error) {
	if err := s.gateway.RecordAction(ctx, riskRecordFromPB(req.GetRecord())); err != nil {
		return nil, toStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

// IsRiskUser 实现 gRPC 方法：检查用户是否为风险用户
// 返回布尔值响应
func (s *RiskPBService) IsRiskUser(ctx context.Context, req *riskv1.RiskUserRequest) (*commonv1.BoolResponse, error) {
	ok, err := s.gateway.IsRiskUser(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &commonv1.BoolResponse{Ok: ok}, nil
}

// CleanupExpiredRiskUsers 实现 gRPC 方法：清理过期的风险用户
// 返回清理的记录数量
func (s *RiskPBService) CleanupExpiredRiskUsers(ctx context.Context, _ *emptypb.Empty) (*riskv1.RiskCleanupResponse, error) {
	deleted, err := s.gateway.CleanupExpiredRiskUsers(ctx)
	if err != nil {
		return nil, toStatusError(err)
	}
	return &riskv1.RiskCleanupResponse{Deleted: int64(deleted)}, nil
}
