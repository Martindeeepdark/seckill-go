// Package config 提供风控服务的配置管理
package config

import (
	"fmt"
	"time"

	commonconfig "seckill-common/config"
)

// CacheConfig 控制 RiskCache 的本地缓存参数
type CacheConfig struct {
	Risk CacheEntryConfig
}

// CacheEntryConfig 描述单层缓存的 BigCache 参数
type CacheEntryConfig struct {
	LocalTTL   time.Duration
	Shards     int
	MaxEntries int
}

// Load 读取配置文件，返回通用配置和风控缓存配置
func Load(path string) (commonconfig.Config, CacheConfig, error) {
	raw, err := commonconfig.LoadRaw(path, commonconfig.DefaultEndpoints())
	if err != nil {
		return commonconfig.Config{}, CacheConfig{}, fmt.Errorf("load config %s: %w", path, err)
	}
	cfg := commonconfig.FromMap(raw, "risk-service")
	return cfg, loadCacheConfig(raw), nil
}

func loadCacheConfig(raw map[string]interface{}) CacheConfig {
	risk := CacheEntryConfig{
		LocalTTL:   commonconfig.GetDuration(raw, "risk.cache.risk.local_ttl", 60*time.Second),
		Shards:     commonconfig.GetInt(raw, "risk.cache.risk.shards"),
		MaxEntries: commonconfig.GetInt(raw, "risk.cache.risk.max_entries"),
	}
	if risk.Shards <= 0 {
		risk.Shards = 16
	}
	if risk.MaxEntries <= 0 {
		risk.MaxEntries = 2048
	}
	return CacheConfig{Risk: risk}
}
