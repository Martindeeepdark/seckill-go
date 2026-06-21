// Package main 提供订单服务的主程序入口
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc"

	"seckill-common/discovery"
	"seckill-common/interceptor"
	"seckill-common/logger"
	commserver "seckill-common/server"
	"seckill-common/tracing"

	"seckill-order-service/internal/config"
	"seckill-order-service/internal/infrastructure"
	"seckill-order-service/internal/infrastructure/cache"
	"seckill-order-service/internal/infrastructure/rpc"
)

// main 是程序入口函数
func main() {
	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	if err := start(configPath); err != nil {
		log.Fatal(err)
	}
}

// start 启动订单服务
// configPath: 配置文件路径
// 返回错误表示启动失败
func start(configPath string) error {
	// 加载配置
	cfg, cacheCfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	// 初始化日志
	if err := logger.Init(cfg); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer logger.Sync()
	log := logger.GetSlogLogger() // 获取 slog 兼容层用于基础设施
	// 初始化链路追踪
	shutdownTrace := tracing.InitTracing(cfg, log)
	defer func() { _ = shutdownTrace(context.Background()) }() //nolint:errcheck // best-effort cleanup
	// 初始化数据存储
	repo := infrastructure.NewStore(context.Background(), cfg, log)
	// 包装本地缓存层（BigCache local-only），加速热门订单查询
	cachedRepo, err := cache.NewOrderCache(repo, cacheConfigFromOrder(cacheCfg), log)
	if err != nil {
		return fmt.Errorf("create order cache: %w", err)
	}
	defer func() { _ = cachedRepo.Close() }() //nolint:errcheck // best-effort cleanup
	// 注册服务发现
	deregister, err := discovery.RegisterGRPCService(context.Background(), cfg, log)
	if err != nil {
		return fmt.Errorf("register grpc service: %w", err)
	}
	defer func() { _ = deregister(context.Background()) }() //nolint:errcheck // best-effort cleanup

	// 启动 gRPC 服务器
	srv := commserver.NewGRPCServer(cfg.GRPCAddr, log, interceptor.TraceUnaryServerInterceptor())
	srv.Register(func(registrar grpc.ServiceRegistrar) {
		rpc.RegisterOrderPBServer(registrar, rpc.NewOrderPBService(cachedRepo))
	})
	return fmt.Errorf("start grpc server: %w", srv.Start())
}

func cacheConfigFromOrder(c config.CacheConfig) cache.Config {
	return cache.Config{
		Order: cache.EntryConfig{
			LocalTTL:   c.Order.LocalTTL,
			Shards:     c.Order.Shards,
			MaxEntries: c.Order.MaxEntries,
		},
	}
}
