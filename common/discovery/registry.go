package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/registry"

	commonredis "github.com/Martindeeepdark/go-common/cache/redis"
	goredis "github.com/redis/go-redis/v9"

	"seckill-common/config"
)

// Option Redis 注册中心配置选项
type Option func(*RedisRegistry)

// WithPollInterval 设置服务发现的轮询间隔
func WithPollInterval(d time.Duration) Option {
	return func(r *RedisRegistry) { r.poll = d }
}

// WithFallback 设置服务发现的回退策略
func WithFallback(d registry.Discovery) Option {
	return func(r *RedisRegistry) { r.fallback = d }
}

// RedisRegistry 基于 Redis 的服务注册与发现
type RedisRegistry struct {
	cache     *commonredis.Client   // Redis 客户端
	commands  redisRegistryCommands // Redis 命令接口
	namespace string                // 命名空间
	ttl       time.Duration         // 服务注册的 TTL
	poll      time.Duration         // 轮询间隔
	fallback  registry.Discovery    // 回退发现策略
}

// redisRegistryCommands Redis 命令接口
// 用于测试时 mock Redis 操作
type redisRegistryCommands interface {
	HSet(ctx context.Context, key string, values ...interface{}) *goredis.IntCmd
	HDel(ctx context.Context, key string, fields ...string) *goredis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *goredis.BoolCmd
	HGetAll(ctx context.Context, key string) *goredis.MapStringStringCmd
}

