// Package server 提供 HTTP 服务器和中间件
package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"seckill-common/tracing"
)

const adminRoleHeader = "X-User-Role"

// adminErrorResponse 是 admin 鉴权中间件的错误响应结构。
// 字段命名与 handler.Result 对齐，确保客户端无感知。
// 不复用 handler 包是为了避免 server <-> handler 循环依赖。
type adminErrorResponse struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Success        bool   `json:"success"`
	RequestTraceID string `json:"requestTraceId,omitempty"`
}

func writeAdminError(c *gin.Context, status int, code, message string) {
	c.JSON(status, adminErrorResponse{
		Code:           code,
		Message:        message,
		Success:        false,
		RequestTraceID: tracing.TraceID(c.Request.Context()),
	})
	c.Abort()
}

// RequireAdmin 校验 X-User-Role header 是否包含 admin 角色。
// 与现有 X-User-Id 模式一致：依赖上游 auth proxy 注入 header，
// gateway 不做 JWT 自校验。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := strings.TrimSpace(c.GetHeader(adminRoleHeader))
		if role == "" {
			writeAdminError(c, http.StatusUnauthorized, "not_admin", "缺少管理员角色信息")
			return
		}
		if !isAdminRole(role) {
			writeAdminError(c, http.StatusForbidden, "forbidden", "需要管理员权限")
			return
		}
		c.Next()
	}
}

// isAdminRole 判断 role 值是否为管理员。
// 支持单值（"admin"）和逗号分隔多角色（"operator,admin"）。
// 大小写不敏感以兼容不同上游约定。
func isAdminRole(role string) bool {
	for _, r := range strings.Split(role, ",") {
		if strings.EqualFold(strings.TrimSpace(r), "admin") {
			return true
		}
	}
	return false
}
