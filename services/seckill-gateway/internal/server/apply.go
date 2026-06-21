package server

import (
	commonconfig "seckill-common/config"

	"seckill-gateway-service/internal/config"
)

// ApplyGatewayConfig 将 config.GatewayConfig 的全部动态字段同步到运行时配置容器。
// 启动时用 YAML 配置初始化；etcd watch 回调中用合并后的配置再次调用。
// nil 接收者安全：方便在测试或组装阶段跳过未就绪的 runtime。
func ApplyGatewayConfig(rt *GatewayRuntimeConfig, cfg config.GatewayConfig) {
	if rt == nil {
		return
	}
	rules := make([]RateLimitRule, len(cfg.RateLimit.Rules))
	for i, r := range cfg.RateLimit.Rules {
		rules[i] = RateLimitRule{
			Resource: r.Resource,
			Count:    r.Count,
			Interval: r.Interval,
		}
	}
	rt.UpdateRateLimit(cfg.RateLimit.Enabled, RateLimitOptions{
		MaxQPS:       cfg.RateLimit.MaxQPS,
		Rules:        rules,
		UserEnabled:  cfg.RateLimit.UserEnabled,
		UserRate:     cfg.RateLimit.UserRate,
		UserInterval: cfg.RateLimit.UserInterval,
	})
	rt.UpdateDegrade(DegradeOptions{
		Enabled:          cfg.Degrade.Enabled,
		FailureThreshold: cfg.Degrade.FailureThreshold,
		Timeout:          cfg.Degrade.Timeout,
	})
	rt.UpdateRisk(cfg.Risk.Enabled)
	rt.UpdateMachineCheck(cfg.MachineCheck.Enabled)
}

// ApplyGatewaySegment 应用 base raw map 与 etcd gateway 段内容合并后的配置。
// base 是 LoadRaw 返回的完整 raw map（含 gateway 外层），
// etcdGatewaySegment 是 etcd 中存储的 gateway 段内容（无外层 wrapper，符合设计文档约定）。
// 合并目标是 base["gateway"]：etcd 中未出现的字段保留 base 值，避免 partial PUT 丢失配置。
// etcdGatewaySegment 为 nil 或空时直接用 base（DELETE 事件回退到 YAML 默认值）。
func ApplyGatewaySegment(rt *GatewayRuntimeConfig, base, etcdGatewaySegment map[string]any) {
	if rt == nil || len(base) == 0 {
		return
	}
	merged := make(map[string]any, len(base))
	for k, v := range base {
		merged[k] = v
	}
	if len(etcdGatewaySegment) > 0 {
		var baseGateway map[string]any
		if v, ok := merged["gateway"].(map[string]any); ok {
			baseGateway = v
		}
		mergedGateway := make(map[string]any, len(baseGateway))
		for k, v := range baseGateway {
			mergedGateway[k] = v
		}
		commonconfig.MergeMap(mergedGateway, etcdGatewaySegment)
		merged["gateway"] = mergedGateway
	}
	ApplyGatewayConfig(rt, config.GatewayFromMap(merged))
}

