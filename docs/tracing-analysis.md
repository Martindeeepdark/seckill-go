# 全链路追踪设计分析与对比

## 当前 Go 实现现状

### 1. TraceId 生成与透传机制

#### HTTP 层（Gateway）

**已实现：**
- ✅ `TraceMiddleware` 在 HTTP 入口自动生成/提取 traceId
- ✅ 支持多种格式：`X-Trace-Id`、W3C `traceparent`、B3 `X-B3-TraceId`
- ✅ 自动写入响应头返回给客户端
- ✅ 集成 OpenTelemetry，创建 span 并传播
- ✅ 优先级在鉴权和限流之前（保证被拒绝的请求也有 traceId）

**位置：** `services/seckill-gateway/internal/server/http.go:99-118`

```go
func TraceMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        incomingTraceID := tracing.TraceIDFromCarrier(c.GetHeader)
        ctx, span, traceID := tracing.StartSpan(c.Request.Context(), 
            "HTTP "+c.Request.Method+" "+c.Request.URL.Path, incomingTraceID)
        c.Request = c.Request.WithContext(ctx)
        c.Writer.Header().Set(tracing.HeaderTraceID, traceID)
        c.Writer.Header().Set(tracing.HeaderRequestID, traceID)
        if traceParent := tracing.TraceParent(ctx); traceParent != "" {
            c.Writer.Header().Set(tracing.HeaderTraceParent, traceParent)
        }
        c.Set(tracing.TraceIDKey, traceID)
        c.Next()
        tracing.EndSpan(span, err)
    }
}
```

#### gRPC 层（跨服务）

**已实现：**
- ✅ `TraceUnaryClientInterceptor` 在客户端自动注入 traceId 到 gRPC metadata
- ✅ `TraceUnaryServerInterceptor` 在服务端自动提取 traceId
- ✅ 创建 span 并传播上下文
- ✅ 支持 W3C traceparent 透传

**位置：** `common/interceptor/grpc.go:15-56`

```go
// Client端
func TraceUnaryClientInterceptor() grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, ...) error {
        ctx, span, traceID := tracing.StartSpan(ctx, "gRPC "+method, "")
        ctx = metadata.AppendToOutgoingContext(
            ctx,
            tracing.MetadataTraceID, traceID,
            tracing.MetadataRequestID, traceID,
        )
        if traceParent := tracing.TraceParent(ctx); traceParent != "" {
            ctx = metadata.AppendToOutgoingContext(ctx, tracing.HeaderTraceParent, traceParent)
        }
        err := invoker(ctx, method, req, reply, cc, opts...)
        tracing.EndSpan(span, err)
        return err
    }
}

// Provider端
func TraceUnaryServerInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        ctx, traceID := contextFromIncomingMetadata(ctx)
        ctx, span, _ := tracing.StartSpan(ctx, "gRPC "+info.FullMethod, traceID)
        resp, err := handler(ctx, req)
        tracing.EndSpan(span, err)
        return resp, err
    }
}
```

#### MQ 层（消息队列）

**已实现：**
- ✅ traceId 作为消息体字段 `SeckillMessage.RequestTraceID`
- ✅ Processor 消费时从消息体提取并恢复上下文
- ✅ NATS 消息头同时携带 `X-Trace-Id`（测试代码中）

**位置：** 
- 消息定义：`services/seckill-processor/internal/domain/model/seckill.go`
- 消费处理：`services/seckill-processor/internal/application/seckill.go:121-137`

```go
type SeckillMessage struct {
    TraceID        string `json:"traceId"`        // 秒杀请求的唯一追踪 ID
    RequestTraceID string `json:"requestTraceId"` // 原始请求追踪 ID
    // ... 其他字段
}

func (app *SeckillApp) HandleSeckill(ctx context.Context, message model.SeckillMessage) error {
    ctx, span, requestTraceID := tracing.StartSpan(ctx, "seckill.processor.process", message.RequestTraceID)
    defer tracing.EndSpan(span, spanErr)
    
    if message.RequestTraceID == "" {
        message.RequestTraceID = requestTraceID
    }
    // ... 处理逻辑
}
```

