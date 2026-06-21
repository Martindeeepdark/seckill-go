# Go 秒杀微服务

参考 Java 秒杀链路，按 DDD 和微服务边界拆分的 Go 实现。外部 HTTP 使用 Gin，内部服务调用使用 Kratos gRPC，基础能力优先复用 `/Users/wenyz/GolandProjects/common`。

## 服务拆分

- `activity-service`：活动和 SKU 查询 gRPC
- `stock-service`：库存快照、扣减、释放 gRPC
- `risk-service`：小黑屋、风控记录、综合风险评估 gRPC
- `order-service`：订单创建和查询 gRPC
- `support-service`：会员查询、支付预下单、查单、关单、自由卡生命周期、主域订单同步 gRPC，当前支付网关为模拟实现
- `seckill-gateway`：对外 Gin HTTP 网关，负责接收/生成 `X-Trace-Id`、JWT 鉴权、机审、限流、编排、入队和管理后台 HTTP
- `seckill-api`：兼容旧启动名，内部复用 `seckill-gateway` 启动逻辑
- `seckill-processor`：消费 NATS JetStream 主秒杀队列，二次校验、扣库存、创建订单、写回排队结果，并处理 Redis 补偿/延迟任务
- `seckill-job`：对应 Java `seckill-job`，定时扫描活动状态和已结束活动的待支付订单，内部通过 gRPC 调各领域服务

## 架构约定

- `cmd/main.go` 是唯一 `main`，所有服务通过 urfave/cli 子命令启动。
- `internal/domain/seckill` 放领域实体和仓储端口。
- `internal/application/seckill` 放应用用例和业务编排。
- `internal/server` 放 Gin HTTP 与 Kratos gRPC server。
- `infrastructure` 放 Redis、队列、RPC client/server、common 适配和本地适配。
- `api/<service>/v1/*.proto` 是内部 gRPC 契约源，每个领域服务一个 proto package，`api/common/v1/common.proto` 只放跨服务基础请求/响应类型，`make api` 生成 `*.pb.go` 和 `*_grpc.pb.go` 桩代码。
- `configs` 使用 `github.com/Martindeeepdark/go-common/config` 加载 YAML。
- 日志、Snowflake ID、Redis 基础能力复用 `go-common`。
- 防腐层和未来数据库 repository 默认使用简单 SQL：按主键、外键、状态、时间范围查询并配索引；列表场景优先提供批量端口，例如 `ListOrdersByActivities` 对应 `WHERE activity_no IN (...)`，避免上层循环单查造成 N+1，也避免用复杂 join 拼领域对象。
- gRPC client 使用 Kratos discovery resolver、`rpc.timeout` 超时和 circuit breaker middleware；服务端启动时注册到 Redis registry，客户端优先发现 Redis 实例，只有显式开启 `discovery.static_fallback` 时才回退 `discovery.services` 静态表。
- Gin gateway 内置配置化 QPS 保护，对应 Java Sentinel gateway flow rule 的职责，触发后返回 HTTP 429 和统一 JSON。
- Gin gateway 内置 JWT 鉴权中间件，对应 Java `AuthGatewayFilter` 的职责；开启后活动/商品公开查询白名单放行，其余接口校验 `Authorization: Bearer <jwt>`，并移除客户端伪造的 `X-User-Id` 后注入可信用户 ID。
- `X-Trace-Id` 是请求链路追踪 ID，HTTP 响应和 gRPC metadata 同时透出 W3C `traceparent`，从 `seckill-gateway` 进入后透传到 gRPC metadata 和异步消息；NATS JetStream/Redis List/ZSet 消费侧会从消息里的 `requestTraceId` 恢复 context 再调下游 RPC。统一 JSON 响应里的 `requestTraceId` 是网关链路 ID，`data.traceId` 是秒杀排队/订单轮询的业务标识。
- 活动和 SKU 元数据使用本地 L1 + Redis L2 两级缓存，对应 Java 的 Caffeine + Redis 查询职责；库存仍由 `stock-service` 通过 Redis/RPC 实时处理。
- `risk-service` 承担 Java `RiskEvaluateService`/`RiskDubboService` 的业务职责：小黑屋、风控记录、最近高风险记录、最近行为次数阈值。
- 管理后台活动接口对应 Java `seckill-admin`，HTTP 入口在 Gin gateway，活动/商品写操作仍通过 activity-service gRPC 完成。
- 主秒杀请求队列默认使用 NATS JetStream，对应 Java RocketMQ 在秒杀异步削峰里的职责；Redis Stream 仅保留为显式开启的本地 fallback，不再作为默认主 MQ。
- 预支付结果写 Redis 缓存，支付回调使用 Redis 分布式锁防止同一订单并发更新；订单同步/自由卡发放失败时写入 Redis List 补偿队列，由 `seckill-processor` 重试，超过上限进入 dead-letter list。
- 秒杀订单创建成功后写支付超时延迟任务，到期仍待支付则关闭订单、释放库存和限购计数、关闭支付单。
- `seckill-job` 使用 Go 定时循环替代 Java XXL-Job，补偿任务不直连业务存储，统一走 activity/order/stock/support gRPC。

