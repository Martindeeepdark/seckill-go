// Package config 提供 seckill-gateway 的配置管理
package config

import (
	"fmt"
	"strings"
	"time"

	commonfile "github.com/Martindeeepdark/go-common/config"

	commonconfig "seckill-common/config"
	"seckill-common/rpcclient"
)

// Config 包含 seckill-gateway 服务的所有配置
type Config struct {
	commonconfig.Config
	HTTPAddr string
	RPC      RPCConfig
	MQ       MQConfig
	Gateway  GatewayConfig
	Etcd     EtcdConfig
}

// RPCConfig 保存后端 gRPC 服务的端点地址
type RPCConfig struct {
	Activity              string
	Stock                 string
	Risk                  string
	Order                 string
	Payment               string
	Timeout               time.Duration
	CircuitBreakerEnabled bool
	Pool                  rpcclient.ConnectionPoolConfig
}

// MQConfig 控制用于移交秒杀请求的外部异步队列
type MQConfig struct {
	Type               string
	NATSURL            string
	NATSStream         string
	NATSSubject        string
	NATSPostPaySubject string
}

// GatewayConfig 保存 gateway 特定设置
type GatewayConfig struct {
	MachineCheck MachineCheckConfig
	RateLimit    RateLimitConfig
	Risk         RiskConfig
	Degrade      DegradeConfig
	Cache        CacheConfig
}

// CacheConfig 控制 ActivityCache 的本地缓存参数与后台刷新间隔
type CacheConfig struct {
	RefreshInterval time.Duration
	Detail          ActivityCacheEntryConfig
	List            ActivityCacheEntryConfig
}

// ActivityCacheEntryConfig 描述单层 (detail 或 list) 缓存的 BigCache 参数
type ActivityCacheEntryConfig struct {
	LocalTTL   time.Duration
	Shards     int
	MaxEntries int
}

// MachineCheckConfig 控制反机器人验证
type MachineCheckConfig struct {
	Enabled      bool
	Mode         string
	Secret       string
	RandomLength int
	TTL          time.Duration
}

// RateLimitConfig 控制速率限制
type RateLimitConfig struct {
	Enabled      bool
	MaxQPS       int
	Rules        []RateLimitRule
	UserEnabled  bool
	UserRate     int
	UserInterval time.Duration
	UserExpire   time.Duration
}

// RateLimitRule 限流规则
type RateLimitRule struct {
	Resource string
	Count    int
	Interval time.Duration
}

// RiskConfig 控制风险评估
type RiskConfig struct {
	Enabled bool
}

// DegradeConfig 控制熔断器设置
type DegradeConfig struct {
	Enabled          bool
	FailureThreshold int
	Timeout          time.Duration
}

// EtcdConfig 控制 etcd 配置源连接
type EtcdConfig struct {
	Endpoints []string
	Prefix    string
}

// Load 读取并解析配置文件
func Load(path string) (Config, error) {
	raw, err := LoadRaw(path)
	if err != nil {
		return Config{}, err
	}
	base := commonconfig.FromMap(raw, "seckill-gateway")
	cfg := Config{
		Config:   base,
		HTTPAddr: commonconfig.ServiceAddr(raw, "seckill-gateway", "http.addr"),
		RPC:      rpcFromMap(raw),
		MQ:       mqFromMap(raw),
		Gateway:  GatewayFromMap(raw),
		Etcd:     etcdFromMap(raw),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = "0.0.0.0:8080"
	}
	return cfg, nil
}

// LoadRaw 读取配置文件并返回合并后的原始 map，用于 etcd 动态合并。
func LoadRaw(path string) (map[string]any, error) {
	raw := defaultMap()
	loaded, err := commonfile.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	commonconfig.MergeMap(raw, loaded.ToMap())
	return raw, nil
}

