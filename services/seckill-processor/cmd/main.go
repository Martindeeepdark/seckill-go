// Package main 是秒杀处理器的入口程序
// 负责初始化所有依赖（配置、日志、RPC客户端、消息队列）
// 并启动三个核心处理协程：秒杀处理、支付超时处理、支付后处理
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	commonlogs "github.com/Martindeeepdark/go-common/logs"
	"github.com/Martindeeepdark/go-common/eventbus"
	"github.com/go-kratos/kratos/v2/registry"
	goredis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	activityv1 "seckill-api/activity/v1"
	freecardv1 "seckill-api/free_card/v1"
	orderv1 "seckill-api/order/v1"
	ordersyncv1 "seckill-api/order_sync/v1"
	paymentv1 "seckill-api/payment/v1"
	riskv1 "seckill-api/risk/v1"
	stockv1 "seckill-api/stock/v1"

	"seckill-common/discovery"
	"seckill-common/logger"
	"seckill-common/metrics"
	"seckill-common/rpcclient"
	"seckill-common/traceresult"
	"seckill-common/tracing"

	"seckill-processor-service/internal/application"
	"seckill-processor-service/internal/application/usecase"
	processorconfig "seckill-processor-service/internal/config"
	domainservice "seckill-processor-service/internal/domain/service"
	"seckill-processor-service/internal/infrastructure/cache"
	"seckill-processor-service/internal/infrastructure/gateway"
	"seckill-processor-service/internal/infrastructure/identity"
	"seckill-processor-service/internal/infrastructure/queue"
)

// main 是程序入口函数
// 支持命令行参数指定配置文件路径，默认使用 configs/config.yaml
func main() {
	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	if err := run(configPath); err != nil {
		log.Fatal(err)
	}
}

