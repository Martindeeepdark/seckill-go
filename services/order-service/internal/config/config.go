// Package config 提供订单服务的配置管理
package config

import (
	"fmt"
	"time"

	commonconfig "seckill-common/config"
)

// CacheConfig 控制 OrderCache 的本地缓存参数
type CacheConfig struct {
	Order CacheEntryConfig
}

// CacheEntryConfig 描述单层缓存的 BigCache 参数
type CacheEntryConfig struct {
	LocalTTL   time.Duration
	Shards     int
	MaxEntries int
}

// Load 读取配置文件，返回通用配置和订单缓存配置
func Load(path string) (commonconfig.Config, CacheConfig, error) {
	raw, err := commonconfig.LoadRaw(path, commonconfig.DefaultEndpoints())
	if err != nil {
		return commonconfig.Config{}, CacheConfig{}, fmt.Errorf("load config %s: %w", path, err)
	}
	cfg := commonconfig.FromMap(raw, "order-service")
	return cfg, loadCacheConfig(raw), nil
}

func loadCacheConfig(raw map[string]interface{}) CacheConfig {
	order := CacheEntryConfig{
		LocalTTL:   commonconfig.GetDuration(raw, "order.cache.order.local_ttl", 3*time.Minute),
		Shards:     commonconfig.GetInt(raw, "order.cache.order.shards"),
		MaxEntries: commonconfig.GetInt(raw, "order.cache.order.max_entries"),
	}
	if order.Shards <= 0 {
		order.Shards = 16
	}
	if order.MaxEntries <= 0 {
		order.MaxEntries = 1024
	}
	return CacheConfig{Order: order}
}