// defaultMap 返回默认配置映射
func defaultMap() map[string]any {
	endpoints := commonconfig.DefaultEndpoints()
	services := map[string]any{}
	advertise := map[string]any{}
	for _, ep := range endpoints {
		services[ep.Name] = []any{ep.Address}
		advertise[ep.Name] = ep.Address
	}
	return map[string]any{
		"log":   map[string]any{"level": "info"},
		"trace": map[string]any{"enabled": false, "endpoint": "127.0.0.1:4317", "insecure": true},
		"services": map[string]any{
			"seckill-gateway": map[string]any{"http": map[string]any{"addr": "0.0.0.0:8080"}},
		},
		"data": map[string]any{"redis": map[string]any{"addr": "127.0.0.1:6379", "password": "", "db": 0}},
		"mq": map[string]any{
			"type": "nats",
			"nats": map[string]any{
				"url":              "nats://127.0.0.1:4222",
				"stream":           "SECKILL",
				"subject":          "seckill.order.part_in",
				"post_pay_subject": "seckill.post_pay",
			},
		},
		"discovery": map[string]any{
			"mode": "redis", "namespace": "seckill", "static_fallback": true,
			"ttl": "15s", "refresh_interval": "5s",
			"services":  services,
			"advertise": advertise,
		},
		"rpc": map[string]any{
			"activity": "discovery:///activity-service",
			"stock":    "discovery:///stock-service",
			"risk":     "discovery:///risk-service",
			"order":    "discovery:///order-service",
			"payment":  "discovery:///support-service",
			"timeout":  "3s",
			"circuit_breaker": map[string]any{
				"enabled": true,
			},
			"initial_window_size":      262144,
			"initial_conn_window_size": 1048576,
			"max_recv_msg_size":        4194304,
			"keepalive_time":           "30s",
			"keepalive_timeout":        "10s",
		},
		"gateway": map[string]any{
			"machine_check": map[string]any{
				"enabled":       false,
				"mode":          "java",
				"secret":        "",
				"random_length": 16,
				"ttl":           "30s",
			},
			"rate_limit": map[string]any{
				"enabled": false,
				"max_qps": 100,
				"rules": []any{
					map[string]any{"resource": "POST /api/seckill/part-in", "count": 1000, "interval": "1s"},
					map[string]any{"resource": "GET /api/activities", "count": 200, "interval": "1s"},
				},
				"user_enabled":  false,
				"user_rate":     10,
				"user_interval": "10s",
				"user_expire":   "5m",
			},
			"risk": map[string]any{
				"enabled": true,
			},
			"degrade": map[string]any{
				"enabled":           false,
				"failure_threshold": 5,
				"timeout":           "10s",
			},
			"cache": map[string]any{
				"refresh_interval": "30s",
				"detail": map[string]any{
					"local_ttl":   "5m",
					"shards":      16,
					"max_entries": 256,
				},
				"list": map[string]any{
					"local_ttl":   "5m",
					"shards":      4,
					"max_entries": 4,
				},
			},
		},
	}
}

// mqFromMap 从映射创建 MQ 配置
func mqFromMap(raw map[string]any) MQConfig {
	cfg := MQConfig{
		Type:               commonconfig.GetString(raw, "mq.type"),
		NATSURL:            commonconfig.GetString(raw, "mq.nats.url"),
		NATSStream:         commonconfig.GetString(raw, "mq.nats.stream"),
		NATSSubject:        commonconfig.GetString(raw, "mq.nats.subject"),
		NATSPostPaySubject: commonconfig.GetString(raw, "mq.nats.post_pay_subject"),
	}
	if cfg.Type == "" {
		cfg.Type = "nats"
	}
	if cfg.NATSURL == "" {
		cfg.NATSURL = "nats://127.0.0.1:4222"
	}
	if cfg.NATSStream == "" {
		cfg.NATSStream = "SECKILL"
	}
	if cfg.NATSSubject == "" {
		cfg.NATSSubject = "seckill.order.part_in"
	}
	if cfg.NATSPostPaySubject == "" {
		cfg.NATSPostPaySubject = "seckill.post_pay"
	}
	return cfg
}

// rpcFromMap 从映射创建 RPC 配置
func rpcFromMap(raw map[string]any) RPCConfig {
	return RPCConfig{
		Activity:              commonconfig.GetString(raw, "rpc.activity"),
		Stock:                 commonconfig.GetString(raw, "rpc.stock"),
		Risk:                  commonconfig.GetString(raw, "rpc.risk"),
		Order:                 commonconfig.GetString(raw, "rpc.order"),
		Payment:               commonconfig.GetString(raw, "rpc.payment"),
		Timeout:               commonconfig.GetDuration(raw, "rpc.timeout", 3*time.Second),
		CircuitBreakerEnabled: commonconfig.GetBool(raw, "rpc.circuit_breaker.enabled"),
		Pool:                  rpcclient.ConnectionPoolConfigFromMap(raw, "rpc"),
	}
}

