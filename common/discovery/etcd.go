package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/registry"
	clientv3 "go.etcd.io/etcd/client/v3"

	"seckill-common/config"
)

// EtcdOption etcd 注册中心配置选项
type EtcdOption func(*EtcdRegistry)

// EtcdWithFallback 设置服务发现的回退策略
func EtcdWithFallback(d registry.Discovery) EtcdOption {
	return func(r *EtcdRegistry) { r.fallback = d }
}

// EtcdWithDialTimeout 设置 etcd 客户端连接超时
func EtcdWithDialTimeout(d time.Duration) EtcdOption {
	return func(r *EtcdRegistry) { r.dialTimeout = d }
}

// EtcdWithPrefix 设置服务注册的 key 前缀
func EtcdWithPrefix(prefix string) EtcdOption {
	return func(r *EtcdRegistry) { r.prefix = prefix }
}

// EtcdRegistry 基于 etcd 的服务注册与发现
type EtcdRegistry struct {
	client      *clientv3.Client
	fallback    registry.Discovery
	prefix      string
	dialTimeout time.Duration
}

// NewEtcdRegistry 创建 etcd 服务注册中心
func NewEtcdRegistry(ctx context.Context, endpoints []string, opts ...EtcdOption) (*EtcdRegistry, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("etcd endpoints is required")
	}
	r := &EtcdRegistry{
		prefix:      "/microservices",
		dialTimeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(r)
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: r.dialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd client connect: %w", err)
	}
	if err := client.Sync(ctx); err != nil {
		// Sync 失败不阻塞，后续操作仍可重试
		_ = client.Close()
		return nil, fmt.Errorf("etcd client sync: %w", err)
	}
	r.client = client
	return r, nil
}

// serviceKey 生成服务实例的 etcd key
func (r *EtcdRegistry) serviceKey(serviceName, version string) string {
	return fmt.Sprintf("%s/%s/%s", r.prefix, serviceName, version)
}

// Register 注册服务实例到 etcd
func (r *EtcdRegistry) Register(ctx context.Context, instance *registry.ServiceInstance) error {
	if r.client == nil {
		return fmt.Errorf("etcd client is nil")
	}
	key := r.serviceKey(instance.Name, instance.Version)
	val := instanceToValue(instance)
	ttl := int64(15)
	if lease, err := r.client.Grant(ctx, ttl); err == nil {
		_, err = r.client.Put(ctx, key+"/"+instance.Endpoints[0], val, clientv3.WithLease(lease.ID))
		if err != nil {
			return fmt.Errorf("etcd put service instance: %w", err)
		}
		// 保持租约续期
		ch, err := r.client.KeepAlive(ctx, lease.ID)
		if err == nil {
			go func() {
				for range ch {
				}
			}()
		}
	} else {
		// 回退到不带 TTL 的 Put
		_, err = r.client.Put(ctx, key+"/"+instance.Endpoints[0], val)
		if err != nil {
			return fmt.Errorf("etcd put service instance: %w", err)
		}
	}
	return nil
}

// Deregister 从 etcd 注销服务实例
func (r *EtcdRegistry) Deregister(ctx context.Context, instance *registry.ServiceInstance) error {
	if r.client == nil {
		return fmt.Errorf("etcd client is nil")
	}
	key := r.serviceKey(instance.Name, instance.Version)
	_, err := r.client.Delete(ctx, key+"/"+instance.Endpoints[0])
	if err != nil {
		return fmt.Errorf("etcd delete service instance: %w", err)
	}
	return nil
}

// GetService 获取指定服务的实例列表
func (r *EtcdRegistry) GetService(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if r.client == nil {
		if r.fallback != nil {
			return r.fallback.GetService(ctx, serviceName)
		}
		return nil, fmt.Errorf("etcd client is nil")
	}
	key := r.serviceKey(serviceName, "v1")
	resp, err := r.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		if r.fallback != nil {
			instances, fbErr := r.fallback.GetService(ctx, serviceName)
			if fbErr != nil {
				return nil, fmt.Errorf("etcd get and fallback failed: %w, %w", err, fbErr)
			}
			return instances, nil
		}
		return nil, fmt.Errorf("etcd get service %s: %w", serviceName, err)
	}
	if len(resp.Kvs) == 0 && r.fallback != nil {
		return r.fallback.GetService(ctx, serviceName)
	}
	instances := make([]*registry.ServiceInstance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		instance := valueToInstance(serviceName, string(kv.Value))
		if instance != nil {
			instances = append(instances, instance)
		}
	}
	return instances, nil
}

// Watch 监听服务变更
func (r *EtcdRegistry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if r.client == nil {
		return nil, fmt.Errorf("etcd client is nil")
	}
	return newEtcdWatcher(ctx, r, serviceName)
}

// Close 关闭 etcd 客户端
func (r *EtcdRegistry) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Client 返回原始 etcd 客户端，供配置源等复用
func (r *EtcdRegistry) Client() *clientv3.Client {
	return r.client
}

// instanceToValue 将服务实例序列化为存储值
func instanceToValue(instance *registry.ServiceInstance) string {
	if len(instance.Endpoints) == 0 {
		return ""
	}
	return instance.Endpoints[0]
}

// valueToInstance 将存储值反序列化为服务实例
func valueToInstance(serviceName, val string) *registry.ServiceInstance {
	addr := strings.TrimSpace(val)
	if addr == "" {
		return nil
	}
	return &registry.ServiceInstance{
		Name:      serviceName,
		Version:   "v1",
		Endpoints: []string{addr},
		Metadata:  map[string]string{"grpcAddr": addr},
	}
}