// run 初始化并运行秒杀处理器
// 执行步骤：
// 1. 加载配置
// 2. 初始化日志和链路追踪
// 3. 连接所有 RPC 服务（活动、库存、订单、风控、支持）
// 4. 创建网关客户端
// 5. 根据配置创建消息队列实现（NATS/Redis/Memory）
// 6. 启动三个处理协程并等待完成
func run(configPath string) error {
	cfg, procCfg, rpcCfg, err := processorconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := logger.Init(cfg); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer logger.Sync()
	log := logger.GetSlogLogger() // 获取 slog 兼容层用于基础设施

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// 初始化链路追踪
	shutdownTrace := tracing.InitTracing(cfg, log)
	defer func() {
		if err := shutdownTrace(context.Background()); err != nil {
			log.Warn("shutdown tracing failed", "err", err)
		}
	}()

	// 初始化服务发现
	disc, err := discovery.NewRPCDiscovery(ctx, cfg, log)
	if err != nil {
		return fmt.Errorf("init rpc discovery: %w", err)
	}

	// 连接各个微服务的 gRPC 客户端
	activityRPC, err := dialService(ctx, disc, rpcCfg, "activity-service", rpcCfg.ActivityAddr)
	if err != nil {
		return err
	}
	defer logClose(log, "activity rpc", activityRPC.Close)

	stockRPC, err := dialService(ctx, disc, rpcCfg, "stock-service", rpcCfg.StockAddr)
	if err != nil {
		return err
	}
	defer logClose(log, "stock rpc", stockRPC.Close)

	orderRPC, err := dialService(ctx, disc, rpcCfg, "order-service", rpcCfg.OrderAddr)
	if err != nil {
		return err
	}
	defer logClose(log, "order rpc", orderRPC.Close)

	riskRPC, err := dialService(ctx, disc, rpcCfg, "risk-service", rpcCfg.RiskAddr)
	if err != nil {
		return err
	}
	defer logClose(log, "risk rpc", riskRPC.Close)

	// 连接支持服务（支付、自由卡、订单同步）
	supportRPC, err := dialService(ctx, disc, rpcCfg, "support-service", rpcCfg.SupportAddr)
	if err != nil {
		return err
	}
	defer logClose(log, "support rpc", supportRPC.Close)

	// 创建网关客户端封装
	activityClient := gateway.NewActivityClient(activityv1.NewActivityServiceClient(activityRPC), log)
	stockClient := gateway.NewStockClient(stockv1.NewStockServiceClient(stockRPC), log)
	riskClient := gateway.NewRiskClient(riskv1.NewRiskServiceClient(riskRPC), log)
	orderClient := gateway.NewOrderClient(orderv1.NewOrderServiceClient(orderRPC), log)
	paymentClient := gateway.NewPaymentClient(paymentv1.NewPaymentServiceClient(supportRPC), log)
	freeCardClient := gateway.NewFreeCardClient(freecardv1.NewFreeCardServiceClient(supportRPC), log)
	orderSyncClient := gateway.NewOrderSyncClient(ordersyncv1.NewOrderSyncServiceClient(supportRPC), log)

	var redisClient *goredis.Client
	if cfg.RedisAddr != "" {
		redisClient = goredis.NewClient(&goredis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		defer logClose(log, "redis client", redisClient.Close)
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis ping: %w", err)
		}
		metrics.SetClient(redisClient)
		commonlogs.Info("processor metrics client initialized")
	} else {
		log.Warn("redis addr is empty in processor, metrics will not be collected")
	}

	activityGateway, err := cache.NewActivityCache(activityClient, redisClient, cacheConfigFromProc(procCfg.Cache), log)
	if err != nil {
		return fmt.Errorf("create activity cache: %w", err)
	}
	defer logClose(log, "activity cache", activityGateway.Close)

	commonlogs.Info("processor metrics client initialized")

	// 根据配置创建消息队列实现（NATS/Redis/Memory）
	// NATS: 支持持久化和集群
	// Redis: 使用 Stream 或 Sorted Set 实现队列
	// Memory: 内存队列，仅用于测试
	var messageConsumer application.MessageConsumer
	var natsQueue *queue.NATSMessageQueue
	var natsPaymentTimeoutQueue *queue.NATSPaymentTimeoutQueue
	var natsPostPayQueue *queue.NATSPostPayQueue
	var seckillConsumer *queue.RedisMessageQueue
	var memoryQueue *queue.MemoryMessageQueue
	var paymentTimeoutQueue application.PaymentTimeoutPublisher
	var paymentTimeoutConsumer application.PaymentTimeoutConsumer
	var postPayQueue application.PostPayTaskConsumer
	var traceResults application.TraceResultStore
	var processorStore application.ProcessorStore

	if redisClient != nil {
		traceResults = traceresult.NewRedisStore(redisClient)
		processorStore = traceresult.NewProcessorStore(redisClient)
	}

	switch strings.ToLower(procCfg.QueueType) {
	case "nats":
		natsQueue, err = queue.NewNATSMessageQueue(procCfg.NATS, log)
		if err != nil {
			return fmt.Errorf("create nats seckill queue: %w", err)
		}
		defer logClose(log, "nats seckill queue", natsQueue.Close)
		messageConsumer = natsQueue
		natsPaymentTimeoutQueue, err = queue.NewNATSPaymentTimeoutQueue(procCfg.NATS, log)
		if err != nil {
			return fmt.Errorf("create nats payment timeout queue: %w", err)
		}
		defer logClose(log, "nats payment timeout queue", natsPaymentTimeoutQueue.Close)
		paymentTimeoutQueue = natsPaymentTimeoutQueue
		paymentTimeoutConsumer = natsPaymentTimeoutQueue
		natsPostPayQueue, err = queue.NewNATSPostPayQueue(procCfg.NATS, log)
		if err != nil {
			return fmt.Errorf("create nats post-pay queue: %w", err)
		}
		defer logClose(log, "nats post-pay queue", natsPostPayQueue.Close)
		postPayQueue = natsPostPayQueue
	case "redis":
		if redisClient == nil {
			return fmt.Errorf("redis queue requested but redis addr is empty")
		}
		seckillConsumer = queue.NewRedisMessageQueue(redisClient, log)
		messageConsumer = seckillConsumer
		redisPaymentTimeoutQueue := queue.NewRedisPaymentTimeoutQueue(redisClient, log)
		paymentTimeoutQueue = redisPaymentTimeoutQueue
		paymentTimeoutConsumer = redisPaymentTimeoutQueue
		postPayQueue = queue.NewRedisPostPayQueue(redisClient, log)
	case "memory", "":
		memoryQueue = queue.NewMemoryMessageQueue(procCfg.QueueSize, log)
		defer logClose(log, "memory queue", memoryQueue.Close)
		messageConsumer = memoryQueue
	default:
		return fmt.Errorf("unsupported processor queue type: %s", procCfg.QueueType)
	}

	// 创建应用服务。
	bus := eventbus.NewEventBus()
	seckillService := domainservice.NewSeckillService(
		activityGateway, stockClient, riskClient, orderClient, bus,
		identity.SnowflakeIDGenerator{},
		identity.RPCTemporaryChecker{},
		log,
	)
	var seckillOpts []application.SeckillAppOption
	if paymentTimeoutQueue != nil {
		seckillOpts = append(seckillOpts, application.WithPaymentTimeouts(paymentTimeoutQueue, procCfg.PaymentTimeoutDelay))
	}
	if traceResults != nil {
		seckillOpts = append(seckillOpts, application.WithTraceResults(traceResults))
	}
	if processorStore != nil {
		seckillOpts = append(seckillOpts, application.WithProcessorStore(processorStore))
	}

	// 创建 Use Case
	submitSeckillUC := usecase.NewSubmitSeckill(seckillService, processorStore, log)
	handlePaymentTimeoutUC := usecase.NewHandlePaymentTimeout(orderClient, stockClient, paymentClient, log)
	handlePostPayUC := usecase.NewHandlePostPay(freeCardClient, orderSyncClient, log)

	seckillApp := application.NewSeckillApp(
		submitSeckillUC,
		bus,
		messageConsumer,
		log,
		seckillOpts...,
	)
	if err := seckillApp.RegisterHandlers(); err != nil {
		return fmt.Errorf("register seckill event handlers: %w", err)
	}

	postPayProcessor := application.NewPostPayProcessor(handlePostPayUC, postPayQueue, log)

	paymentTimeoutProcessor := application.NewPaymentTimeoutProcessor(
		handlePaymentTimeoutUC,
		paymentTimeoutConsumer,
		log,
	)

	instanceID := instanceID()

	// 启动健康检查 HTTP 服务，供 K8s/Docker 探活
	var checkers []healthChecker
	if natsQueue != nil {
		checkers = append(checkers, natsQueue)
	}
	if natsPaymentTimeoutQueue != nil {
		checkers = append(checkers, natsPaymentTimeoutQueue)
	}
	if natsPostPayQueue != nil {
		checkers = append(checkers, natsPostPayQueue)
	}
	startHealthServer(log, checkers...)

	commonlogs.Infof("seckill-processor started queueType=%s consumerGroup=%s paymentTimeoutDelay=%s instanceID=%s",
		procCfg.QueueType, procCfg.ConsumerGroup, procCfg.PaymentTimeoutDelay.String(), instanceID)

	errCh := make(chan error, 3)

	go func() {
		errCh <- seckillApp.Run(ctx, procCfg.ConsumerGroup, cfg.ServiceName+"-"+instanceID)
	}()
	go func() {
		errCh <- postPayProcessor.Run(ctx, cfg.ServiceName+"-postpay-"+instanceID)
	}()
	go func() {
		errCh <- paymentTimeoutProcessor.Run(ctx, cfg.ServiceName+"-payment-timeout-"+instanceID)
	}()

	select {
	case err := <-errCh:
		if err != nil && ctx.Err() == nil {
			cancel()
			return fmt.Errorf("processor run: %w", err)
		}
		cancel()
	case <-ctx.Done():
	}
	return nil
}

