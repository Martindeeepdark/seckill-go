// Package config 提供秒杀处理器的配置管理
// 支持从 YAML 文件加载配置，包括队列类型、NATS、RPC 等配置项
package config

import (
	"fmt"
	"time"

	commonconfig "github.com/Martindeeepdark/go-common/config"

	"seckill-common/config"
	"seckill-common/rpcclient"
)

// ProcessorConfig 秒杀处理器配置
type ProcessorConfig struct {
	ConsumerGroup       string        // 消费者组名称
	PaymentTimeoutDelay time.Duration // 支付超时延迟时间
	QueueType           string        // 队列类型："memory", "redis", 或 "nats"
	QueueSize           int           // 内存队列大小
	NATS                NATSConfig    // NATS 配置
	Cache               CacheConfig   // 本地缓存配置
}

// CacheConfig 控制 ActivityCache 的本地缓存参数
type CacheConfig struct {
	Detail CacheEntryConfig
	SKU    CacheEntryConfig
}

// CacheEntryConfig 描述单层 (detail 或 sku) 缓存的 BigCache 参数
type CacheEntryConfig struct {
	LocalTTL   time.Duration
	Shards     int
	MaxEntries int
}

// NATSConfig NATS 消息队列配置
type NATSConfig struct {
	URL                   string // NATS 服务器地址
	Stream                string // JetStream 流名称
	Subject               string // 秒杀消息主题
	PostPaySubject        string // 支付后任务主题
	PaymentTimeoutSubject string // 支付超时任务主题
}

// RPCConfig RPC 客户端配置
type RPCConfig struct {
	Timeout              time.Duration                  // 请求超时时间
	CircuitBreaker       bool                           // 是否启用熔断器
	CircuitBreakerPolicy rpcclient.CircuitBreakerPolicy // 熔断器策略
	ActivityAddr         string                         // 活动服务地址（直接连接时使用）
	StockAddr            string                         // 库存服务地址
	OrderAddr            string                         // 订单服务地址
	RiskAddr             string                         // 风控服务地址
	SupportAddr          string                         // 支持服务地址（支付、自由卡、订单同步）
}

// Load 从指定路径加载配置文件
// 返回通用配置、处理器配置、RPC 配置
// 如果加载失败，返回错误
func Load(path string) (config.Config, ProcessorConfig, RPCConfig, error) {
	raw := defaultProcessorMap()
	loaded, err := commonconfig.Load(path)
	if err != nil {
		return config.Config{}, ProcessorConfig{}, RPCConfig{}, fmt.Errorf("load config %s: %w", path, err)
	}
	config.MergeMap(raw, loaded.ToMap())
	cfg := config.FromMap(raw, "seckill-processor")
	return cfg, loadProcessorConfig(raw), loadRPCConfig(cfg, raw), nil
}

func defaultProcessorMap() map[string]interface{} {
	services := map[string]interface{}{
		"activity-service": []interface{}{"127.0.0.1:9001"},
		"stock-service":    []interface{}{"127.0.0.1:9002"},
		"risk-service":     []interface{}{"127.0.0.1:9003"},
		"order-service":    []interface{}{"127.0.0.1:9004"},
		"support-service":  []interface{}{"127.0.0.1:9005"},
	}
	advertise := map[string]interface{}{
		"seckill-processor": "",
	}
	return map[string]interface{}{
		"log":   map[string]interface{}{"level": "info"},
		"trace": map[string]interface{}{"enabled": false, "endpoint": "127.0.0.1:4317", "insecure": true},
		"server": map[string]interface{}{
			"grpc": map[string]interface{}{"addr": ""},
		},
		"data": map[string]interface{}{
			"redis": map[string]interface{}{"addr": "127.0.0.1:6379", "password": "", "db": 0},
		},
		"discovery": map[string]interface{}{
			"mode": "redis", "namespace": "seckill", "static_fallback": false,
			"ttl": "15s", "refresh_interval": "5s",
			"services": services, "advertise": advertise,
		},
		"processor": map[string]interface{}{
			"consumer_group":        "seckill-processor",
			"payment_timeout_delay": "10m",
			"queue_type":            "nats",
			"queue_size":            1024,
			"nats": map[string]interface{}{
				"url":                     "nats://127.0.0.1:4222",
				"stream":                  "SECKILL",
				"subject":                 "seckill.order.part_in",
				"post_pay_subject":        "seckill.post_pay",
				"payment_timeout_subject": "seckill.payment.timeout",
			},
			"cache": map[string]interface{}{
				"detail": map[string]interface{}{
					"local_ttl":   "2m",
					"shards":      16,
					"max_entries": 1024,
				},
				"sku": map[string]interface{}{
					"local_ttl":   "2m",
					"shards":      32,
					"max_entries": 4096,
				},
			},
		},
		"rpc": map[string]interface{}{
			"timeout": "3s",
			"circuit_breaker": map[string]interface{}{
				"enabled": true,
			},
		},
	}
}