## 消息队列

主秒杀削峰链路默认使用 NATS JetStream，gateway 发布 `seckill.orders`，processor 以 `consumer_group` 作为 durable consumer 消费。消费处理异常会按 `redelivery_delay` 触发 `Nak` 重投，达到 `max_deliver` 后写入 `dead_letter_subject` 并终止该消息，避免无限重试；Redis Stream 配置仍保留为显式 fallback。支付超时延迟任务继续用 Redis ZSet 表达到期时间。

```yaml
queue:
  provider: nats
  redis_fallback: false
  stream: seckill:stream:orders
  consumer_group: seckill-processor
  nats:
    url: nats://127.0.0.1:4222
    stream: SECKILL_ORDERS
    subject: seckill.orders
    dead_letter_subject: seckill.orders.dlq
    max_deliver: 3
    ack_wait: 30s
    redelivery_delay: 1s
```

## 服务发现

Java 参考项目用 Nacos 承担 Spring Cloud/Dubbo 的注册发现职责。Go 版当前用 Redis 实现 Kratos `registry.Registrar` / `registry.Discovery`，不引入 Java 生态组件名：

```yaml
discovery:
  mode: redis
  namespace: seckill
  static_fallback: false
  ttl: 15s
  refresh_interval: 5s
  services:
    activity-service:
      - 127.0.0.1:9001
  advertise:
    activity-service: 127.0.0.1:9001
```

`services` 是显式开启 `static_fallback` 后使用的静态兜底地址，`advertise` 是服务注册到 Redis 时暴露给其他服务的地址。Docker 环境里这里写服务名，例如 `activity-service:9001`。

服务端启动后会把 gRPC 实例写入 Redis 并周期续约；续约间隔会自动限制在 TTL 以内，避免实例在下一次心跳前过期。默认情况下 Redis 注册/发现失败会让服务启动失败，避免生产环境静默退回单机静态地址。

## 内部 RPC 契约

内部 RPC 接口用 protobuf 管理，契约按服务拆分在 `api/<service>/v1` 下：

- `api/activity/v1/activity.proto`：活动与 SKU
- `api/stock/v1/stock.proto`：库存
- `api/risk/v1/risk.proto`：风控
- `api/order/v1/order.proto`：订单
- `api/payment/v1/payment.proto`：支付
- `api/member/v1/member.proto`：会员
- `api/free_card/v1/free_card.proto`：自由卡
- `api/order_sync/v1/order_sync.proto`：主域订单同步
- `api/common/v1/common.proto`：跨服务基础请求/响应类型

每个服务都有独立 protobuf package 和生成包，例如 `seckill.api.activity` 会生成到 `seckill/api/activity/v1`。

```bash
make init # 安装 protoc-gen-go / protoc-gen-go-grpc
make api  # 根据 api/**/*.proto 生成 pb 桩
```

`make api` 对齐 `/Users/wenyz/GolandProjects/kratos_template` 的生成方式，只生成内部 gRPC 需要的 `pb.go` 和 `grpc.pb.go`；Gin HTTP 入口仍由 gateway handler 管理。

## 网关保护

Java 参考项目的 `AuthGatewayFilter` 会先白名单放行活动/商品查询，再校验 Bearer JWT，并把解析出的 userId 注入下游请求头。Go 版对应配置在 `gateway.auth`；为了保留当前本地快速体验，示例配置默认关闭，生产或联调可打开：

```yaml
gateway:
  auth:
    enabled: true
    secret: default-jwt-secret-key-must-be-at-least-32-bytes
    white_list:
      - /healthz
      - /api/seckill/activity/**
      - /api/seckill/product/**
```

JWT 使用 HS256，`sub` 为用户 ID，`exp` 为过期时间；鉴权成功后 gateway 会删除原请求里的 `X-User-Id`，再写入 JWT 中的用户 ID，避免客户端伪造用户身份。

Java 参考项目用 Sentinel GatewayFlowRule 管理网关流控。Go 版在 Gin gateway 中用本地窗口限流实现同一类保护职责，规则在 `gateway.protection.rules`：

