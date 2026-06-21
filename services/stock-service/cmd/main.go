// Package main 提供 stock-service 的服务入口
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"google.golang.org/grpc"

	"seckill-common/config"
	"seckill-common/discovery"
	"seckill-common/eventbus"
	"seckill-common/eventbus/hybrid"
	"seckill-common/eventbus/local"
	natsebus "seckill-common/eventbus/nats"
	"seckill-common/interceptor"
	"seckill-common/logger"

	"go.uber.org/zap"

	commserver "seckill-common/server"
	"seckill-common/tracing"

	"seckill-stock-service/internal/application"
	"seckill-stock-service/internal/infrastructure"
	"seckill-stock-service/internal/infrastructure/persistence"
	"seckill-stock-service/internal/infrastructure/rpc"

	natsgo "github.com/nats-io/nats.go"
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

// start 启动库存服务
func start(configPath string) error {
	// 加载配置
	cfg, err := config.Load(configPath, "stock-service", config.DefaultEndpoints())
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
	// 创建存储层（内存或Redis）
	repo := infrastructure.NewStore(context.Background(), cfg, log)

	// 初始化 NATS 连接（如果配置了 NATS URL）
	var nc *natsgo.Conn
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222" // 默认 NATS URL
	}

	// 尝试连接 NATS，失败时使用纯本地总线
	nc, err = natsgo.Connect(natsURL)
	if err != nil {
		slog.Default().Info("NATS connection failed, using local bus only", "error", err, "nats_url", natsURL)
		nc = nil
	} else {
		slog.Default().Info("NATS connected", "nats_url", natsURL)
	}

	// 初始化 EventBus（HybridBus：LocalBus + NATSBus）
	bus, err := setupEventBus(nc)
	if err != nil {
		return fmt.Errorf("setup event bus: %w", err)
	}
	defer func() { _ = bus.Close() }() //nolint:errcheck // cleanup on shutdown

	// 创建 AppService
	appService := application.NewStockAppService(infrastructure.LocalStockGateway{Store: repo}, bus)

	// 创建异步数据库写入器（使用内存 DB 演示）
	mockDB := persistence.NewMockDB()
	asyncWriter := persistence.NewAsyncDBWriter(mockDB, bus, zap.L())

	// 启动异步消费者
	ctx := context.Background()
	if err := asyncWriter.Start(ctx); err != nil {
		return fmt.Errorf("start async writer: %w", err)
	}
	slog.Default().Info("Async DB writer started")

	// 注册服务到发现中心
	deregister, err := discovery.RegisterGRPCService(context.Background(), cfg, log)
	if err != nil {
		return fmt.Errorf("register grpc service: %w", err)
	}
	defer func() { _ = deregister(context.Background()) }() //nolint:errcheck // best-effort cleanup

	// 创建并启动gRPC服务器
	srv := commserver.NewGRPCServer(cfg.GRPCAddr, log, interceptor.TraceUnaryServerInterceptor())
	srv.Register(func(registrar grpc.ServiceRegistrar) {
		rpc.RegisterStockPBServer(registrar, rpc.NewStockPBService(infrastructure.LocalStockGateway{Store: repo}, appService))
	})
	return fmt.Errorf("start grpc server: %w", srv.Start())
}

// setupEventBus 初始化事件总线（HybridBus：LocalBus + NATSBus）
func setupEventBus(nc *natsgo.Conn) (eventbus.Bus, error) {
	// 创建本地总线（用于本地事件分发）
	localBus := local.NewLocalBus()

	// 如果没有 NATS 连接，只使用本地总线
	if nc == nil {
		slog.Default().Info("NATS connection not provided, using local bus only")
		return localBus, nil
	}

	// 创建 NATS 总线（用于跨服务事件传播）
	natsBus, err := natsebus.NewNATSBus(nc, natsebus.NewJSONEncoder())
	if err != nil {
		return nil, fmt.Errorf("create NATS bus: %w", err)
	}

	// 创建路由器：配置哪些事件需要发布到远程
	// stock.reserved 和 stock.released 需要发布到 NATS（用于异步落库消费者）
	router := hybrid.NewDefaultRouter([]string{
		"stock.reserved",
		"stock.released",
	})

	// 创建混合总线
	bus, err := hybrid.NewHybridBus(localBus, natsBus, router)
	if err != nil {
		return nil, fmt.Errorf("create hybrid bus: %w", err)
	}

	slog.Default().Info("HybridBus initialized: LocalBus + NATSBus")
	return bus, nil
}
