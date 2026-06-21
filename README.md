# Go 秒杀微服务

基于 DDD 和微服务边界拆分的 Go 秒杀系统。

- **外部 HTTP**: Gin
- **内部 RPC**: Kratos gRPC
- **消息队列**: NATS JetStream
- **服务发现**: Redis Registry
- **基础库**: `go-common` 共享库

---

## 快速开始

### 启动基础设施

```bash
make redis postgres
```

### 启动服务

```bash
make gateway      # HTTP 网关 :8080
make processor    # 消息消费者
make activity     # 活动服务 :9001
make stock        # 库存服务 :9002
make risk         # 风控服务 :9003
make order        # 订单服务 :9004
make support      # 支付/会员 :9005
make job          # 定时任务
```

### 快速测试

```bash
# 自动化烟测
make smoke

# 手动测试
curl -H 'X-User-ID: 1' \
  'http://localhost:8080/api/seckill/pre-check?activityNo=1001'
```

---

## 服务架构

### 核心服务

| 服务 | 端口 | 职责 |
|------|------|------|
| **seckill-gateway** | 8080 | HTTP 网关、鉴权、限流、入队 |
| **seckill-processor** | - | 异步消费、扣库存、创单 |
| **seckill-job** | - | 定时任务、补偿 |

### 领域服务

| 服务 | 端口 | 职责 |
|------|------|------|
| **activity-service** | 9001 | 活动与 SKU 查询 |
| **stock-service** | 9002 | 库存扣减、释放 |
| **risk-service** | 9003 | 风控评估、黑名单 |
| **order-service** | 9004 | 订单创建与查询 |
| **support-service** | 9005 | 支付、会员、自由卡 |

---

## 核心流程

### 秒杀请求链路

```
用户请求
  ↓
Gin Gateway (鉴权 + 限流 + 风控预检)
  ↓
WorkerPool (并发校验活动/库存/风控)
  ↓
NATS JetStream 入队
  ↓
Processor 消费 (二次校验 + 扣库存 + 创单)
  ↓
Redis 写回结果
  ↓
用户轮询
```

### 支付流程

```
预下单 → 写 Redis 缓存
  ↓
支付回调 (分布式锁防并发)
  ↓
更新订单状态
  ↓
补偿任务: 订单同步 + 发放自由卡
  ↓
支付超时延迟任务 (NATS JetStream)
```

---

## 技术特性

### 消息队列

- **主队列**: NATS JetStream (异步削峰)
- **补偿队列**: Redis List (订单同步/发卡失败重试)
- **延迟任务**: NATS JetStream (支付超时延迟消息)

```yaml
queue:
  provider: nats
  stream: SECKILL_ORDERS
  subject: seckill.orders
  max_deliver: 3
  ack_wait: 30s
```

### 服务发现

- **注册中心**: Redis
- **服务端**: 启动时注册 + 周期续约
- **客户端**: 优先 Redis 发现，可回退静态地址

```yaml
discovery:
  mode: redis
  namespace: seckill
  ttl: 15s
```

### 网关保护

#### JWT 鉴权

```yaml
gateway:
  auth:
    enabled: true
    white_list:
      - /api/seckill/activity/**
      - /api/seckill/product/**
```

#### 限流保护

```yaml
gateway:
  protection:
    enabled: true
    rules:
      - resource: seckill-service
        path_prefixes: [/api/seckill, /api/pay]
        count: 1000
        interval: 1s
```

### 两级缓存

- **L1**: 本地内存缓存 (30m TTL)
- **L2**: Redis (30m TTL)
- **回源**: Activity gRPC

```yaml
cache:
  activity:
    enabled: true
    max_size: 512
    local_ttl: 30m
    redis_ttl: 30m
```

### 风控评估

- **黑名单**: Redis 永久拉黑
- **高风险窗口**: 24小时内高风险行为
- **行为阈值**: 1小时内秒杀次数限制