### 2. 日志自动注入

#### 当前实现

**基础设施：**
- ✅ 使用 `context.Context` 携带 traceId
- ✅ `tracing.TraceID(ctx)` 随时可从上下文提取
- ✅ `RequestLogger` 中间件自动记录 HTTP 请求日志并附带 traceId

**位置：** `services/seckill-gateway/internal/server/http.go:84-97`

```go
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()
        logger.Info("http request",
            "traceId", tracing.TraceID(c.Request.Context()),
            "method", c.Request.Method,
            "path", c.Request.URL.Path,
            "status", c.Writer.Status(),
            "elapsed", time.Since(start).String(),
        )
    }
}
```

**业务日志示例：**

```go
// 当前代码中的日志写法
logger.Info("worker pool starting", 
    "workerCount", p.config.WorkerCount,
    "queueSize", p.config.QueueSize)

logger.Warn("risk check failed, allowing by default", "error", err)
```

### 3. 与 Java 参考实现的对比

| 维度 | Java 实现 | Go 当前实现 | 差距分析 |
|-----|----------|------------|---------|
| **Gateway 层生成** | ✅ TraceGatewayFilter | ✅ TraceMiddleware | 功能对等 |
| **HTTP 透传** | ✅ TraceFilter + MDC | ✅ TraceMiddleware + Context | Go 用 context 替代 MDC |
| **gRPC 透传** | ✅ TraceDubboFilter + attachment | ✅ TraceUnaryInterceptor + metadata | 功能对等 |
| **MQ 透传** | ✅ 消息体字段 | ✅ 消息体字段 + header | Go 实现更完整 |
| **自动日志注入** | ✅ StructuredLog.buildLogContent | ❌ **手动传递** | **有差距** |
| **线程池传递** | ✅ TransmittableThreadLocal | ✅ Context 天然传递 | Go 天然优势 |
| **清理时机** | ✅ finally 块 | ✅ defer + span.End | 功能对等 |

### 4. 核心差距：StructuredLog 自动注入

#### Java 实现的优势

```java
// Java 代码：开发人员无需关心 traceId
StructuredLog.info("订单创建完成")
    .put("userId", userId)
    .put("orderNo", orderNo)
    .build();

// 自动输出：
// INFO 订单创建完成 || {"traceId":"a1b2c3d4","userId":10001,"orderNo":"SK001"}
```

**自动注入机制：**
```java
private String buildLogContent() {
    // 自动从 TraceContext 读取 traceId
    String traceId = TraceContext.getTraceId();
    if (traceId != null && !traceId.isEmpty() && !fields.containsKey("traceId")) {
        fields.put("traceId", traceId);
    }
    
    StringBuilder sb = new StringBuilder();
    if (message != null && !message.isEmpty()) {
        sb.append(message).append(" || ");
    }
    sb.append(MAPPER.writeValueAsString(fields));
    return sb.toString();
}
```

#### Go 当前实现

```go
// Go 代码：需要手动传递 traceId
logger.Info("http request",
    "traceId", tracing.TraceID(c.Request.Context()), // 手动添加
    "method", c.Request.Method,
    "path", c.Request.URL.Path,
)
```

**问题：**
1. 每个日志调用都需要手动添加 `"traceId", tracing.TraceID(ctx)`
2. 容易遗漏，导致部分日志无法关联
3. 代码冗余，不够优雅

## 改进方案

### 方案 1：slog.Handler 包装器（推荐）

创建自定义 `slog.Handler`，自动从 context 提取 traceId 并注入到日志中。