// GatewayFromMap 从映射创建 Gateway 配置。
// 导出供 main.go 在 etcd watch 回调中复用同一份解析逻辑。
func GatewayFromMap(raw map[string]any) GatewayConfig {
	machineTTL := commonconfig.GetDuration(raw, "gateway.machine_check.ttl", 0)
	if machineTTL <= 0 {
		if seconds := commonconfig.GetInt(raw, "gateway.machine_check.ttl_seconds"); seconds > 0 {
			machineTTL = time.Duration(seconds) * time.Second
		} else {
			machineTTL = 30 * time.Second
		}
	}
	return GatewayConfig{
		MachineCheck: MachineCheckConfig{
			Enabled:      commonconfig.GetBool(raw, "gateway.machine_check.enabled"),
			Mode:         commonconfig.GetString(raw, "gateway.machine_check.mode"),
			Secret:       commonconfig.GetString(raw, "gateway.machine_check.secret"),
			RandomLength: commonconfig.GetInt(raw, "gateway.machine_check.random_length"),
			TTL:          machineTTL,
		},
		RateLimit: RateLimitConfig{
			Enabled:      commonconfig.GetBool(raw, "gateway.rate_limit.enabled"),
			MaxQPS:       commonconfig.GetInt(raw, "gateway.rate_limit.max_qps"),
			Rules:        rateLimitRulesFromMap(raw),
			UserEnabled:  commonconfig.GetBool(raw, "gateway.rate_limit.user_enabled"),
			UserRate:     commonconfig.GetInt(raw, "gateway.rate_limit.user_rate"),
			UserInterval: commonconfig.GetDuration(raw, "gateway.rate_limit.user_interval", 10*time.Second),
			UserExpire:   commonconfig.GetDuration(raw, "gateway.rate_limit.user_expire", 5*time.Minute),
		},
		Risk: RiskConfig{
			Enabled: commonconfig.GetBool(raw, "gateway.risk.enabled"),
		},
		Degrade: DegradeConfig{
			Enabled:          commonconfig.GetBool(raw, "gateway.degrade.enabled"),
			FailureThreshold: commonconfig.GetInt(raw, "gateway.degrade.failure_threshold"),
			Timeout:          commonconfig.GetDuration(raw, "gateway.degrade.timeout", 10*time.Second),
		},
		Cache: cacheFromMap(raw),
	}
}

// cacheFromMap 从映射创建 ActivityCache 配置
func cacheFromMap(raw map[string]any) CacheConfig {
	detail := ActivityCacheEntryConfig{
		LocalTTL:   commonconfig.GetDuration(raw, "gateway.cache.detail.local_ttl", 5*time.Minute),
		Shards:     commonconfig.GetInt(raw, "gateway.cache.detail.shards"),
		MaxEntries: commonconfig.GetInt(raw, "gateway.cache.detail.max_entries"),
	}
	if detail.Shards <= 0 {
		detail.Shards = 16
	}
	if detail.MaxEntries <= 0 {
		detail.MaxEntries = 256
	}
	list := ActivityCacheEntryConfig{
		LocalTTL:   commonconfig.GetDuration(raw, "gateway.cache.list.local_ttl", 5*time.Minute),
		Shards:     commonconfig.GetInt(raw, "gateway.cache.list.shards"),
		MaxEntries: commonconfig.GetInt(raw, "gateway.cache.list.max_entries"),
	}
	if list.Shards <= 0 {
		list.Shards = 4
	}
	if list.MaxEntries <= 0 {
		list.MaxEntries = 4
	}
	return CacheConfig{
		RefreshInterval: commonconfig.GetDuration(raw, "gateway.cache.refresh_interval", 30*time.Second),
		Detail:          detail,
		List:            list,
	}
}

// etcdFromMap 从映射创建 Etcd 配置
func etcdFromMap(raw map[string]any) EtcdConfig {
	var endpoints []string
	if v := commonconfig.GetString(raw, "etcd.endpoints"); v != "" {
		for _, ep := range splitList(v) {
			ep = strings.TrimSpace(ep)
			if ep != "" {
				endpoints = append(endpoints, ep)
			}
		}
	}
	prefix := commonconfig.GetString(raw, "etcd.prefix")
	if prefix == "" {
		prefix = "/seckill/config"
	}
	return EtcdConfig{Endpoints: endpoints, Prefix: prefix}
}

func splitList(s string) []string {
	return strings.Split(s, ",")
}

// rateLimitRulesFromMap 从映射创建限流规则列表
func rateLimitRulesFromMap(raw map[string]any) []RateLimitRule {
	value, ok := commonconfig.Lookup(raw, "gateway.rate_limit.rules")
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	rules := make([]RateLimitRule, 0, len(items))
	for _, item := range items {
		source, ok := item.(map[string]any)
		if !ok {
			continue
		}
		nested := map[string]any{"rule": source}
		resource := commonconfig.GetString(nested, "rule.resource")
		count := commonconfig.GetInt(nested, "rule.count")
		if resource == "" || count <= 0 {
			continue
		}
		rules = append(rules, RateLimitRule{
			Resource: resource,
			Count:    count,
			Interval: commonconfig.GetDuration(nested, "rule.interval", time.Second),
		})
	}
	return rules
}
