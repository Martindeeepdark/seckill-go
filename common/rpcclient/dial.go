// Package rpcclient 提供 RPC 客户端连接管理
package rpcclient

import (
	"context"
	"fmt"
	"time"

	aegiscb "github.com/go-kratos/aegis/circuitbreaker"
	"github.com/go-kratos/aegis/circuitbreaker/sre"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/circuitbreaker"
	"github.com/go-kratos/kratos/v2/registry"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"seckill-common/config"
	"seckill-common/interceptor"
)

const defaultTimeout = 3 * time.Second // 默认 RPC 超时时间

// ConnectionPoolConfig 保存 gRPC 连接池调优参数。
// 零值表示未设置，调用 ApplyDefaults 后使用合理的默认值。
type ConnectionPoolConfig struct {
	InitialWindowSize     int32         // 流级初始窗口大小（字节），默认 256KB
	InitialConnWindowSize int32         // 连接级初始窗口大小（字节），默认 1MB
	MaxRecvMsgSize        int           // 最大接收消息大小（字节），默认 4MB
	KeepaliveTime         time.Duration // Keepalive 探测间隔，默认 30s
	KeepaliveTimeout      time.Duration // Keepalive 探测超时，默认 10s
}

// ApplyDefaults 为未设置的参数填充默认值，已设置的参数保持不变。
func (c ConnectionPoolConfig) ApplyDefaults() ConnectionPoolConfig {
	if c.InitialWindowSize <= 0 {
		c.InitialWindowSize = 262144 // 256KB
	}
	if c.InitialConnWindowSize <= 0 {
		c.InitialConnWindowSize = 1048576 // 1MB
	}
	if c.MaxRecvMsgSize <= 0 {
		c.MaxRecvMsgSize = 4194304 // 4MB
	}
	if c.KeepaliveTime <= 0 {
		c.KeepaliveTime = 30 * time.Second
	}
	if c.KeepaliveTimeout <= 0 {
		c.KeepaliveTimeout = 10 * time.Second
	}
	return c
}

// GRPCDialOptions 根据配置生成 gRPC 拨号选项。
// 先应用默认值，再生成选项列表。
func (c ConnectionPoolConfig) GRPCDialOptions() []grpc.DialOption {
	applied := c.ApplyDefaults()
	return []grpc.DialOption{
		grpc.WithInitialWindowSize(applied.InitialWindowSize),
		grpc.WithInitialConnWindowSize(applied.InitialConnWindowSize),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(applied.MaxRecvMsgSize)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    applied.KeepaliveTime,
			Timeout: applied.KeepaliveTimeout,
		}),
	}
}

// ConnectionPoolConfigFromMap 从配置映射中读取 gRPC 连接池调优参数。
func ConnectionPoolConfigFromMap(raw map[string]interface{}, prefix string) ConnectionPoolConfig {
	return ConnectionPoolConfig{
		InitialWindowSize:     int32(config.GetInt(raw, prefix+".initial_window_size")),
		InitialConnWindowSize: int32(config.GetInt(raw, prefix+".initial_conn_window_size")),
		MaxRecvMsgSize:        config.GetInt(raw, prefix+".max_recv_msg_size"),
		KeepaliveTime:         config.GetDuration(raw, prefix+".keepalive_time", 0),
		KeepaliveTimeout:      config.GetDuration(raw, prefix+".keepalive_timeout", 0),
	}
}

// Config RPC 客户端配置
type Config struct {
	Endpoint             string               // 服务端点地址
	Timeout              time.Duration        // 请求超时时间
	CircuitBreaker       bool                 // 是否启用熔断器
	CircuitBreakerPolicy CircuitBreakerPolicy // 熔断器策略
	Discovery            registry.Discovery   // 服务发现实例
	Pool                 ConnectionPoolConfig  // 连接池调优参数
}

// CircuitBreakerPolicy 是 gRPC client 的 SRE 熔断策略配置。
type CircuitBreakerPolicy struct {
	Success float64       // 成功率阈值，0 表示使用 Kratos 默认值
	Request int64         // 触发熔断计算的最小请求数，0 表示默认值
	Window  time.Duration // 统计窗口，0 表示默认值
	Bucket  int           // 窗口桶数量，0 表示默认值
}

// Endpoint 构造服务端点地址
// 优先使用配置的地址，其次使用服务发现，最后使用回退地址
func Endpoint(serviceName string, configured string, fallback string) string {
	if configured != "" {
		return configured
	}
	if serviceName != "" {
		return "discovery:///" + serviceName
	}
	return fallback
}

// Dial 创建 gRPC 客户端连接
// 配置超时、拦截器、熔断器和服务发现
func Dial(ctx context.Context, cfg Config) (*grpc.ClientConn, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("rpc endpoint is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	options := []kratosgrpc.ClientOption{
		kratosgrpc.WithEndpoint(cfg.Endpoint),
		kratosgrpc.WithTimeout(timeout),
		kratosgrpc.WithUnaryInterceptor(interceptor.TraceUnaryClientInterceptor()),
		kratosgrpc.WithOptions(cfg.Pool.GRPCDialOptions()...),
	}
	if cfg.CircuitBreaker {
		options = append(options, kratosgrpc.WithMiddleware(newCircuitBreakerMiddleware(cfg.CircuitBreakerPolicy)))
	}
	if cfg.Discovery != nil {
		options = append(options, kratosgrpc.WithDiscovery(cfg.Discovery), kratosgrpc.WithPrintDiscoveryDebugLog(false))
	}
	conn, err := kratosgrpc.DialInsecure(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("dial grpc insecure: %w", err)
	}
	return conn, nil
}

func newCircuitBreakerMiddleware(policy CircuitBreakerPolicy) middleware.Middleware {
	if policy.isZero() {
		return circuitbreaker.Client()
	}
	return circuitbreaker.Client(circuitbreaker.WithCircuitBreaker(func() aegiscb.CircuitBreaker {
		return sre.NewBreaker(policy.sreOptions()...)
	}))
}

func (p CircuitBreakerPolicy) isZero() bool {
	return p.Success <= 0 && p.Request <= 0 && p.Window <= 0 && p.Bucket <= 0
}

func (p CircuitBreakerPolicy) sreOptions() []sre.Option {
	opts := make([]sre.Option, 0, 4)
	if p.Success > 0 {
		opts = append(opts, sre.WithSuccess(p.Success))
	}
	if p.Request > 0 {
		opts = append(opts, sre.WithRequest(p.Request))
	}
	if p.Window > 0 {
		opts = append(opts, sre.WithWindow(p.Window))
	}
	if p.Bucket > 0 {
		opts = append(opts, sre.WithBucket(p.Bucket))
	}
	return opts
}
