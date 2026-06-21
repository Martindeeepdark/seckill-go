// Package config 提供定时任务的配置加载和解析
package config

import (
	"fmt"
	"time"

	commonconfig "github.com/Martindeeepdark/go-common/config"

	"seckill-common/config"
	"seckill-common/rpcclient"
)

// JobConfig 定时任务配置
type JobConfig struct {
	RunOnStart                  bool          // 是否在启动时立即执行任务
	ActivityStatusCheckInterval time.Duration // 活动状态检查间隔
	TimeoutOrderCheckInterval   time.Duration // 超时订单检查间隔
	StockReleaseCheckInterval   time.Duration // 库存释放检查间隔
	ActivityDataCleanupInterval time.Duration // 活动数据清理间隔
	ActivityDataRetention       time.Duration // 活动数据保留时长
	RiskUserCleanupInterval     time.Duration // 风控用户清理间隔
	DailyStatisticsInterval     time.Duration // 每日统计间隔
	CacheWarmupInterval         time.Duration // 缓存预热间隔
	CacheRefreshInterval        time.Duration // 缓存刷新间隔
	CacheWarmupAhead            time.Duration // 缓存预热提前时长
	PaymentReconcileInterval    time.Duration // 支付对账间隔
	OrderSyncCheckInterval      time.Duration // 订单同步检查间隔
}

// RPCConfig RPC客户端配置
type RPCConfig struct {
	Timeout              time.Duration                  // RPC调用超时时间
	CircuitBreaker       bool                           // 是否启用熔断器
	CircuitBreakerPolicy rpcclient.CircuitBreakerPolicy // 熔断器策略
	ActivityAddr         string                         // 活动服务地址
	StockAddr            string                         // 库存服务地址
	OrderAddr            string                         // 订单服务地址
	RiskAddr             string                         // 风控服务地址
	SupportAddr          string                         // 支持服务地址
}

// AppCBConfig 应用层熔断器配置，per-service 粒度。
type AppCBConfig struct {
	// Enabled 是否启用应用层熔断。
	Enabled bool
	// Success 成功率阈值（0~1），默认 0.6。
	Success float64
	// Request 触发熔断计算的最小请求数，默认 100。
	Request int64
	// Window 统计窗口时长，默认 10s。
	Window time.Duration
}

// Load 从配置文件加载所有配置
// path: 配置文件路径
// 返回: 通用配置, 任务配置, RPC配置, 应用层熔断器配置, 错误
func Load(path string) (config.Config, JobConfig, RPCConfig, AppCBConfig, error) {
	raw := defaultJobMap()
	loaded, err := commonconfig.Load(path)
	if err != nil {
		return config.Config{}, JobConfig{}, RPCConfig{}, AppCBConfig{}, fmt.Errorf("load config %s: %w", path, err)
	}
	config.MergeMap(raw, loaded.ToMap())
	cfg := config.FromMap(raw, "seckill-job")
	return cfg, loadJobConfig(raw), loadRPCConfig(cfg, raw), loadAppCBConfig(raw), nil
}

// defaultJobMap 返回默认配置映射
func defaultJobMap() map[string]interface{} {
	services := map[string]interface{}{
		"activity-service": []interface{}{"127.0.0.1:9001"},
		"stock-service":    []interface{}{"127.0.0.1:9002"},
		"risk-service":     []interface{}{"127.0.0.1:9003"},
		"order-service":    []interface{}{"127.0.0.1:9004"},
		"support-service":  []interface{}{"127.0.0.1:9005"},
	}
	advertise := map[string]interface{}{
		"seckill-job": "",
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
		"job": map[string]interface{}{
			"run_on_start":                   true,
			"activity_status_check_interval": "1m",
			"timeout_order_check_interval":   "10m",
			"stock_release_check_interval":   "10m",
			"activity_data_cleanup_interval": "24h",
			"activity_data_retention":        "24h",
			"risk_user_cleanup_interval":     "24h",
			"daily_statistics_interval":      "24h",
			"cache_warmup_interval":          "5m",
			"cache_refresh_interval":         "1m",
			"cache_warmup_ahead":             "10m",
			"payment_reconcile_interval":     "30m",
			"order_sync_check_interval":      "5m",
		},
		"rpc": map[string]interface{}{
			"timeout": "3s",
			"circuit_breaker": map[string]interface{}{
				"enabled": true,
			},
		},
		"app_cb": map[string]interface{}{
			"enabled": true,
			"success": 0.6,
			"request": 100,
			"window":  "10s",
		},
	}
}

// loadJobConfig 从原始配置映射加载任务配置
func loadJobConfig(raw map[string]interface{}) JobConfig {
	return JobConfig{
		RunOnStart:                  config.GetBool(raw, "job.run_on_start"),
		ActivityStatusCheckInterval: config.GetDuration(raw, "job.activity_status_check_interval", time.Minute),
		TimeoutOrderCheckInterval:   config.GetDuration(raw, "job.timeout_order_check_interval", 10*time.Minute),
		StockReleaseCheckInterval:   config.GetDuration(raw, "job.stock_release_check_interval", 10*time.Minute),
		ActivityDataCleanupInterval: config.GetDuration(raw, "job.activity_data_cleanup_interval", 24*time.Hour),
		ActivityDataRetention:       config.GetDuration(raw, "job.activity_data_retention", 24*time.Hour),
		RiskUserCleanupInterval:     config.GetDuration(raw, "job.risk_user_cleanup_interval", 24*time.Hour),
		DailyStatisticsInterval:     config.GetDuration(raw, "job.daily_statistics_interval", 24*time.Hour),
		CacheWarmupInterval:         config.GetDuration(raw, "job.cache_warmup_interval", 5*time.Minute),
		CacheRefreshInterval:        config.GetDuration(raw, "job.cache_refresh_interval", time.Minute),
		CacheWarmupAhead:            config.GetDuration(raw, "job.cache_warmup_ahead", 10*time.Minute),
		PaymentReconcileInterval:    config.GetDuration(raw, "job.payment_reconcile_interval", 30*time.Minute),
		OrderSyncCheckInterval:      config.GetDuration(raw, "job.order_sync_check_interval", 5*time.Minute),
	}
}

// loadRPCConfig 从配置加载RPC客户端配置
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

// loadAppCBConfig 从配置加载应用层熔断器配置
func loadAppCBConfig(raw map[string]interface{}) AppCBConfig {
	success := 0.6
	if v, ok := config.Lookup(raw, "app_cb.success"); ok {
		if f, ok := v.(float64); ok {
			success = f
		}
	}
	request := int64(100)
	if v := config.GetInt(raw, "app_cb.request"); v > 0 {
		request = int64(v)
	}
	return AppCBConfig{
		Enabled: config.GetBool(raw, "app_cb.enabled"),
		Success: success,
		Request: request,
		Window:  config.GetDuration(raw, "app_cb.window", 10*time.Second),
	}
}