// NewRedisRegistry 创建 Redis 服务注册中心
// 参数：
//   - addr: Redis 服务器地址
//   - password: Redis 密码
//   - db: Redis 数据库编号
//   - namespace: 服务命名空间
//   - ttl: 服务注册的 TTL
//   - opts: 配置选项
func NewRedisRegistry(ctx context.Context, addr, password string, db int, namespace string, ttl time.Duration, opts ...Option) (*RedisRegistry, error) {
	cache := commonredis.New(addr, commonredis.WithPassword(password), commonredis.WithDB(db))
	if err := cache.Redis().Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	r := &RedisRegistry{cache: cache, commands: cache.Redis(), namespace: namespace, ttl: ttl}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// Register 注册服务实例
// 将服务地址写入 Redis，并设置 TTL
func (r *RedisRegistry) Register(ctx context.Context, instance *registry.ServiceInstance) error {
	endpoint, err := instanceEndpoint(instance)
	if err != nil {
		return err
	}
	key := instanceKey(r.namespace, instance.Name, instance.Version)
	if err := r.commands.HSet(ctx, key, endpoint, endpoint).Err(); err != nil {
		return fmt.Errorf("redis hset service instance: %w", err)
	}
	if r.ttl > 0 {
		if err := r.commands.Expire(ctx, key, r.ttl).Err(); err != nil {
			return fmt.Errorf("redis expire service instance key: %w", err)
		}
	}
	return nil
}

// Deregister 注销服务实例
// 从 Redis 中移除服务地址
func (r *RedisRegistry) Deregister(ctx context.Context, instance *registry.ServiceInstance) error {
	endpoint, err := instanceEndpoint(instance)
	if err != nil {
		return err
	}
	key := instanceKey(r.namespace, instance.Name, instance.Version)
	if err := r.commands.HDel(ctx, key, endpoint).Err(); err != nil {
		return fmt.Errorf("redis hdel service instance: %w", err)
	}
	return nil
}

// GetService 获取指定服务的实例列表
// 从 Redis 查询服务地址，失败时回退到静态配置
func (r *RedisRegistry) GetService(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	key := instanceKey(r.namespace, serviceName, "v1")
	addrs, err := r.commands.HGetAll(ctx, key).Result()
	if err != nil {
		// Redis 查询失败，尝试回退策略
		if r.fallback != nil {
			instances, fallbackErr := r.fallback.GetService(ctx, serviceName)
			if fallbackErr != nil {
				return nil, fmt.Errorf("fallback get service %s: %w", serviceName, fallbackErr)
			}
			return instances, nil
		}
		return nil, fmt.Errorf("redis hgetall: %w", err)
	}
	// 无可用实例，尝试回退策略
	if len(addrs) == 0 && r.fallback != nil {
		instances, fallbackErr := r.fallback.GetService(ctx, serviceName)
		if fallbackErr != nil {
			return nil, fmt.Errorf("fallback get service %s: %w", serviceName, fallbackErr)
		}
		return instances, nil
	}
	endpoints := make([]string, 0, len(addrs))
	for addr := range addrs {
		if strings.TrimSpace(addr) != "" {
			endpoints = append(endpoints, addr)
		}
	}
	sort.Strings(endpoints)
	instances := make([]*registry.ServiceInstance, 0, len(endpoints))
	for _, addr := range endpoints {
		instances = append(instances, &registry.ServiceInstance{
			Name: serviceName, Version: "v1",
			Endpoints: []string{addr},
			Metadata:  map[string]string{"grpcAddr": addr},
		})
	}
	return instances, nil
}

// Watch 监听服务变更
// 返回一个 watcher，定期轮询服务变更
func (r *RedisRegistry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if strings.TrimSpace(serviceName) == "" {
		return nil, fmt.Errorf("service name is required")
	}
	watchCtx, cancel := context.WithCancel(ctx)
	return &redisWatcher{
		ctx:         watchCtx,
		cancel:      cancel,
		registry:    r,
		serviceName: serviceName,
		interval:    watchPollInterval(r.poll),
	}, nil
}

// Close 关闭 Redis 连接
func (r *RedisRegistry) Close() error {
	if r.cache != nil {
		if err := r.cache.Close(); err != nil {
			return fmt.Errorf("close redis cache: %w", err)
		}
	}
	return nil
}

// RegisterGRPCService 注册 gRPC 服务到服务发现
// 支持 redis 和 etcd 两种模式
// 返回清理函数，用于服务关闭时注销
func RegisterGRPCService(ctx context.Context, cfg config.Config, logger *slog.Logger) (func(context.Context) error, error) {
	// etcd 模式：委托给 etcd 注册
	if strings.EqualFold(cfg.DiscoveryMode, "etcd") {
		return RegisterGRPCServiceEtcd(ctx, cfg, logger)
	}
	// redis 模式
	noop := func(context.Context) error { return nil }
	if cfg.GRPCAddr == "" || !strings.EqualFold(cfg.DiscoveryMode, "redis") {
		return noop, nil
	}
	if cfg.RedisAddr == "" {
		if cfg.DiscoveryStaticFallback {
			if logger != nil {
				logger.Warn("redis registry addr empty, grpc service will rely on explicit static discovery fallback")
			}
			return noop, nil
		}
		return nil, fmt.Errorf("redis registry enabled but data.redis.addr is empty")
	}
	redisReg, err := NewRedisRegistry(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.DiscoveryNS, cfg.DiscoveryTTL, WithPollInterval(cfg.DiscoveryTick))
	if err != nil {
		if cfg.DiscoveryStaticFallback {
			if logger != nil {
				logger.Warn("redis registry unavailable, grpc service will rely on explicit static discovery fallback", "addr", cfg.RedisAddr, "error", err)
			}
			return noop, nil
		}
		return nil, fmt.Errorf("redis registry unavailable: %w", err)
	}

	addr := AdvertisedGRPCAddr(cfg)
	instance := &registry.ServiceInstance{
		Name: cfg.ServiceName, Version: "v1",
		Endpoints: []string{NormalizeGRPCEndpoint(addr)},
		Metadata:  map[string]string{"grpcAddr": addr},
	}
	if err := redisReg.Register(ctx, instance); err != nil {
		if cfg.DiscoveryStaticFallback {
			if logger != nil {
				logger.Warn("grpc service registry failed, relying on explicit static discovery fallback", "service", cfg.ServiceName, "addr", addr, "error", err)
			}
			_ = redisReg.Close() //nolint:errcheck // best-effort cleanup
			return noop, nil
		}
		_ = redisReg.Close() //nolint:errcheck // best-effort cleanup
		return nil, fmt.Errorf("grpc service registry failed: %w", err)
	}
	if logger != nil {
		logger.Info("grpc service registered", "service", cfg.ServiceName, "addr", addr, "ttl", cfg.DiscoveryTTL.String())
	}

	// 启动后台协程，定期刷新注册
	keepAliveCtx, cancel := context.WithCancel(context.Background())
	go func() {
		interval := RefreshInterval(cfg.DiscoveryTTL, cfg.DiscoveryTick)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-keepAliveCtx.Done():
				return
			case <-ticker.C:
				if err := redisReg.Register(keepAliveCtx, instance); err != nil && logger != nil {
					logger.Warn("grpc service registry refresh failed", "service", cfg.ServiceName, "addr", addr, "error", err)
				}
			}
		}
	}()

	// 返回清理函数
	return func(ctx context.Context) error {
		cancel()
		defer func() { _ = redisReg.Close() }() //nolint:errcheck // best-effort cleanup
		return redisReg.Deregister(ctx, instance)
	}, nil
}

// NewRPCDiscovery 创建 RPC 服务发现实例
// 根据配置选择 etcd、Redis 或静态服务发现
func NewRPCDiscovery(ctx context.Context, cfg config.Config, logger *slog.Logger) (registry.Discovery, error) {
	// etcd 模式：委托给 etcd 发现
	if strings.EqualFold(cfg.DiscoveryMode, "etcd") {
		return NewEtcdDiscovery(ctx, cfg, logger)
	}
	// redis / static 模式
	static := NewStaticDiscovery(cfg.Discovery)
	if !strings.EqualFold(cfg.DiscoveryMode, "redis") || cfg.RedisAddr == "" {
		if strings.EqualFold(cfg.DiscoveryMode, "redis") && cfg.RedisAddr == "" && !cfg.DiscoveryStaticFallback {
			return nil, fmt.Errorf("redis discovery enabled but data.redis.addr is empty")
		}
		if strings.EqualFold(cfg.DiscoveryMode, "redis") && cfg.DiscoveryStaticFallback && logger != nil {
			logger.Warn("redis discovery addr empty, using explicit static fallback")
		}
		return static, nil
	}
	redisDisc, err := NewRedisRegistry(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.DiscoveryNS, cfg.DiscoveryTTL, WithFallback(static), WithPollInterval(cfg.DiscoveryTick))
	if err != nil {
		if !cfg.DiscoveryStaticFallback {
			return nil, fmt.Errorf("redis discovery unavailable: %w", err)
		}
		if logger != nil {
			logger.Warn("redis discovery unavailable, using explicit static fallback", "addr", cfg.RedisAddr, "error", err)
		}
		return static, nil
	}
	if logger != nil {
		logger.Info("redis discovery enabled", "namespace", cfg.DiscoveryNS, "ttl", cfg.DiscoveryTTL.String())
	}
	return redisDisc, nil
}