```go
// common/logger/trace_handler.go
package logger

import (
    "context"
    "log/slog"
    
    "seckill-common/tracing"
)

// TraceHandler 包装 slog.Handler，自动注入 traceId
type TraceHandler struct {
    handler slog.Handler
}

func NewTraceHandler(h slog.Handler) *TraceHandler {
    return &TraceHandler{handler: h}
}

func (h *TraceHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.handler.Enabled(ctx, level)
}

func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
    // 自动注入 traceId（如果存在）
    if traceID := tracing.TraceID(ctx); traceID != "" {
        r.Add("traceId", slog.StringValue(traceID))
    }
    return h.handler.Handle(ctx, r)
}

func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &TraceHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *TraceHandler) WithGroup(name string) slog.Handler {
    return &TraceHandler{handler: h.handler.WithGroup(name)}
}
```

**使用方式：**

```go
// common/logger/logger.go
func New(cfg config.Config) *slog.Logger {
    level := toCommonLogLevel(cfg.LogLevel)
    if err := commonlogs.Init(level); err != nil {
        slog.Default().Warn("common logger init failed", "error", err)
    }
    commonlogs.L().Infof("starting %s", cfg.ServiceName)
    
    // 包装 JSON handler
    jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})
    traceHandler := NewTraceHandler(jsonHandler)
    
    return slog.New(traceHandler)
}
```

**业务代码改动：**

```go
// 之前：需要手动添加 traceId
logger.Info("http request",
    "traceId", tracing.TraceID(c.Request.Context()),
    "method", c.Request.Method,
    "path", c.Request.URL.Path,
)

// 之后：自动注入，只需要传 context
logger.InfoContext(c.Request.Context(), "http request",
    "method", c.Request.Method,
    "path", c.Request.URL.Path,
)
```

**优点：**
- ✅ 完全自动化，开发人员无需关心 traceId
- ✅ 与 Java StructuredLog 功能对等
- ✅ 标准库方案，不引入额外依赖
- ✅ 不会遗漏 traceId

**缺点：**
- ⚠️ 需要将所有 `logger.Info()` 改为 `logger.InfoContext(ctx, ...)`
- ⚠️ 对于没有 context 的地方无法自动注入（但可以回退到手动添加）

### 方案 2：Context-aware Logger 包装

创建持有 context 的 logger 包装器。

```go
// common/logger/context_logger.go
package logger

import (
    "context"
    "log/slog"
    
    "seckill-common/tracing"
)

// ContextLogger 持有 context 的 logger
type ContextLogger struct {
    ctx    context.Context
    logger *slog.Logger
}

func WithContext(ctx context.Context, logger *slog.Logger) *ContextLogger {
    return &ContextLogger{ctx: ctx, logger: logger}
}

func (l *ContextLogger) Info(msg string, args ...any) {
    args = l.appendTraceID(args)
    l.logger.InfoContext(l.ctx, msg, args...)
}

func (l *ContextLogger) Warn(msg string, args ...any) {
    args = l.appendTraceID(args)
    l.logger.WarnContext(l.ctx, msg, args...)
}

func (l *ContextLogger) Error(msg string, args ...any) {
    args = l.appendTraceID(args)
    l.logger.ErrorContext(l.ctx, msg, args...)
}

func (l *ContextLogger) appendTraceID(args []any) []any {
    if traceID := tracing.TraceID(l.ctx); traceID != "" {
        // 检查是否已经有 traceId
        for i := 0; i < len(args)-1; i += 2 {
            if key, ok := args[i].(string); ok && key == "traceId" {
                return args // 已有 traceId，不重复添加
            }
        }
        return append([]any{"traceId", traceID}, args...)
    }
    return args
}
```

**使用方式：**

```go
// HTTP handler 中
func (h *SeckillHandler) partIn(c *gin.Context) {
    log := logger.WithContext(c.Request.Context(), h.logger)
    
    // 自动注入 traceId
    log.Info("processing seckill request", 
        "userId", uid, 
        "activityNo", req.ActivityNo)
}
```