// logClose 安全地关闭资源并记录错误
// 参数：
//   - log: 日志记录器
//   - name: 资源名称，用于日志
//   - close: 关闭函数
func logClose(log any, name string, close func() error) {
	if close == nil {
		return
	}
	if err := close(); err != nil {
		commonlogs.Warnf("close failed resource=%s err=%v", name, err)
	}
}

// dialService 连接 gRPC 服务
// 使用服务发现和熔断配置创建连接
// 参数：
//   - ctx: 上下文
//   - disc: 服务发现
//   - cfg: RPC 配置
//   - serviceName: 服务名称（用于日志）
//   - endpoint: 服务端点地址
//
// 返回 gRPC 客户端连接
func dialService(ctx context.Context, disc registry.Discovery, cfg processorconfig.RPCConfig, serviceName string, endpoint string) (*grpc.ClientConn, error) {
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

// healthChecker 健康检查接口，由 NATS 队列实现。
type healthChecker interface {
	IsHealthy() bool
}

// startHealthServer 启动一个轻量 HTTP 健康检查端点 :8081，
// 供 Docker healthcheck / K8s liveness probe 使用。
// 检查所有 NATS 队列的连接状态，任一不健康返回 503。
func startHealthServer(_ any, checkers ...healthChecker) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		for _, c := range checkers {
			if c != nil && !c.IsHealthy() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	go func() {
		if err := http.ListenAndServe(":8081", mux); err != nil {
			commonlogs.Warnf("health server stopped err=%v", err)
		}
	}()
}

// instanceID 返回当前实例标识，优先使用 hostname 的后 8 位，
// 在 Docker 里 hostname 即容器 ID，天然唯一。
func instanceID() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	if len(host) > 8 {
		host = host[len(host)-8:]
	}
	return host
}

func cacheConfigFromProc(c processorconfig.CacheConfig) cache.Config {
	return cache.Config{
		Detail: cache.EntryConfig{
			LocalTTL:   c.Detail.LocalTTL,
			Shards:     c.Detail.Shards,
			MaxEntries: c.Detail.MaxEntries,
		},
		SKU: cache.EntryConfig{
			LocalTTL:   c.SKU.LocalTTL,
			Shards:     c.SKU.Shards,
			MaxEntries: c.SKU.MaxEntries,
		},
	}
}
