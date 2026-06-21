package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"seckill-gateway-service/internal/server"
)

// ConfigSource 抽象 etcd 配置源，便于测试和 nil 安全。
type ConfigSource interface {
	Put(ctx context.Context, serviceName string, cfg map[string]any) error
	Delete(ctx context.Context, serviceName string) error
}

// ConfigHandler 处理动态配置的 HTTP 路由。
type ConfigHandler struct {
	runtime     *server.GatewayRuntimeConfig
	source      ConfigSource
	serviceName string
}

// NewConfigHandler 创建配置处理器。
// source 为 nil 时，PUT/DELETE 直接更新 RuntimeConfig（测试或无 etcd 场景）。
func NewConfigHandler(runtime *server.GatewayRuntimeConfig, source ConfigSource) *ConfigHandler {
	return &ConfigHandler{
		runtime:     runtime,
		source:      source,
		serviceName: "seckill-gateway",
	}
}

// Register 在 gin 路由组上注册 config 路由。
func (h *ConfigHandler) Register(group *gin.RouterGroup) {
	group.GET("/config", h.get)
	group.PUT("/config", h.put)
	group.DELETE("/config", h.delete)
}

// get 返回当前动态配置快照。
func (h *ConfigHandler) get(c *gin.Context) {
	snap := h.runtime.Snapshot()
	data := gin.H{
		"riskEnabled":         snap.RiskEnabled,
		"machineCheckEnabled": snap.MachineCheckEnabled,
		"rateLimit": gin.H{
			"enabled": snap.RateLimit.Enabled,
			"maxQPS":  snap.RateLimit.Options.MaxQPS,
		},
		"degrade": gin.H{
			"enabled":          snap.Degrade.Enabled,
			"failureThreshold": snap.Degrade.FailureThreshold,
		},
	}
	ok(c, data)
}

// put 写入 etcd 配置；无 etcd 时直接更新 RuntimeConfig。
func (h *ConfigHandler) put(c *gin.Context) {
	var cfg map[string]any
	if err := c.ShouldBindJSON(&cfg); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求参数错误")
		return
	}

	if h.source != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		if err := h.source.Put(ctx, h.serviceName, cfg); err != nil {
			failRPC(c, err)
			return
		}
	} else {
		applyConfigToRuntime(h.runtime, cfg)
	}
	ok(c, gin.H{"updated": true})
}

// delete 删除 etcd 配置（触发回退）；无 etcd 时无操作。
func (h *ConfigHandler) delete(c *gin.Context) {
	if h.source != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		if err := h.source.Delete(ctx, h.serviceName); err != nil {
			failRPC(c, err)
			return
		}
	}
	ok(c, gin.H{"deleted": true})
}

// applyConfigToRuntime 直接从原始 map 更新 RuntimeConfig（无 etcd 场景）。
func applyConfigToRuntime(rc *server.GatewayRuntimeConfig, raw map[string]any) {
	if v, ok := raw["risk"].(map[string]any); ok {
		if enabled, ok := v["enabled"].(bool); ok {
			rc.UpdateRisk(enabled)
		}
	}
	if v, ok := raw["machine_check"].(map[string]any); ok {
		if enabled, ok := v["enabled"].(bool); ok {
			rc.UpdateMachineCheck(enabled)
		}
	}
	if v, ok := raw["rate_limit"].(map[string]any); ok {
		enabled, _ := v["enabled"].(bool)
		maxQPS, _ := v["max_qps"].(float64)
		rc.UpdateRateLimit(enabled, server.RateLimitOptions{
			MaxQPS: int(maxQPS),
		})
	}
	if v, ok := raw["degrade"].(map[string]any); ok {
		enabled, _ := v["enabled"].(bool)
		threshold, _ := v["failure_threshold"].(float64)
		rc.UpdateDegrade(server.DegradeOptions{
			Enabled:          enabled,
			FailureThreshold: int(threshold),
		})
	}
}