**优点：**
- ✅ API 简洁，不改变现有日志调用方式
- ✅ 只需在 handler 入口创建一次

**缺点：**
- ⚠️ 需要显式创建 ContextLogger
- ⚠️ 传递 logger 参数的地方需要改用 ContextLogger

### 方案 3：Middleware 注入 Logger

在 Gin 中间件中创建带 traceId 的 logger 并存入 context。

```go
// services/seckill-gateway/internal/server/trace_logger.go
package server

import (
    "log/slog"
    
    "github.com/gin-gonic/gin"
    
    "seckill-common/tracing"
)

const loggerContextKey = "logger"

// TraceLoggerMiddleware 创建带 traceId 的 logger 并注入 context
func TraceLoggerMiddleware(baseLogger *slog.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        traceID := tracing.TraceID(c.Request.Context())
        // 创建带 traceId 的子 logger
        logger := baseLogger.With("traceId", traceID)
        c.Set(loggerContextKey, logger)
        c.Next()
    }
}

// GetLogger 从 context 获取带 traceId 的 logger
func GetLogger(c *gin.Context) *slog.Logger {
    if logger, exists := c.Get(loggerContextKey); exists {
        if l, ok := logger.(*slog.Logger); ok {
            return l
        }
    }
    return slog.Default()
}
```

**使用方式：**

```go
// main.go 中注册中间件
srv := server.NewHTTPServer(cfg.HTTPAddr, log, middlewares...)
srv.engine.Use(server.TraceLoggerMiddleware(log))

// handler 中使用
func (h *SeckillHandler) partIn(c *gin.Context) {
    log := server.GetLogger(c)
    
    // 自动包含 traceId
    log.Info("processing seckill request", 
        "userId", uid, 
        "activityNo", req.ActivityNo)
}
```

**优点：**
- ✅ 对业务代码侵入最小
- ✅ 只在 HTTP 层有效（Gin specific）

**缺点：**
- ⚠️ 只适用于 HTTP handler，gRPC 和其他层需要其他方案
- ⚠️ 依赖 Gin context

## 推荐方案

**综合推荐：方案 1（slog.Handler 包装器）**

理由：
1. **标准化**：基于 Go 标准库 `slog`，不引入额外复杂度
2. **全局生效**：适用于所有场景（HTTP、gRPC、MQ、定时任务）
3. **功能对等**：与 Java StructuredLog 实现相同效果
4. **符合 Go 哲学**：使用 `context.Context` 传递元数据是 Go 的惯用法

**迁移路径：**

1. **Phase 1**：实现 TraceHandler 并在 `common/logger` 中启用
2. **Phase 2**：逐步将关键路径的日志改为 `InfoContext`/`WarnContext`/`ErrorContext`
   - Gateway HTTP handlers
   - Processor 消息处理
   - 关键业务逻辑
3. **Phase 3**：在代码审查中强制要求新代码使用 `*Context` 方法
4. **Phase 4**：批量重构剩余日志调用

## 日志规范（对标 Java 实现）

### 规范要求

| 规范项 | Java 实现 | Go 实现建议 |
|--------|----------|------------|
| **message 字段** | 简明扼要，不含变量 | 同 Java |
| **键值对字段** | 通过 `.put()` 添加 | `slog` 的 key-value 参数 |
| **关键业务标识** | userId、orderNo、activityNo、traceId | 同 Java，traceId 自动注入 |
| **异常传递** | `.exception(e)` | `"error", err` 或 `slog.Any("error", err)` |
| **日志级别** | INFO/WARN/ERROR/DEBUG | 同 Java |

### 示例对比

**Java:**
```java
StructuredLog.info("订单创建完成")
    .put("userId", userId)
    .put("orderNo", orderNo)
    .put("activityNo", activityNo)
    .build();

// 输出：INFO 订单创建完成 || {"traceId":"a1b2c3d4","userId":10001,"orderNo":"SK001","activityNo":"ACT001"}
```

