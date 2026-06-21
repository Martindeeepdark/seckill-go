// Package main 提供 support-service 的服务入口
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"

	orderv1 "seckill-api/order/v1"
	commconfig "seckill-common/config"
	"seckill-common/discovery"
	comminterceptor "seckill-common/interceptor"
	commlogger "seckill-common/logger"
	"seckill-common/rpcclient"
	commserver "seckill-common/server"
	commtracing "seckill-common/tracing"

	supportapp "seckill-support-service/internal/application"
	"seckill-support-service/internal/infrastructure/gateway"
	"seckill-support-service/internal/infrastructure/ledger"
	"seckill-support-service/internal/infrastructure/queue"
	"seckill-support-service/internal/infrastructure/rpc"
)

// main 是服务入口函数
func main() {
	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	if err := start(configPath); err != nil {
		log.Fatal(err)
	}
}

// start 启动支持服务
func start(configPath string) error {
	ctx := context.Background()

	cfg, err := commconfig.Load(configPath, "support-service", commconfig.DefaultEndpoints())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := commlogger.Init(cfg); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer commlogger.Sync()
	log := commlogger.GetSlogLogger() // 获取 slog 兼容层用于基础设施

	shutdownTrace := commtracing.InitTracing(cfg, log)
	defer func() { _ = shutdownTrace(ctx) }() //nolint:errcheck // best-effort cleanup

	deregister, err := discovery.RegisterGRPCService(ctx, cfg, log)
	if err != nil {
		return fmt.Errorf("register grpc service: %w", err)
	}
	defer func() { _ = deregister(ctx) }() //nolint:errcheck // best-effort cleanup

	rpcDiscovery, err := discovery.NewRPCDiscovery(ctx, cfg, log)
	if err != nil {
		return fmt.Errorf("create rpc discovery: %w", err)
	}
	// 连接订单服务
	orderConn, err := rpcclient.Dial(ctx, rpcclient.Config{
		Endpoint:       rpcclient.Endpoint("order-service", "", ""),
		CircuitBreaker: true,
		Discovery:      rpcDiscovery,
	})
	if err != nil {
		return fmt.Errorf("dial order service: %w", err)
	}
	defer func() { _ = orderConn.Close() }() //nolint:errcheck // best-effort cleanup

	// 创建网关和服务实例
	payments := gateway.NewMockPaymentGateway()
	l := ledger.NewSupportLedger()
	orderCB := gateway.NewCircuitBreaker("order-service", 5, 30*time.Second, log)
	orders := gateway.NewOrderGateway(orderv1.NewOrderServiceClient(orderConn), orderCB)

	var appOptions []supportapp.AppOption
	if cfg.NATSAddr != "" {
		postPayPublisher, err := queue.NewNATSPostPayPublisher(cfg.NATSAddr, "SECKILL", "seckill.post_pay", log)
		if err != nil {
			log.Warn("nats post-pay publisher unavailable, using inline post-pay", "addr", cfg.NATSAddr, "error", err)
		} else {
			defer func() { _ = postPayPublisher.Close() }() //nolint:errcheck // best-effort cleanup
			appOptions = append(appOptions, supportapp.WithPostPayTasks(postPayPublisher))
		}
	}
	paymentApp := supportapp.NewApp(orders, payments, l, l, log, appOptions...)

	srv := commserver.NewGRPCServer(cfg.GRPCAddr, log, comminterceptor.TraceUnaryServerInterceptor())
	srv.Register(func(registrar grpc.ServiceRegistrar) {
		rpc.RegisterMemberPBServer(registrar, rpc.NewMemberPBService(l))
		rpc.RegisterPaymentPBServer(registrar, rpc.NewPaymentPBService(payments, paymentApp))
		rpc.RegisterFreeCardPBServer(registrar, rpc.NewFreeCardPBService(l))
		rpc.RegisterOrderSyncPBServer(registrar, rpc.NewOrderSyncPBService(l))
	})
	return fmt.Errorf("start grpc server: %w", srv.Start())
}
