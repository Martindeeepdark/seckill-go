// Package main 提供 risk-service 的服务入口
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	commonfile "github.com/Martindeeepdark/go-common/config"
	"google.golang.org/grpc"

	commonconfig "seckill-common/config"
	"seckill-common/discovery"
	"seckill-common/interceptor"
	"seckill-common/logger"
	commserver "seckill-common/server"
	"seckill-common/tracing"
	riskapp "seckill-risk-service/internal/application/risk"

	"seckill-risk-service/internal/config"
	"seckill-risk-service/internal/infrastructure"
	"seckill-risk-service/internal/infrastructure/cache"
	"seckill-risk-service/internal/infrastructure/rpc"
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

// start 启动风控服务
func start(configPath string) error {
	// 加载配置（含缓存参数）
	cfg, cacheCfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	// 加载风控配置
	riskCfg, err := loadRiskConfig(configPath)
	if err != nil {
		return err
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
	// 创建存储层
	repo := infrastructure.NewStore(context.Background(), cfg, log)
	// 包装本地缓存层（BigCache local-only），加速 IsRiskUser 热点查询
	cachedRepo, err := cache.NewRiskCache(repo, cacheConfigFromRisk(cacheCfg), log)
	if err != nil {
		return fmt.Errorf("create risk cache: %w", err)
	}
	defer func() { _ = cachedRepo.Close() }() //nolint:errcheck // best-effort cleanup
	// 创建风控评估器
	evaluator := &riskapp.Evaluator{
		Repo:   cachedRepo,
		Config: riskCfg.BlackList,
		Risk:   riskCfg.Risk,
	}
	// 注册服务到发现中心
	deregister, err := discovery.RegisterGRPCService(context.Background(), cfg, log)
	if err != nil {
		return fmt.Errorf("register grpc service: %w", err)
	}
	defer func() { _ = deregister(context.Background()) }() //nolint:errcheck // best-effort cleanup

	// 创建并启动gRPC服务器
	srv := commserver.NewGRPCServer(cfg.GRPCAddr, log, interceptor.TraceUnaryServerInterceptor())
	srv.Register(func(registrar grpc.ServiceRegistrar) {
		rpc.RegisterRiskPBServer(registrar, rpc.NewRiskPBService(evaluator))
	})
	return fmt.Errorf("start grpc server: %w", srv.Start())
}

func cacheConfigFromRisk(c config.CacheConfig) cache.Config {
	return cache.Config{
		Risk: cache.EntryConfig{
			LocalTTL:   c.Risk.LocalTTL,
			Shards:     c.Risk.Shards,
			MaxEntries: c.Risk.MaxEntries,
		},
	}
}

// riskRuntimeConfig 风控服务运行时配置
type riskRuntimeConfig struct {
	BlackList riskapp.BlackListConfig
	Risk      riskapp.RiskConfig
}

// loadRiskConfig 加载风控配置
func loadRiskConfig(path string) (riskRuntimeConfig, error) {
	raw := map[string]interface{}{
		"black_list": map[string]interface{}{
			"enabled":           true,
			"mark_start_before": "300s",
			"mark_end_before":   "10s",
			"expire_after":      "300s",
		},
		"risk": map[string]interface{}{
			"high_risk_threshold": 10,
			"risk_user_ttl":       "24h",
			"recent_window":       "1h",
			"high_risk_window":    "24h",
		},
	}
	loaded, err := commonfile.Load(path)
	if err != nil {
		return riskRuntimeConfig{}, fmt.Errorf("load risk runtime config %s: %w", path, err)
	}
	commonconfig.MergeMap(raw, loaded.ToMap())
	return riskRuntimeConfig{
		BlackList: riskapp.BlackListConfig{
			Enabled:         commonconfig.GetBool(raw, "black_list.enabled"),
			MarkStartBefore: commonconfig.GetDuration(raw, "black_list.mark_start_before", 300*time.Second),
			MarkEndBefore:   commonconfig.GetDuration(raw, "black_list.mark_end_before", 10*time.Second),
			ExpireAfter:     commonconfig.GetDuration(raw, "black_list.expire_after", 300*time.Second),
		},
		Risk: riskapp.RiskConfig{
			HighRiskThreshold: commonconfig.GetInt(raw, "risk.high_risk_threshold"),
			RiskUserTTL:       commonconfig.GetDuration(raw, "risk.risk_user_ttl", 24*time.Hour),
			RecentWindow:      commonconfig.GetDuration(raw, "risk.recent_window", time.Hour),
			HighRiskWindow:    commonconfig.GetDuration(raw, "risk.high_risk_window", 24*time.Hour),
		},
	}, nil
}