**Go (改进后):**
```go
logger.InfoContext(ctx, "订单创建完成",
    "userId", userId,
    "orderNo", orderNo,
    "activityNo", activityNo,
)

// 输出：{"time":"2026-06-21T20:00:00Z","level":"INFO","msg":"订单创建完成","traceId":"a1b2c3d4","userId":10001,"orderNo":"SK001","activityNo":"ACT001"}
```

## 生产问题排查流程（与 Java 对齐）

### 1. 获取 traceId

**已支持：**
- ✅ 用户反馈：从订单号检索日志 → 提取 traceId
- ✅ 监控告警：告警信息包含采样的 traceId
- ✅ userId + 时间范围模糊检索（JSON 格式支持）

### 2. 常见场景排查路径

**场景一：用户点击抢购没响应**
```bash
# 使用 traceId 在 ELK/Loki 检索
traceId:a1b2c3d4

# 分析日志停在哪一层
- 只有 Gateway 日志 → 鉴权拒绝或限流
- seckill-gateway 成功入队但 processor 无日志 → MQ 问题
- processor 有消费但订单创建失败 → 检查错误日志
```

**场景二：下单成功但没结果**
```bash
# 检索 processor 日志
traceId:a1b2c3d4 AND service:seckill-processor

# 分析处理流程
- 幂等校验拦截 → 检查 Redis key
- 库存扣减成功但订单创建异常 → 检查 gRPC 调用
- 订单创建成功但前端轮询失败 → 检查 Redis result key
```

### 3. 告警规则（基于结构化日志）

```yaml
# Prometheus AlertManager 配置示例
- alert: SeckillOrderCreationFailed
  expr: |
    sum(rate(logs{service="seckill-processor",msg=~".*订单创建异常.*"}[1m])) > 10
  labels:
    severity: P1
  annotations:
    summary: "秒杀订单创建异常率过高"

- alert: PaymentCallbackTimeout
  expr: |
    sum(rate(logs{service="seckill-processor",msg=~".*支付回调获取锁超时.*"}[1m])) > 5
  labels:
    severity: P1

- alert: FreeCardIssueFailed
  expr: |
    sum(rate(logs{service="support-service",msg=~".*支付后发卡失败.*"}[1m])) > 20
  labels:
    severity: P2
```

## 总结

### 当前优势

1. **基础设施完善**：traceId 生成、传播、上下文管理都已实现
2. **多协议支持**：HTTP、gRPC、MQ 都有自动透传
3. **OpenTelemetry 集成**：与标准 tracing 体系无缝集成
4. **Go 天然优势**：context 传递比 ThreadLocal 更优雅

### 核心差距

1. **日志自动注入**：需要手动添加 traceId，容易遗漏
2. **规范执行**：缺少像 Java StructuredLog 那样的强制规范

### 改进优先级

| 优先级 | 改进项 | 预计工作量 | 价值 |
|--------|--------|----------|------|
| **P0** | 实现 TraceHandler 自动注入 | 2小时 | ⭐⭐⭐⭐⭐ |
| **P1** | 改造 Gateway handlers 使用 InfoContext | 4小时 | ⭐⭐⭐⭐ |
| **P1** | 改造 Processor 使用 InfoContext | 4小时 | ⭐⭐⭐⭐ |
| **P2** | 添加日志规范文档和 linter 检查 | 2小时 | ⭐⭐⭐ |
| **P2** | 配置告警规则（基于结构化日志字段） | 4小时 | ⭐⭐⭐⭐ |

### 最终效果

完成改进后，Go 实现将与 Java 参考实现功能对等：

- ✅ traceId 全链路自动透传
- ✅ 日志自动注入，无需手动添加
- ✅ 结构化日志支持精确检索
- ✅ 完整的监控告警体系
- ✅ 生产问题 5 分钟内定位根因