// AdvertisedGRPCAddr 获取服务对外公告的 gRPC 地址
// 优先使用 advertise 配置，回退到 discovery 配置，最后使用 grpc.addr
func AdvertisedGRPCAddr(cfg config.Config) string {
	if addr := strings.TrimSpace(cfg.AdvertiseAddr[cfg.ServiceName]); addr != "" {
		return addr
	}
	if addrs := cfg.Discovery[cfg.ServiceName]; len(addrs) > 0 {
		return addrs[0]
	}
	return cfg.GRPCAddr
}

// NormalizeGRPCEndpoint 规范化 gRPC 端点地址
// 确保地址带有 "grpc://" 前缀
func NormalizeGRPCEndpoint(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, "://") {
		return addr
	}
	return "grpc://" + addr
}

// RefreshInterval 计算服务注册刷新间隔
// 使用 TTL 的 1/3 或配置的间隔，取较小值
func RefreshInterval(ttl, configured time.Duration) time.Duration {
	if ttl <= 0 {
		if configured > 0 {
			return configured
		}
		return 5 * time.Second
	}
	if configured > 0 && configured < ttl {
		return configured
	}
	interval := ttl / 3
	if interval <= 0 {
		return time.Second
	}
	return interval
}

// instanceKey 生成服务实例的 Redis key
func instanceKey(namespace, serviceName, version string) string {
	return fmt.Sprintf("discovery:%s:%s:%s", namespace, serviceName, version)
}

// instanceEndpoint 提取服务实例的端点地址
func instanceEndpoint(instance *registry.ServiceInstance) (string, error) {
	if instance == nil {
		return "", fmt.Errorf("service instance is required")
	}
	if strings.TrimSpace(instance.Name) == "" {
		return "", fmt.Errorf("service instance name is required")
	}
	if len(instance.Endpoints) == 0 || strings.TrimSpace(instance.Endpoints[0]) == "" {
		return "", fmt.Errorf("service instance endpoint is required")
	}
	return strings.TrimSpace(instance.Endpoints[0]), nil
}

// redisWatcher Redis 服务 watcher
// 定期轮询服务变更
type redisWatcher struct {
	ctx         context.Context
	cancel      context.CancelFunc
	registry    *RedisRegistry
	serviceName string
	interval    time.Duration
	last        string // 上次的服务签名
	seen        bool   // 是否已首次返回
}

// Next 等待服务变更
// 轮询服务实例，仅在检测到变更时返回
func (w *redisWatcher) Next() ([]*registry.ServiceInstance, error) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		instances, err := w.registry.GetService(w.ctx, w.serviceName)
		if err != nil {
			return nil, err
		}
		signature := serviceSignature(instances)
		if w.shouldReturn(instances, signature) {
			return instances, nil
		}
		select {
		case <-w.ctx.Done():
			return nil, fmt.Errorf("watch service %s: %w", w.serviceName, w.ctx.Err())
		case <-ticker.C:
		}
	}
}

// Stop 停止 watcher
func (w *redisWatcher) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}

// shouldReturn 判断是否应该返回当前实例列表
// 首次有实例时返回，之后仅在签名变更时返回
func (w *redisWatcher) shouldReturn(instances []*registry.ServiceInstance, signature string) bool {
	if !w.seen {
		if len(instances) == 0 {
			return false
		}
		w.seen = true
		w.last = signature
		return true
	}
	if signature == w.last {
		return false
	}
	w.last = signature
	return true
}

// watchPollInterval 计算 watcher 轮询间隔
func watchPollInterval(configured time.Duration) time.Duration {
	if configured > 0 {
		return configured
	}
	return 5 * time.Second
}

// serviceSignature 生成服务实例签名
// 用于检测服务变更
func serviceSignature(instances []*registry.ServiceInstance) string {
	if len(instances) == 0 {
		return ""
	}
	endpoints := make([]string, 0, len(instances))
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		for _, endpoint := range instance.Endpoints {
			if strings.TrimSpace(endpoint) != "" {
				endpoints = append(endpoints, endpoint)
			}
		}
	}
	sort.Strings(endpoints)
	return strings.Join(endpoints, "\n")
}

// 编译时类型检查
var _ redisRegistryCommands = (*goredis.Client)(nil)
