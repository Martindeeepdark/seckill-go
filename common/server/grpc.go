// Package server 提供 gRPC 服务器管理
package server

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// GRPCServer gRPC 服务器封装
type GRPCServer struct {
	logger *slog.Logger
	server *kratosgrpc.Server
}

// NewGRPCServer 创建 gRPC 服务器
// 参数：
//   - addr: 监听地址
//   - logger: 日志记录器
//   - interceptors: 拦截器列表
func NewGRPCServer(addr string, logger *slog.Logger, interceptors ...grpc.UnaryServerInterceptor) *GRPCServer {
	opts := []kratosgrpc.ServerOption{
		kratosgrpc.Address(addr),
		kratosgrpc.Options(
			grpc.KeepaliveParams(keepalive.ServerParameters{
				Time:    30 * time.Second,
				Timeout: 10 * time.Second,
			}),
			grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
				MinTime:             10 * time.Second,
				PermitWithoutStream: true,
			}),
		),
	}
	for _, i := range interceptors {
		opts = append(opts, kratosgrpc.UnaryInterceptor(i))
	}
	return &GRPCServer{
		logger: logger,
		server: kratosgrpc.NewServer(opts...),
	}
}

// Register 注册 gRPC 服务
func (s *GRPCServer) Register(register func(grpc.ServiceRegistrar)) {
	register(s.server)
}

// Start 启动 gRPC 服务器
// 监听系统信号，优雅关闭
func (s *GRPCServer) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if s.logger != nil {
			s.logger.Info("grpc server starting")
		}
		errCh <- s.server.Start(ctx)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		// 收到终止信号，执行优雅关闭
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return fmt.Errorf("stop grpc server: %w", s.server.Stop(shutdownCtx))
	}
}

// Stop 停止 gRPC 服务器
func (s *GRPCServer) Stop(ctx context.Context) error {
	return fmt.Errorf("stop grpc server: %w", s.server.Stop(ctx))
}