```yaml
seckill:
  risk:
    high_risk_threshold: 10
    recent_window: 1h
    high_risk_window: 24h
```

---

## API 接口

### 前台接口

```bash
# 查询进行中活动
GET /api/seckill/activity/active

# 查询活动详情
GET /api/seckill/activity/:activityNo

# 查询活动商品
GET /api/seckill/product/:activityNo

# 秒杀预检
GET /api/seckill/pre-check?activityNo=1001

# 秒杀下单
POST /api/seckill/part-in
{
  "activityNo": "1001",
  "skuNo": "2001",
  "quantity": 1,
  "machineToken": "xxx"
}

# 排队结果查询
POST /api/seckill/queue/check
{
  "traceId": "xxx"
}

# 预支付
POST /api/pay/prepay?orderNo=xxx&payChannel=mock

# 支付回调（模拟）
POST /api/pay/notify/mock
{
  "orderNo": "xxx",
  "transactionNo": "xxx"
}
```

### 管理接口

```bash
# 活动列表
GET /api/admin/activity/list

# 活动详情
GET /api/admin/activity/:activityNo

# 创建活动
POST /api/admin/activity

# 更新活动
PUT /api/admin/activity

# 结束活动
POST /api/admin/activity/:activityNo/end

# 添加商品
POST /api/admin/activity/product

# 移除商品
DELETE /api/admin/activity/product?activityNo=xxx&skuNo=xxx
```

**鉴权要求**: 请求头 `X-User-Role: admin`

---

## 开发指南

### 代码结构

```
services/<service>/
├── cmd/main.go           # 服务入口
├── internal/
│   ├── domain/           # 领域实体与仓储端口
│   ├── application/      # 用例与业务编排
│   ├── infrastructure/   # Redis、RPC、队列适配
│   └── server/           # HTTP/gRPC 服务器
└── configs/              # YAML 配置
```

### Proto 代码生成

```bash
make init  # 安装 protoc-gen-go / protoc-gen-go-grpc
make api   # 生成 *.pb.go 和 *_grpc.pb.go
```

### 质量检查

```bash
make test  # 运行测试
make lint  # 代码检查
```

---

## Docker 部署

```bash
# 构建镜像
make docker-build

# 启动所有服务
make docker-up

# 停止服务
make docker-down
```

---

## 定时任务

| 任务 | 间隔 | 职责 |
|------|------|------|
| **活动状态检查** | 1m | 扫描活动时间窗口，更新状态 |
| **超时订单检查** | 10m | 关闭超时未支付订单 |
| **库存清理** | 10m | 清理已结束活动的库存 key |
| **活动数据清理** | 24h | 清理过期活动的限购计数 |
| **风控用户清理** | 24h | 清理过期的风险用户标记 |
| **缓存预热** | 5m | 预热即将开始的活动缓存 |
| **缓存刷新** | 1m | 刷新进行中活动的缓存 |
| **支付对账** | 30m | 对账支付网关状态 |
| **订单同步检查** | 5m | 检查主域订单同步状态 |

---

## 配置说明

### 动态配置热加载

```yaml
dynamic_config:
  enabled: true
  refresh_interval: 5s
```

支持热加载的配置项：
- 秒杀降级开关 (`seckill.degrade.*`)
- 网关鉴权规则 (`gateway.auth.*`)
- 网关流控规则 (`gateway.protection.rules`)
- RPC 熔断开关 (`rpc.circuit_breaker.enabled`)

### 降级开关

```yaml
seckill:
  degrade:
    seckill_closed: false       # 关闭秒杀入口
    skip_machine_check: false   # 跳过机审
    skip_risk_check: false      # 跳过风控
    skip_order_sync: false      # 跳过订单同步
    skip_card_issue: false      # 跳过自由卡发放
```

---

## 环境要求

- Go 1.25+
- Redis 7+
- PostgreSQL 15+
- NATS Server 2.10+

---

## 许可证

MIT License
