// Package server 提供 HTTP 服务器和中间件
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	commonlogs "github.com/Martindeeepdark/go-common/logs"

	"seckill-common/tracing"
)

// Registrar 在 gin Engine 上注册路由
type Registrar interface {
	Register(router *gin.Engine)
}

// HTTPServer 封装基于 gin 的 HTTP 服务器，支持优雅关闭
type HTTPServer struct {
	engine *gin.Engine
	server *http.Server
}

// NewHTTPServer 创建新的 HTTP 服务器，包含链路追踪和日志中间件
func NewHTTPServer(addr string, _ any, middlewares ...gin.HandlerFunc) *HTTPServer {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(TraceMiddleware())
	engine.Use(RequestLogger())
	engine.Use(middlewares...)
	return &HTTPServer{
		engine: engine,
		server: &http.Server{
			Addr:              addr,
			Handler:           engine,
			ReadHeaderTimeout: 3 * time.Second,
		},
	}
}

// Register 添加路由
func (s *HTTPServer) Register(registrar Registrar) {
	registrar.Register(s.engine)
}

// Start 启动 HTTP 服务器并阻塞直到关闭信号
func (s *HTTPServer) Start() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	errCh := make(chan error, 1)
	go func() {
		commonlogs.Infof("http server listening on %s", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return err
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		return nil
	}
}

// RequestLogger 记录每个 HTTP 请求
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		ctx := c.Request.Context()
		commonlogs.CtxInfof(ctx, "http request method=%s path=%s status=%d elapsed=%s",
			c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start).String())
	}
}

// TraceMiddleware 将链路追踪 ID 注入请求上下文
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		incomingTraceID := tracing.TraceIDFromCarrier(c.GetHeader)
		ctx, span, traceID := tracing.StartSpan(c.Request.Context(), "HTTP "+c.Request.Method+" "+c.Request.URL.Path, incomingTraceID)
		c.Request = c.Request.WithContext(ctx)
		c.Writer.Header().Set(tracing.HeaderTraceID, traceID)
		c.Writer.Header().Set(tracing.HeaderRequestID, traceID)
		if traceParent := tracing.TraceParent(ctx); traceParent != "" {
			c.Writer.Header().Set(tracing.HeaderTraceParent, traceParent)
		}
		c.Set(tracing.TraceIDKey, traceID)
		c.Next()
		if len(c.Errors) > 0 {
			tracing.EndSpan(span, c.Errors.Last())
			return
		}
		tracing.EndSpan(span, nil)
	}
}