```yaml
gateway:
  protection:
    enabled: true
    rules:
      - resource: seckill-service
        path_prefixes:
          - /api/seckill
          - /api/pay
        count: 1000
        interval: 1s
```

`resource` 对应网关资源名，`path_prefixes` 决定哪些请求命中该规则，`count/interval` 表示窗口内允许的请求数。

## 风控评估

Java 参考项目的风控链路是 Redis 黑名单 -> 最近高风险记录 -> 最近 1 小时行为次数阈值。Go 版保持同一业务语义，但通过 `risk-service` 的 Kratos gRPC 暴露：

```yaml
seckill:
  risk:
    high_risk_threshold: 10
    risk_user_ttl: 24h
    recent_window: 1h
    high_risk_window: 24h
```

入口链路会记录 `MACHINE_CHECK_FAIL`、`RATE_LIMIT_HIT`、`SECKILL` 等风控行为；活动未开始前命中小黑屋窗口会记录 `PRE_CHECK` 并写 Redis 黑名单。`risk-service` 发现用户最近存在高风险记录或最近秒杀行为次数达到阈值后，会写入 `seckill:risk:user:<userId>`，后续请求直接拦截。`seckill-processor` 在异步扣库存前也会二次调用风控评估，避免旁路消息绕过入口。

## 活动两级缓存

Java 参考项目里活动/商品查询走 Caffeine + Redis，两级缓存未命中才回源 Dubbo。Go 版在 `infrastructure.CachedActivityGateway` 中实现同一职责：

```yaml
cache:
  activity:
    enabled: true
    max_size: 512
    local_ttl: 30m
    refresh_after: 5s
    redis_ttl: 30m
    null_ttl: 60s
    warmup_ahead: 10m
    refresh_enabled: true
    refresh_initial: 30s
    refresh_tick: 60s
```

查询链路是本地缓存 -> Redis -> Activity gRPC；本地缓存到 `refresh_after` 后会后台只查 Redis 刷新，Redis miss 或异常时保留旧值，不主动打回源。not found 会写短 TTL 的 `NULL` 标记防穿透。缓存 key 沿用 Java 语义，例如 `seckill:activity:info:<activityNo>` 和 `seckill:activity:product:list:<activityNo>`。

当前 L1 本地缓存已抽成 `activityLocalCache` 字节缓存接口，默认实现是进程内 TTL/容量受控缓存；后续如果允许拉取 `github.com/allegro/bigcache/v3`，只需要补一个本地 adapter，不再改活动查询链路本身。

`seckill-processor` 会周期性刷新 Redis L2：进行中活动会刷新活动详情、商品列表和活动列表缓存；`warmup_ahead` 窗口内即将开始的活动也会提前写入活动/SKU 缓存。SKU 库存只在 Redis key 不存在时写入初始库存，避免覆盖秒杀中的实时扣减值。

## 前台活动查询

Java `ActivityController` 的公开查询入口已落到 Go gateway，内部仍通过 Activity gRPC 读取活动缓存/服务：

```text
GET /api/seckill/activity/active
GET /api/seckill/activity/:activityNo
GET /api/seckill/product/:activityNo
```

`active` 只返回 `activityStatus = 1` 的活动列表，详情接口返回 Java 风格 `ActivityDetailVO`，包含 `activityOpen` 和商品列表。`/api/seckill/product/:activityNo` 对应 Java `ProductQueryService.listActivityProducts` 的公开读取能力，复用活动缓存和 Activity gRPC，不额外引入复杂查询。兼容早期调试的 `/api/activities` 与 `/api/activities/:activityNo` 仍保留，但它们返回领域实体快照，不作为 Java 对齐接口。

## 管理后台

Java `seckill-admin` 的活动管理接口已落到 Go gateway 的 `/api/admin/activity`，内部通过 Kratos gRPC 调 activity-service：

```text
GET    /api/admin/activity/list
GET    /api/admin/activity/:activityNo
POST   /api/admin/activity
PUT    /api/admin/activity
POST   /api/admin/activity/:activityNo/end
POST   /api/admin/activity/product
DELETE /api/admin/activity/product?activityNo=...&skuNo=...
```

管理接口支持创建/更新/结束活动、添加/移除活动 SKU。活动或商品变更成功后会主动驱逐活动两级缓存中的活动详情、商品列表和活动列表 key，避免 gateway 继续读取旧 L1/L2 缓存。

### Admin 鉴权

`/api/admin/*` 路由组挂载了 `server.RequireAdmin()` 中间件，读取请求头 `X-User-Role`：

