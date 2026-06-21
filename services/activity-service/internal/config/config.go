// Package config 提供秒杀活动服务的配置加载和结构定义。
package config

import (
	"fmt"
	"time"

	commonconfig "github.com/Martindeeepdark/go-common/config"

	commconfig "seckill-common/config"
)

// Config 是活动服务的配置结构。
type Config struct {
	commconfig.Config
	Cache CacheConfig
}

// CacheConfig 定义缓存相关配置。
type CacheConfig struct {
	Activity ActivityCacheConfig
}

// ActivityCacheConfig 定义活动缓存配置。
type ActivityCacheConfig struct {
	Enabled        bool          // 是否启用缓存
	MaxSize        int           // 本地缓存最大条目数
	LocalTTL       time.Duration // 本地缓存过期时间
	RefreshAfter   time.Duration // 本地缓存刷新时间
	RedisTTL       time.Duration // Redis 缓存过期时间
	NullTTL        time.Duration // 空值缓存过期时间
	WarmupAhead    time.Duration // 预热提前时间
	RefreshEnabled bool          // 是否启用自动刷新
	RefreshInitial time.Duration // 初始刷新延迟
	RefreshTick    time.Duration // 刷新间隔
}

// Load 从指定路径加载服务配置。
func Load(path string, serviceName string) (Config, error) {
	base, err := commconfig.Load(path, serviceName, commconfig.DefaultEndpoints())
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	cache, err := loadCacheConfig(path)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Config: base,
		Cache:  cache,
	}, nil
}

// loadCacheConfig 加载缓存配置。
func loadCacheConfig(path string) (CacheConfig, error) {
	raw := defaultCacheMap()
	loaded, err := commonconfig.Load(path)
	if err != nil {
		return CacheConfig{}, fmt.Errorf("load cache config %s: %w", path, err)
	}
	commconfig.MergeMap(raw, loaded.ToMap())
	return cacheFromMap(raw), nil
}

// defaultCacheMap 返回缓存配置的默认值。
func defaultCacheMap() map[string]interface{} {
	return map[string]interface{}{
		"cache": map[string]interface{}{
			"activity": map[string]interface{}{
				"enabled":         true,
				"max_size":        512,
				"local_ttl":       "30m",
				"refresh_after":   "5s",
				"redis_ttl":       "30m",
				"null_ttl":        "60s",
				"warmup_ahead":    "10m",
				"refresh_enabled": true,
				"refresh_initial": "30s",
				"refresh_tick":    "60s",
			},
		},
	}
}

// cacheFromMap 从配置字典构造缓存配置结构。
func cacheFromMap(raw map[string]interface{}) CacheConfig {
	return CacheConfig{
		Activity: ActivityCacheConfig{
			Enabled:        commconfig.GetBool(raw, "cache.activity.enabled"),
			MaxSize:        commconfig.GetInt(raw, "cache.activity.max_size"),
			LocalTTL:       commconfig.GetDuration(raw, "cache.activity.local_ttl", 30*time.Minute),
			RefreshAfter:   commconfig.GetDuration(raw, "cache.activity.refresh_after", 5*time.Second),
			RedisTTL:       commconfig.GetDuration(raw, "cache.activity.redis_ttl", 30*time.Minute),
			NullTTL:        commconfig.GetDuration(raw, "cache.activity.null_ttl", time.Minute),
			WarmupAhead:    commconfig.GetDuration(raw, "cache.activity.warmup_ahead", 10*time.Minute),
			RefreshEnabled: commconfig.GetBool(raw, "cache.activity.refresh_enabled"),
			RefreshInitial: commconfig.GetDuration(raw, "cache.activity.refresh_initial", 30*time.Second),
			RefreshTick:    commconfig.GetDuration(raw, "cache.activity.refresh_tick", time.Minute),
		},
	}
}