func loadProcessorConfig(raw map[string]interface{}) ProcessorConfig {
	cfg := ProcessorConfig{
		ConsumerGroup:       config.GetString(raw, "processor.consumer_group"),
		PaymentTimeoutDelay: config.GetDuration(raw, "processor.payment_timeout_delay", 10*time.Minute),
		QueueType:           config.GetString(raw, "processor.queue_type"),
		QueueSize:           config.GetInt(raw, "processor.queue_size"),
		NATS: NATSConfig{
			URL:                   config.GetString(raw, "processor.nats.url"),
			Stream:                config.GetString(raw, "processor.nats.stream"),
			Subject:               config.GetString(raw, "processor.nats.subject"),
			PostPaySubject:        config.GetString(raw, "processor.nats.post_pay_subject"),
			PaymentTimeoutSubject: config.GetString(raw, "processor.nats.payment_timeout_subject"),
		},
		Cache: loadCacheConfig(raw),
	}
	if cfg.QueueType == "" {
		cfg.QueueType = "nats"
	}
	if cfg.NATS.URL == "" {
		cfg.NATS.URL = "nats://127.0.0.1:4222"
	}
	if cfg.NATS.Stream == "" {
		cfg.NATS.Stream = "SECKILL"
	}
	if cfg.NATS.Subject == "" {
		cfg.NATS.Subject = "seckill.order.part_in"
	}
	if cfg.NATS.PostPaySubject == "" {
		cfg.NATS.PostPaySubject = "seckill.post_pay"
	}
	if cfg.NATS.PaymentTimeoutSubject == "" {
		cfg.NATS.PaymentTimeoutSubject = "seckill.payment.timeout"
	}
	return cfg
}

func loadCacheConfig(raw map[string]interface{}) CacheConfig {
	detail := CacheEntryConfig{
		LocalTTL:   config.GetDuration(raw, "processor.cache.detail.local_ttl", 2*time.Minute),
		Shards:     config.GetInt(raw, "processor.cache.detail.shards"),
		MaxEntries: config.GetInt(raw, "processor.cache.detail.max_entries"),
	}
	if detail.Shards <= 0 {
		detail.Shards = 16
	}
	if detail.MaxEntries <= 0 {
		detail.MaxEntries = 1024
	}
	sku := CacheEntryConfig{
		LocalTTL:   config.GetDuration(raw, "processor.cache.sku.local_ttl", 2*time.Minute),
		Shards:     config.GetInt(raw, "processor.cache.sku.shards"),
		MaxEntries: config.GetInt(raw, "processor.cache.sku.max_entries"),
	}
	if sku.Shards <= 0 {
		sku.Shards = 32
	}
	if sku.MaxEntries <= 0 {
		sku.MaxEntries = 4096
	}
	return CacheConfig{Detail: detail, SKU: sku}
}

func loadRPCConfig(cfg config.Config, raw map[string]interface{}) RPCConfig {
	fallback := func(name string) string {
		if addrs := cfg.Discovery[name]; len(addrs) > 0 {
			return addrs[0]
		}
		return ""
	}
	endpoint := func(serviceName string, key string) string {
		return rpcclient.Endpoint(serviceName, config.GetString(raw, "rpc."+key), fallback(serviceName))
	}
	return RPCConfig{
		Timeout:              config.GetDuration(raw, "rpc.timeout", 3*time.Second),
		CircuitBreaker:       config.GetBool(raw, "rpc.circuit_breaker.enabled"),
		CircuitBreakerPolicy: rpcclient.CircuitBreakerPolicyFromMap(raw, "rpc.circuit_breaker"),
		ActivityAddr:         endpoint("activity-service", "activity"),
		StockAddr:            endpoint("stock-service", "stock"),
		OrderAddr:            endpoint("order-service", "order"),
		RiskAddr:             endpoint("risk-service", "risk"),
		SupportAddr:          endpoint("support-service", "support"),
	}
}