- 缺失/空白 → `401 not_admin`
- 非管理员值 → `403 forbidden`
- 含 `admin`（大小写不敏感，支持逗号分隔多角色）→ 放行

**部署约束**：gateway 不做 JWT 自校验，完全信任上游 auth proxy 注入的 `X-User-Role`（与现有 `X-User-Id` 模式一致）。生产环境必须保证 gateway 仅监听内网，auth proxy 是唯一入口；若 gateway 直接对外暴露，攻击者可伪造 `X-User-Role: admin` 绕过鉴权。

## 支付后置补偿

Java 参考项目用 Redis 对账列表和 RocketMQ 处理支付成功后的订单同步。Go 版主秒杀 MQ 已使用 NATS JetStream；支付后置补偿暂用 Redis List 保存 post-pay task，processor 消费后通过 Kratos gRPC 调 `support-service`：

```yaml
support:
  prepay_cache:
    ttl: 5m
  post_pay:
    queue: seckill:support:postpay:list
    dead_letter: seckill:support:postpay:dead
    max_attempts: 5
    retry_delay: 1s
  callback_lock:
    ttl: 10s
  payment_timeout:
    queue: seckill:support:payment-timeout:zset
    dead_letter: seckill:support:payment-timeout:dead
    delay: 10m
    max_attempts: 5
    retry_delay: 1s
    poll_interval: 1s
```

这条链路承载两类任务：`SYNC_ORDER` 和 `ISSUE_CARD`。即时 RPC 成功时不会重复入队；失败时入队重试，support 侧仍按 `orderNo` 做幂等。

自由卡服务对齐 Java `FreeCardDubboService` 的核心生命周期 RPC：`IssueCard`、`GetCard`、`ListCards`、`ActivateCard`、`FreezeCard`、`UnfreezeCard`。发卡按 `orderNo` 幂等；激活会写激活时间和到期时间；冻结/解冻按未激活、已激活、已冻结、已过期的状态流转规则校验。

会员服务对齐 Java `MemberDubboService`，通过 `member.proto` 暴露 `GetUserByID`、`GetUserByPhone` 和 `GetMemberLevel`。当前 support-service 使用内存 ledger 承载会员查询，后续可替换为持久化仓储而不影响内部 RPC 契约。

支付超时任务使用 Redis sorted set 表达延迟时间，到期后 processor 通过 gRPC 查询/关闭支付单、关闭订单服务里的待支付订单，并调用库存服务释放库存。

## 定时补偿任务

Java `seckill-job` 中的核心补偿任务已落到 Go 的 `seckill-job` 子命令：

```yaml
job:
  run_on_start: true
  activity_status_check_interval: 1m
  timeout_order_check_interval: 10m
  stock_release_check_interval: 10m
  activity_data_cleanup_interval: 24h
  activity_data_retention: 24h
  risk_user_cleanup_interval: 24h
  daily_statistics_interval: 24h
  cache_warmup_interval: 5m
  cache_refresh_interval: 1m
  payment_reconcile_interval: 30m
  order_sync_check_interval: 5m
```

`activity_status_check_interval` 会扫描活动时间窗口：未开始活动到点变为进行中，进行中活动到结束时间变为已结束，并驱逐活动缓存。`timeout_order_check_interval` 会扫描已结束活动下仍为待支付的订单，通过 order-service 关闭订单，通过 stock-service 释放库存/限购计数，通过 support-service 关闭支付单。这是支付超时延迟任务的补偿路径，用来覆盖延迟消息丢失或消费失败。

`stock_release_check_interval` 对应 Java `StockReleaseTask`：活动结束后驱逐活动详情/商品列表缓存，并通过 stock-service 清理库存 key，避免结束活动长期占用 Redis。

`activity_data_cleanup_interval` 对应 Java `ActivityDataCleanupTask`：只处理状态为已结束且结束时间早于 `activity_data_retention` 的活动，通过 stock-service 的独立 gRPC 清理用户限购计数，保留活动结束后的补偿/排查窗口。

`risk_user_cleanup_interval` 对应 Java `RiskUserCleanupTask`：通过 risk-service 的独立 gRPC 清理已过期的风险用户标记，避免小黑屋临时 key 堆积。

`daily_statistics_interval` 对应 Java `DailyStatisticsTask`：统计进行中/已结束活动数量，并对已结束活动输出活动编号、名称、初始总库存和活动时间的结构化日志，供日志平台做活动看板。

