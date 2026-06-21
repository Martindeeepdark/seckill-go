// Package handler 提供 HTTP 请求处理器
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"seckill-common/tracing"
)

// Result 是标准 JSON 响应信封
type Result struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Data           any    `json:"data,omitempty"`
	RequestTraceID string `json:"requestTraceId,omitempty"`
}

// ok 返回成功响应
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Result{
		Code:           "success",
		Message:        "操作成功",
		Data:           data,
		RequestTraceID: tracing.TraceID(c.Request.Context()),
	})
}

// fail 返回失败响应
func fail(c *gin.Context, status int, code string, message string) {
	c.JSON(status, Result{
		Code:           code,
		Message:        message,
		RequestTraceID: tracing.TraceID(c.Request.Context()),
	})
}

// failRPC 返回 RPC 错误响应
func failRPC(c *gin.Context, err error) {
	if err == nil {
		fail(c, http.StatusInternalServerError, "rpc_error", "内部服务异常")
		return
	}
	if handleRPCFailure(c, err) {
		return
	}
	fail(c, http.StatusInternalServerError, "rpc_error", err.Error())
}

// handleRPCFailure 处理 RPC 失败，转换为 HTTP 响应
func handleRPCFailure(c *gin.Context, err error) bool {
	mapped, ok := rpcFailure(err)
	if !ok {
		return false
	}
	fail(c, mapped.status, mapped.code, mapped.message)
	return true
}

// rpcFailureResult RPC 失败结果
type rpcFailureResult struct {
	status  int
	code    string
	message string
}

// rpcFailure 将 RPC 错误映射为 HTTP 响应
func rpcFailure(err error) (rpcFailureResult, bool) {
	if err == nil {
		return rpcFailureResult{}, false
	}
	if failure, ok := kratosFailure(err); ok {
		return failure, true
	}
	if failure, ok := grpcFailure(err); ok {
		return failure, true
	}
	return rpcFailureResult{}, false
}

// kratosFailure 处理 Kratos 框架错误
func kratosFailure(err error) (rpcFailureResult, bool) {
	switch {
	case kratoserrors.Code(err) == http.StatusTooManyRequests:
		return rpcFailureResult{status: http.StatusTooManyRequests, code: "rate_limited", message: "请求过于频繁，请稍后再试"}, true
	case kratoserrors.IsServiceUnavailable(err):
		return rpcFailureResult{status: http.StatusServiceUnavailable, code: "service_degraded", message: "服务繁忙，请稍后再试"}, true
	case kratoserrors.IsGatewayTimeout(err):
		return rpcFailureResult{status: http.StatusGatewayTimeout, code: "service_timeout", message: "服务响应超时，请稍后再试"}, true
	default:
		return rpcFailureResult{}, false
	}
}

// grpcFailure 处理 gRPC 错误
func grpcFailure(err error) (rpcFailureResult, bool) {
	switch status.Code(err) {
	case codes.ResourceExhausted:
		return rpcFailureResult{status: http.StatusTooManyRequests, code: "rate_limited", message: "请求过于频繁，请稍后再试"}, true
	case codes.Unavailable:
		return rpcFailureResult{status: http.StatusServiceUnavailable, code: "service_degraded", message: "服务繁忙，请稍后再试"}, true
	case codes.DeadlineExceeded:
		return rpcFailureResult{status: http.StatusGatewayTimeout, code: "service_timeout", message: "服务响应超时，请稍后再试"}, true
	default:
		return rpcFailureResult{}, false
	}
}
