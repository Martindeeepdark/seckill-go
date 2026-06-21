// Package config 提供配置加载、解析和合并功能
// 支持从文件加载配置、默认值设置、服务发现配置等
package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	commonconfig "github.com/Martindeeepdark/go-common/config"
)

// Config 服务配置结构体
// 包含服务运行所需的所有配置项：数据库、Redis、服务发现、日志、链路追踪等
type Config struct {
	ServiceName             string              // 服务名称
	GRPCAddr                string              // gRPC 服务监听地址
	RedisAddr               string              // Redis 服务器地址
	RedisPassword           string              // Redis 密码
	RedisDB                 int                 // Redis 数据库编号
	PGHost                  string              // PostgreSQL 主机地址
	PGPort                  int                 // PostgreSQL 端口
	PGUser                  string              // PostgreSQL 用户名
	PGPassword              string              // PostgreSQL 密码
	PGDatabase              string              // PostgreSQL 数据库名
	NATSAddr                string              // NATS 消息队列地址
	Discovery               map[string][]string // 服务发现静态配置
	DiscoveryMode           string              // 服务发现模式：etcd、redis 或 static
	DiscoveryNS             string              // 服务发现命名空间
	DiscoveryStaticFallback bool                // 发现失败时是否回退到静态配置
	DiscoveryTTL            time.Duration       // 服务注册的 TTL
	DiscoveryTick           time.Duration       // 服务发现的刷新间隔
	EtcdEndpoints           []string            // etcd 集群地址
	AdvertiseAddr           map[string]string   // 服务对外公告的地址
	LogLevel                slog.Level          // 日志级别
	TraceEnabled            bool                // 是否启用链路追踪
	TraceEndpoint           string              // 链路追踪端点地址
	TraceInsecure           bool                // 链路追踪是否使用不安全连接
}

// ServiceEndpoint 服务端点配置
// 用于配置服务的名称和地址
type ServiceEndpoint struct {
	Name    string // 服务名称
	Address string // 服务地址
}

// Load 从指定路径加载配置文件
// 参数：
//   - path: 配置文件路径
//   - serviceName: 服务名称
//   - endpoints: 服务端点列表
//
// 返回合并后的配置对象
func Load(path string, serviceName string, endpoints []ServiceEndpoint) (Config, error) {
	raw, err := LoadRaw(path, endpoints)
	if err != nil {
		return Config{}, err
	}
	return FromMap(raw, serviceName), nil
}

// LoadRaw 加载配置文件并合并默认值，返回原始 map
// 供 etcd 配置源等需要进一步处理的场景使用
func LoadRaw(path string, endpoints []ServiceEndpoint) (map[string]interface{}, error) {
	raw := defaultMap(endpoints)
	loaded, err := commonconfig.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	MergeMap(raw, loaded.ToMap())
	return raw, nil
}

// defaultMap 创建默认配置映射
// 根据提供的端点列表生成默认的服务发现配置
func defaultMap(endpoints []ServiceEndpoint) map[string]interface{} {
	services := map[string]interface{}{}
	advertise := map[string]interface{}{}
	for _, ep := range endpoints {
		services[ep.Name] = []interface{}{ep.Address}
		advertise[ep.Name] = ep.Address
	}
	return map[string]interface{}{
		"log":    map[string]interface{}{"level": "info"},
		"trace":  map[string]interface{}{"enabled": false, "endpoint": "127.0.0.1:4317", "insecure": true},
		"server": map[string]interface{}{"grpc": map[string]interface{}{"addr": ""}},
		"data":   map[string]interface{}{"redis": map[string]interface{}{"addr": "127.0.0.1:6379", "password": "", "db": 0}, "postgres": map[string]interface{}{"host": "127.0.0.1", "port": 5432, "user": "seckill", "password": "seckill123", "database": "seckill"}, "nats": map[string]interface{}{"addr": ""}},
		"discovery": map[string]interface{}{
			"mode": "redis", "namespace": "seckill", "static_fallback": false,
			"ttl": "15s", "refresh_interval": "5s",
			"services":  services,
			"advertise": advertise,
		},
	}
}

// DefaultEndpoints 返回默认的服务端点列表
// 包含所有内置服务的默认本地地址
func DefaultEndpoints() []ServiceEndpoint {
	return []ServiceEndpoint{
		{"activity-service", "127.0.0.1:9001"},
		{"stock-service", "127.0.0.1:9002"},
		{"risk-service", "127.0.0.1:9003"},
		{"order-service", "127.0.0.1:9004"},
		{"support-service", "127.0.0.1:9005"},
	}
}

// FromMap 从配置映射中构建 Config 对象
// 从嵌套的 map 结构中提取各类配置项
func FromMap(raw map[string]interface{}, serviceName string) Config {
	return Config{
		ServiceName:             serviceName,
		GRPCAddr:                ServiceAddr(raw, serviceName, "grpc.addr"),
		RedisAddr:               GetString(raw, "data.redis.addr"),
		RedisPassword:           GetString(raw, "data.redis.password"),
		RedisDB:                 GetInt(raw, "data.redis.db"),
		PGHost:                  GetString(raw, "data.postgres.host"),
		PGPort:                  GetInt(raw, "data.postgres.port"),
		PGUser:                  GetString(raw, "data.postgres.user"),
		PGPassword:              GetString(raw, "data.postgres.password"),
		PGDatabase:              GetString(raw, "data.postgres.database"),
		NATSAddr:                GetString(raw, "data.nats.addr"),
		Discovery:               GetStringSliceMap(raw, "discovery.services"),
		DiscoveryMode:           GetString(raw, "discovery.mode"),
		DiscoveryNS:             GetString(raw, "discovery.namespace"),
		DiscoveryStaticFallback: GetBool(raw, "discovery.static_fallback"),
		DiscoveryTTL:            GetDuration(raw, "discovery.ttl", 15*time.Second),
		DiscoveryTick:           GetDuration(raw, "discovery.refresh_interval", 5*time.Second),
		EtcdEndpoints:           GetStringSlice(raw, "discovery.etcd.endpoints"),
		AdvertiseAddr:           GetStringMap(raw, "discovery.advertise"),
		LogLevel:                ParseLogLevel(GetString(raw, "log.level")),
		TraceEnabled:            GetBool(raw, "trace.enabled"),
		TraceEndpoint:           GetString(raw, "trace.endpoint"),
		TraceInsecure:           GetBool(raw, "trace.insecure"),
	}
}

