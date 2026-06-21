// Package main 提供 seckill-job 的服务入口
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kratos/kratos/v2/registry"
	"google.golang.org/grpc"

	activityv1 "seckill-api/activity/v1"
	orderv1 "seckill-api/order/v1"
	ordersyncv1 "seckill-api/order_sync/v1"
	paymentv1 "seckill-api/payment/v1"
	riskv1 "seckill-api/risk/v1"
	stockv1 "seckill-api/stock/v1"

	"seckill-common/discovery"
	"seckill-common/logger"
	"seckill-common/rpcclient"
	"seckill-common/tracing"

	"seckill-job-service/internal/application/job"
	jobconfig "seckill-job-service/internal/config"
	"seckill-job-service/internal/infrastructure/gateway"
)

// main 是服务入口函数
func main() {
	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	if err := run(configPath); err != nil {
		log.Fatal(err)
	}
}

// run 启动定时任务服务
func run(configPath string) error {
	cfg, jobCfg, rpcCfg, appCBCfg, err := jobconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config from %s: %w", configPath, err)
	}
	if err := logger.Init(cfg); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer logger.Sync()
	log := logger.GetSlogLogger() // 获取 slog 兼容层用于基础设施

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shutdownTrace := tracing.InitTracing(cfg, log)
	defer func() {
		if err := shutdownTrace(context.Background()); err != nil {
			log.Error("shutdown trace provider", "err", err)
		}
	}()

	disc, err := discovery.NewRPCDiscovery(ctx, cfg, log)
	if err != nil {
		return fmt.Errorf("create RPC discovery: %w", err)
	}

	// 创建应用层熔断器工厂函数
	// 当配置启用时，为每个下游服务创建独立的熔断器
	// 当配置禁用时，创建一个始终放行的熔断器
	newCB := func(serviceName string) *gateway.CircuitBreaker {
		if !appCBCfg.Enabled {
			return gateway.NewCircuitBreaker(serviceName, gateway.CircuitBreakerConfig{
				Enabled: false,
				Success: 1.0,
				Request: 1,
				Window:  0,
			}, log)
		}
		return gateway.NewCircuitBreaker(serviceName, gateway.CircuitBreakerConfig{
			Enabled: true,
			Success: appCBCfg.Success,
			Request: appCBCfg.Request,
			Window:  appCBCfg.Window,
		}, log)
	}

	// 连接各个下游服务
	activityRPC, err := dialService(ctx, disc, rpcCfg, "activity-service", rpcCfg.ActivityAddr)
	if err != nil {
		return err
	}
	defer func() {
		if err := activityRPC.Close(); err != nil {
			log.Error("close activity RPC", "err", err)
		}
	}()

	stockRPC, err := dialService(ctx, disc, rpcCfg, "stock-service", rpcCfg.StockAddr)
	if err != nil {
		return err
	}
	defer func() {
		if err := stockRPC.Close(); err != nil {
			log.Error("close stock RPC", "err", err)
		}
	}()

	orderRPC, err := dialService(ctx, disc, rpcCfg, "order-service", rpcCfg.OrderAddr)
	if err != nil {
		return err
	}
	defer func() {
		if err := orderRPC.Close(); err != nil {
			log.Error("close order RPC", "err", err)
		}
	}()

	riskRPC, err := dialService(ctx, disc, rpcCfg, "risk-service", rpcCfg.RiskAddr)
	if err != nil {
		return err
	}
	defer func() {
		if err := riskRPC.Close(); err != nil {
			log.Error("close risk RPC", "err", err)
		}
	}()

	supportRPC, err := dialService(ctx, disc, rpcCfg, "support-service", rpcCfg.SupportAddr)
	if err != nil {
		return err
	}
	defer func() {
		if err := supportRPC.Close(); err != nil {
			log.Error("close support RPC", "err", err)
		}
	}()

	// 创建各个服务的网关客户端
	activityClient := gateway.NewActivityClient(activityv1.NewActivityServiceClient(activityRPC), log, newCB("activity"))
	orderClient := gateway.NewOrderClient(orderv1.NewOrderServiceClient(orderRPC), log, newCB("order"))
	stockClient := gateway.NewStockClient(stockv1.NewStockServiceClient(stockRPC), log, newCB("stock"))
	paymentClient := gateway.NewPaymentClient(paymentv1.NewPaymentServiceClient(supportRPC), log, newCB("payment"))
	orderSyncClient := gateway.NewOrderSyncClient(ordersyncv1.NewOrderSyncServiceClient(supportRPC), log, newCB("ordersync"))
	riskClient := gateway.NewRiskClient(riskv1.NewRiskServiceClient(riskRPC), log, newCB("risk"))

	// 创建任务运行器
	runner := job.NewRunner(job.Config{
		RunOnStart:                  jobCfg.RunOnStart,
		ActivityStatusCheckInterval: jobCfg.ActivityStatusCheckInterval,
		TimeoutOrderCheckInterval:   jobCfg.TimeoutOrderCheckInterval,
		StockReleaseCheckInterval:   jobCfg.StockReleaseCheckInterval,
		ActivityDataCleanupInterval: jobCfg.ActivityDataCleanupInterval,
		ActivityDataRetention:       jobCfg.ActivityDataRetention,
		RiskUserCleanupInterval:     jobCfg.RiskUserCleanupInterval,
		DailyStatisticsInterval:     jobCfg.DailyStatisticsInterval,
		CacheWarmupInterval:         jobCfg.CacheWarmupInterval,
		CacheRefreshInterval:        jobCfg.CacheRefreshInterval,
		CacheWarmupAhead:            jobCfg.CacheWarmupAhead,
		PaymentReconcileInterval:    jobCfg.PaymentReconcileInterval,
		OrderSyncCheckInterval:      jobCfg.OrderSyncCheckInterval,
	}, activityClient, orderClient, stockClient, paymentClient, log,
		job.WithOrderSync(orderSyncClient),
		job.WithRiskGateway(riskClient),
	)

	log.Info("seckill-job started",
		"activityStatusInterval", jobCfg.ActivityStatusCheckInterval.String(),
		"timeoutOrderInterval", jobCfg.TimeoutOrderCheckInterval.String(),
		"stockReleaseInterval", jobCfg.StockReleaseCheckInterval.String(),
		"paymentReconcileInterval", jobCfg.PaymentReconcileInterval.String(),
		"orderSyncCheckInterval", jobCfg.OrderSyncCheckInterval.String(),
		"runOnStart", jobCfg.RunOnStart,
		"appCircuitBreaker", appCBCfg.Enabled,
	)
	if err := runner.Run(ctx); err != nil {
		return fmt.Errorf("runner exited: %w", err)
	}
	return nil
}

// dialService 拨号连接服务
func dialService(ctx context.Context, disc registry.Discovery, cfg jobconfig.RPCConfig, serviceName string, endpoint string) (*grpc.ClientConn, error) {
	conn, err := rpcclient.Dial(ctx, rpcclient.Config{
		Endpoint:             endpoint,
		Timeout:              cfg.Timeout,
		CircuitBreaker:       cfg.CircuitBreaker,
		CircuitBreakerPolicy: cfg.CircuitBreakerPolicy,
		Discovery:            disc,
	})
	if err != nil {
		return nil, fmt.Errorf("dial %s (%s): %w", serviceName, endpoint, err)
	}
	return conn, nil
}