// etcdWatcher etcd 服务 watcher
type etcdWatcher struct {
	ctx         context.Context
	cancel      context.CancelFunc
	registry    *EtcdRegistry
	serviceName string
	last        string
	seen        bool
}

func newEtcdWatcher(ctx context.Context, r *EtcdRegistry, serviceName string) (*etcdWatcher, error) {
	watchCtx, cancel := context.WithCancel(ctx)
	return &etcdWatcher{
		ctx:         watchCtx,
		cancel:      cancel,
		registry:    r,
		serviceName: serviceName,
	}, nil
}

func (w *etcdWatcher) Next() ([]*registry.ServiceInstance, error) {
	key := w.registry.serviceKey(w.serviceName, "v1")
	watcher := clientv3.NewWatcher(w.registry.client)
	defer watcher.Close()
	watchCh := watcher.Watch(w.ctx, key, clientv3.WithPrefix())
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
		case <-watchCh:
		}
	}
}

func (w *etcdWatcher) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}

func (w *etcdWatcher) shouldReturn(instances []*registry.ServiceInstance, signature string) bool {
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

// RegisterGRPCServiceEtcd 使用 etcd 注册 gRPC 服务
func RegisterGRPCServiceEtcd(ctx context.Context, cfg config.Config, logger *slog.Logger) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if cfg.GRPCAddr == "" || !strings.EqualFold(cfg.DiscoveryMode, "etcd") {
		return noop, nil
	}
	if len(cfg.EtcdEndpoints) == 0 {
		if cfg.DiscoveryStaticFallback {
			if logger != nil {
				logger.Warn("etcd endpoints empty, grpc service will rely on static fallback")
			}
			return noop, nil
		}
		return nil, fmt.Errorf("etcd discovery enabled but discovery.etcd.endpoints is empty")
	}
	static := NewStaticDiscovery(cfg.Discovery)
	etcdReg, err := NewEtcdRegistry(ctx, cfg.EtcdEndpoints,
		EtcdWithFallback(static),
		EtcdWithPrefix("/"+cfg.DiscoveryNS),
	)
	if err != nil {
		if cfg.DiscoveryStaticFallback {
			if logger != nil {
				logger.Warn("etcd registry unavailable, relying on static fallback", "endpoints", cfg.EtcdEndpoints, "error", err)
			}
			return noop, nil
		}
		return nil, fmt.Errorf("etcd registry unavailable: %w", err)
	}

	addr := AdvertisedGRPCAddr(cfg)
	instance := &registry.ServiceInstance{
		Name: cfg.ServiceName, Version: "v1",
		Endpoints: []string{NormalizeGRPCEndpoint(addr)},
		Metadata:  map[string]string{"grpcAddr": addr},
	}
	if err := etcdReg.Register(ctx, instance); err != nil {
		if cfg.DiscoveryStaticFallback {
			if logger != nil {
				logger.Warn("etcd service register failed, relying on static fallback", "service", cfg.ServiceName, "addr", addr, "error", err)
			}
			_ = etcdReg.Close()
			return noop, nil
		}
		_ = etcdReg.Close()
		return nil, fmt.Errorf("etcd service register failed: %w", err)
	}
	if logger != nil {
		logger.Info("grpc service registered via etcd", "service", cfg.ServiceName, "addr", addr, "endpoints", cfg.EtcdEndpoints)
	}

	return func(ctx context.Context) error {
		defer func() { _ = etcdReg.Close() }()
		return etcdReg.Deregister(ctx, instance)
	}, nil
}

// NewEtcdDiscovery 创建基于 etcd 的 RPC 服务发现实例
func NewEtcdDiscovery(ctx context.Context, cfg config.Config, logger *slog.Logger) (registry.Discovery, error) {
	static := NewStaticDiscovery(cfg.Discovery)
	if !strings.EqualFold(cfg.DiscoveryMode, "etcd") || len(cfg.EtcdEndpoints) == 0 {
		if strings.EqualFold(cfg.DiscoveryMode, "etcd") && len(cfg.EtcdEndpoints) == 0 && !cfg.DiscoveryStaticFallback {
			return nil, fmt.Errorf("etcd discovery enabled but discovery.etcd.endpoints is empty")
		}
		if strings.EqualFold(cfg.DiscoveryMode, "etcd") && len(cfg.EtcdEndpoints) == 0 && cfg.DiscoveryStaticFallback && logger != nil {
			logger.Warn("etcd endpoints empty, using static fallback")
		}
		return static, nil
	}
	etcdReg, err := NewEtcdRegistry(ctx, cfg.EtcdEndpoints,
		EtcdWithFallback(static),
		EtcdWithPrefix("/"+cfg.DiscoveryNS),
	)
	if err != nil {
		if !cfg.DiscoveryStaticFallback {
			return nil, fmt.Errorf("etcd discovery unavailable: %w", err)
		}
		if logger != nil {
			logger.Warn("etcd discovery unavailable, using static fallback", "endpoints", cfg.EtcdEndpoints, "error", err)
		}
		return static, nil
	}
	if logger != nil {
		logger.Info("etcd discovery enabled", "endpoints", cfg.EtcdEndpoints, "namespace", cfg.DiscoveryNS)
	}
	return etcdReg, nil
}