// ServiceAddr 获取指定服务的配置地址
// 优先使用服务特定配置，回退到全局配置
func ServiceAddr(raw map[string]interface{}, serviceName string, key string) string {
	if v := GetString(raw, "services."+serviceName+"."+key); v != "" {
		return v
	}
	return GetString(raw, "server."+key)
}

// MergeMap 递归合并两个配置映射
// 将 src 的内容合并到 dst 中，对于嵌套 map 进行递归合并
func MergeMap(dst, src map[string]interface{}) {
	for key, value := range src {
		srcMap, srcOK := value.(map[string]interface{})
		dstMap, dstOK := dst[key].(map[string]interface{})
		if srcOK && dstOK {
			// 递归合并嵌套 map
			MergeMap(dstMap, srcMap)
			continue
		}
		dst[key] = value
	}
}

// GetString 从配置映射中获取字符串值
// 支持点号分隔的路径，如 "data.redis.addr"
func GetString(raw map[string]interface{}, path string) string {
	value, ok := Lookup(raw, path)
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

// GetStringSlice 从配置映射中获取字符串切片
// 支持多种输入类型：[]string、[]interface{}、逗号分隔的字符串
func GetStringSlice(raw map[string]interface{}, path string) []string {
	value, ok := Lookup(raw, path)
	if !ok || value == nil {
		return nil
	}
	switch v := value.(type) {
	case []string:
		return CompactStrings(v)
	case []interface{}:
		values := make([]string, 0, len(v))
		for _, item := range v {
			values = append(values, fmt.Sprint(item))
		}
		return CompactStrings(values)
	case string:
		// 支持逗号分隔的字符串
		return CompactStrings(strings.Split(v, ","))
	default:
		return CompactStrings([]string{fmt.Sprint(v)})
	}
}

// CompactStrings 去除字符串切片中的空白项
// 移除空白字符串，返回非空字符串切片
func CompactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			compacted = append(compacted, value)
		}
	}
	return compacted
}

// GetStringSliceMap 从配置映射中获取字符串切片映射
// 用于解析服务发现配置等服务列表
func GetStringSliceMap(raw map[string]interface{}, path string) map[string][]string {
	value, ok := Lookup(raw, path)
	if !ok || value == nil {
		return nil
	}
	source, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string][]string, len(source))
	for key, val := range source {
		nested := map[string]interface{}{"value": val}
		if values := GetStringSlice(nested, "value"); len(values) > 0 {
			result[key] = values
		}
	}
	return result
}

// GetStringMap 从配置映射中获取字符串映射
// 用于解析服务公告地址等键值对配置
func GetStringMap(raw map[string]interface{}, path string) map[string]string {
	value, ok := Lookup(raw, path)
	if !ok || value == nil {
		return nil
	}
	source, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, val := range source {
		text := strings.TrimSpace(fmt.Sprint(val))
		if text != "" {
			result[key] = text
		}
	}
	return result
}

// GetInt 从配置映射中获取整数值
// 支持多种数值类型的自动转换
func GetInt(raw map[string]interface{}, path string) int {
	value, ok := Lookup(raw, path)
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return intFromInt64(v)
	case int32:
		return int(v)
	case uint64:
		return intFromUint64(v)
	case uint:
		return intFromUint(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}

// GetBool 从配置映射中获取布尔值
func GetBool(raw map[string]interface{}, path string) bool {
	value, ok := Lookup(raw, path)
	if !ok {
		return false
	}
	v, ok := value.(bool)
	return ok && v
}

// GetDuration 从配置映射中获取时间间隔值
// 支持解析 "5s"、"1m" 等格式，解析失败时返回默认值
func GetDuration(raw map[string]interface{}, path string, fallback time.Duration) time.Duration {
	value := GetString(raw, path)
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}

// Lookup 在嵌套映射中查找指定路径的值
// 路径使用点号分隔，如 "data.redis.addr"
func Lookup(raw map[string]interface{}, path string) (interface{}, bool) {
	current := interface{}(raw)
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// ParseLogLevel 解析日志级别字符串
// 支持的值：debug、info、warn、error，默认为 info
func ParseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// intFromInt64 安全地将 int64 转换为 int
// 检查溢出，溢出时返回 0
func intFromInt64(v int64) int {
	maxInt := int64(^uint(0) >> 1)
	minInt := -maxInt - 1
	if v > maxInt || v < minInt {
		return 0
	}
	return int(v)
}

// intFromUint64 安全地将 uint64 转换为 int
// 检查溢出，溢出时返回 0
func intFromUint64(v uint64) int {
	maxInt := uint64(^uint(0) >> 1)
	if v > maxInt {
		return 0
	}
	return int(v)
}

// intFromUint 安全地将 uint 转换为 int
// 检查溢出，溢出时返回 0
func intFromUint(v uint) int {
	maxInt := ^uint(0) >> 1
	if v > maxInt {
		return 0
	}
	return int(v)
}
