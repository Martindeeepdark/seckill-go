// Package main 提供秒杀活动服务的启动入口。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"

	"seckill-activity-service/internal/config"
	"seckill-activity-service/internal/domain/repository"
	"seckill-activity-service/internal/infrastructure/adapter"
	"seckill-activity-service/internal/infrastructure/persistence"
	"seckill-activity-service/internal/infrastructure/rpc"

	commdiscovery "seckill-common/discovery"
	commeventbus "seckill-common/eventbus"
	commLocalBus "seckill-common/eventbus/local"
	comminterceptor "seckill-common/interceptor"
	commlogger "seckill-common/logger"
	commserver "seckill-common/server"
	commtracing "seckill-common/tracing"
)

// main 是服务启动入口，支持通过命令行参数指定配置文件路径。
func main() {
	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	if err := start(configPath); err != nil {
		log.Fatal(err)
	}
}

// start 初始化并启动秒杀活动服务，包括配置加载、日志、追踪、注册中心和 RPC 服务。
func start(configPath string) error {
	ctx := context.Background()

	cfg, err := config.Load(configPath, "activity-service")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := commlogger.Init(cfg.Config); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer commlogger.Sync()
	log := commlogger.GetSlogLogger() // 获取 slog 兼容层用于基础设施

	shutdownTrace := commtracing.InitTracing(cfg.Config, log)
	defer func() { _ = shutdownTrace(ctx) }() //nolint:errcheck // best-effort cleanup

	repo := newStore(ctx, cfg, true)
	eventBus := setupEventBus(log)
	defer func() { _ = eventBus.Close() }() //nolint:errcheck // best-effort cleanup

	cleanup, err := commdiscovery.RegisterGRPCService(ctx, cfg.Config, log)
	if err != nil {
		return fmt.Errorf("register grpc service: %w", err)
	}
	defer func() { _ = cleanup(ctx) }() //nolint:errcheck // best-effort cleanup

	srv := commserver.NewGRPCServer(cfg.GRPCAddr, log, comminterceptor.TraceUnaryServerInterceptor())
	srv.Register(func(registrar grpc.ServiceRegistrar) {
		rpc.RegisterActivityPBServer(registrar, rpc.NewActivityPBService(adapter.LocalActivityGateway{Store: repo}))
	})
	return fmt.Errorf("start grpc server: %w", srv.Start())
}

// newStore 根据配置创建存储层实例，优先使用 Redis，回退到内存存储，并可选择性注入示例数据。
func newStore(ctx context.Context, cfg config.Config, seed bool) repository.Store {
	memory := persistence.NewMemoryStore()
	var repo repository.Store = memory
	if cfg.RedisAddr != "" {
		redisStore, err := persistence.NewRedisStore(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, memory)
		if err != nil {
			fmt.Printf("redis unavailable, using memory store: %v\n", err)
		} else {
			repo = redisStore
			fmt.Println("redis store enabled:", cfg.RedisAddr)
		}
	}
	if seed {
		if err := repo.AddActivity(ctx, persistence.SampleActivity(time.Now())); err != nil {
			fmt.Printf("seed sample data failed: %v\n", err)
		}
	}
	return repo
}

// setupEventBus 初始化事件总线。
func setupEventBus(_ any) commeventbus.Bus {
	// 尝试连接 NATS
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222" // 默认 NATS URL
	}

	// 简化实现：NATS 连接失败时降级到本地总线
	// 实际项目中需要使用 github.com/nats-io/nats.go.Connect
	// 这里仅作为占位符实现
	return commLocalBus.NewLocalBus()
}