`cache_warmup_interval` / `cache_refresh_interval` 对应 Java `CacheWarmupTask` / `CacheRefreshTask`：预热任务扫描进行中活动和 `cache.activity.warmup_ahead` 窗口内即将开始的活动，写入活动详情、商品列表并用 SetNX 初始化 SKU 库存；刷新任务只刷新进行中活动的活动详情、商品列表和活动列表缓存，不覆盖实时库存。

`payment_reconcile_interval` 对应 Java `PaymentReconcileTask`：扫描进行中/已结束活动的待支付和已支付订单，通过 support-service 查询支付网关状态；如果支付网关已支付但秒杀域仍是待支付，会通过 order-service 补偿更新支付状态。

`order_sync_check_interval` 对应 Java `OrderSyncCheckTask`：扫描已支付秒杀订单，通过 support-service 检查主域订单是否存在；缺失时用 order-sync gRPC 补偿同步。

## 动态配置和降级开关

Java 参考项目里的 Nacos 动态规则，在 Go 版中先落为文件配置热加载，不引入 Nacos 客户端。gateway、processor、job 启动后会按 `dynamic_config.refresh_interval` 重新加载配置文件，并通过内存快照让下一次 HTTP/RPC/支付回调读取新配置：

```yaml
dynamic_config:
  enabled: true
  refresh_interval: 5s
```

当前热加载覆盖五类运行期开关：`seckill.degrade.*` 秒杀入口降级，`seckill.degrade.skip_order_sync/skip_card_issue` 支付后置动作降级，`gateway.auth.*` 网关鉴权规则，`gateway.protection.rules` 网关流控规则，以及 `rpc.circuit_breaker.enabled` 内部 RPC 熔断开关。RPC 客户端默认使用 `rpc.timeout: 3s` 和 Kratos circuit breaker middleware，动态配置变更后下一次 RPC 调用会重新读取熔断开关。

降级配置示例：

```yaml
seckill:
  degrade:
    seckill_closed: false
    skip_machine_check: false
    skip_risk_check: false
    skip_order_sync: false
    skip_card_issue: false
```

`seckill_closed` 会让秒杀入口直接返回活动不可用；`skip_machine_check` 和 `skip_risk_check` 用于入口链路降级；`skip_order_sync` / `skip_card_issue` 用于支付成功后的非关键后置动作降级。

## 本地运行

先启动 Redis：

```bash
make redis
```

分别启动服务：

```bash
make activity
make stock
make risk
make order
make support
make processor
make job
make gateway
```

等价的单入口命令：

```bash
go run ./cmd activity-service -c configs/config.yaml
go run ./cmd seckill-gateway -c configs/config.yaml
go run ./cmd seckill-job -c configs/config.yaml
```

默认端口：

```text
seckill-gateway  :8080
activity-service :9001
stock-service    :9002
risk-service     :9003
order-service    :9004
support-service  :9005
```

Docker Compose：

```bash
docker compose up --build
```

## 快速体验

默认种子数据：

- 活动：`1001`
- SKU：`2001`
- 用户头：`X-User-ID: 1`

预检：

```bash
curl -s -H 'X-User-ID: 1' \
  'http://localhost:8080/api/seckill/pre-check?activityNo=1001'
```

下单：

```bash
curl -s -X POST http://localhost:8080/api/seckill/part-in \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 1' \
  -H 'X-Trace-Id: 0123456789abcdef0123456789abcdef' \
  -d '{"activityNo":"1001","skuNo":"2001","quantity":1,"machineToken":"smoke"}'
```

返回体顶层 `requestTraceId` 对应 `X-Trace-Id` 链路追踪；`data.traceId` 是业务排队 ID，下面轮询要用这个值。

轮询：

```bash
curl -s -X POST http://localhost:8080/api/seckill/queue/check \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 1' \
  -d '{"traceId":"上一步返回的traceId"}'
```

预支付：

```bash
curl -s -X POST 'http://localhost:8080/api/pay/prepay?orderNo=订单号&payChannel=mock' \
  -H 'X-User-ID: 1'
```

模拟支付回调：

```bash
curl -s -X POST http://localhost:8080/api/pay/notify/mock \
  -H 'Content-Type: application/json' \
  -d '{"orderNo":"订单号","transactionNo":"mock_tx_001"}'
```

自动烟测：

```bash
make smoke
```

默认会使用 `BASE_URL=http://localhost:8080`、活动 `1001`、SKU `2001`，并在 Redis 写入本地 smoke 所需的库存 key。可通过 `USER_ID`、`ACTIVITY_NO`、`SKU_NO`、`SEED_STOCK=0` 等环境变量覆盖。

## 质量检查

```bash
make test
make lint
```
