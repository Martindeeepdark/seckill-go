# 秒杀系统功能缺失分析与开发路线图

> 对照 Java 秒杀课程 PRD，梳理 Go 秒杀系统当前功能覆盖与缺失情况。
> 生成日期：2026-06-12

---

## 一、当前系统功能覆盖

| 服务 | 核心能力 | 状态 |
|------|---------|------|
| seckill-gateway | HTTP 路由、限流（本地+Redis）、Worker Pool、TraceID、幂等 | 已实现 |
| seckill-processor | 异步消费、后支付处理、发卡、订单同步 | 已实现 |
| seckill-job | 活动状态轮转、超时关单、库存清理、对账补偿（基础） | 已实现 |
| support-service | 支付、会员、自由卡发卡、降级模式 | 已实现 |
| activity-service | 活动 CRUD、多 SKU、状态流转、事件驱动 | 已实现 |
| stock-service | 原子扣库存、库存回收、事件总线 | 已实现 |
| risk-service | 风控评估、黑名单、购买频次限制 | 已实现 |
| order-service | 订单创建、状态流转、去重 | 已实现 |

---

## 二、功能缺失分析

### Tier 1：关键缺失（影响生产可用性）

| # | 功能 | Java PRD 参考 | 缺失影响 | 建议实现方式 |
|---|------|-------------|---------|------------|
| 1 | **运营后台 (seckill-admin)** | 管理端全量 CRUD + 监控面板 | 无法在线管理活动、无法人工干预 | 新服务，gRPC 管理接口 + Web 前端 |
| 2 | **动态配置中心** | 活动级别开关、库存保护阈值、限流参数热更新 | 改配置需重启服务 | etcd watch + 内存缓存，按活动维度 |
| 3 | **人工干预接口** | 售罄强制恢复、异常订单作废、库存手动增补 | 故障时无法快速止损 | admin 服务暴露干预 gRPC，gateway 直接调用 |
| 4 | **对账与补偿体系** | 定时对账 + 差异补偿 + 补偿记录 | 支付成功但发卡/订单同步失败时数据不一致 | seckill-job 增加对账任务 + 补偿表 + 人工审核队列 |
| 5 | **策略模式 + 自由卡账户体系** | 策略模式解耦支付渠道、自由卡作为独立账户 | 支付流程硬编码，自由卡只是 mock | support-service 增加账户余额 + 策略工厂 |

### Tier 2：重要增强（提升系统健壮性）

| # | 功能 | Java PRD 参考 | 缺失影响 | 建议实现方式 |
|---|------|-------------|---------|------------|
| 6 | **三重幂等** | 网关 Token 幂等 + 业务主键幂等 + 支付回调幂等 | 高并发下重复请求可能导致重复扣库存 | gateway: traceId 幂等 → processor: orderId 幂等 → support: payNo 幂等 |
| 7 | **traceId 一次性消费** | traceId 标记已使用，防止重复使用同一 traceId 提交 | 当前仅做状态标记，未严格一次性 | Redis SETNX + TTL，consume 时原子标记 |
| 8 | **购买限次排序** | 限购校验在扣库存之后，理论上可被并发绕过 | 高并发下可能超卖 | 将限购校验移至 worker 处理链最前端，与库存扣减原子化 |
| 9 | **stackRelease（分层释放）** | 库存分层管理：预扣 → 确认 → 释放 | 当前只有扣/回两步，缺乏预扣态 | stock-service 增加冻结态，gateway 提交时预扣，processor 确认或释放 |
| 10 | **队列容量限流** | Worker Pool 满时直接拒绝，防止内存溢出 | 当前有 backpressure 但缺乏明确拒绝策略 | WorkerPool.Submit 返回 err 时 gateway 返回 429 + 友好提示 |
| 11 | **服务重启恢复** | NATS 消费断点续传 + 处理中任务恢复 | 重启时处理中的任务可能丢失 | NATS durable consumer + 处理中任务表记录状态 |

### Tier 3：进阶功能（锦上添花）

| # | 功能 | Java PRD 参考 | 缺失影响 | 建议实现方式 |
|---|------|-------------|---------|------------|
| 12 | **真实支付集成** | 微信/支付宝支付对接 | 当前 mock 支付 | support-service 实现 WechatPayGateway/AlipayGateway |
| 13 | **退款处理** | 退款流程 + 退款状态管理 | 退不了钱 | order-service 增加退款状态 + support-service 退款接口 |
| 14 | **渠道级开关** | 按支付渠道单独开关 | 某渠道故障时无法单独降级 | 动态配置中增加 per-channel enable/disable |
| 15 | **值班统计面板** | 值班人员实时查看关键指标 | 运维黑盒 | admin 服务增加 Prometheus metrics 查询 + 简易仪表盘 |
| 16 | **限量/限时模式切换** | 限量秒杀（逐条写）与限时秒杀（批量写）策略切换 | 当前只有一种处理路径 | stock-service 策略模式，按活动类型选择扣减策略 |

---

## 三、开发阶段规划

### Phase 1：生产可用性补齐（建议优先）

> 目标：系统具备基本的生产运维能力，可以管理活动、人工干预、数据一致性保障

| 任务 | 涉及服务 | 预估复杂度 | 优先级 |
|------|---------|-----------|--------|
| 1.1 运营后台骨架 + 活动 CRUD | 新建 seckill-admin | 高 | P0 |
| 1.2 动态配置中心（etcd watch） | seckill-gateway, seckill-job | 中 | P0 |
| 1.3 人工干预接口（售罄恢复/库存增补/订单作废） | seckill-admin + 各服务 | 中 | P0 |
| 1.4 对账与补偿体系 | seckill-job + support-service | 高 | P0 |
| 1.5 自由卡账户体系 + 策略模式 | support-service | 高 | P1 |

### Phase 2：健壮性加固

> 目标：高并发场景下数据一致性保障，防止超卖、重复消费

| 任务 | 涉及服务 | 预估复杂度 | 优先级 |
|------|---------|-----------|--------|
| 2.1 三重幂等（网关+业务+支付） | seckill-gateway, seckill-processor, support-service | 高 | P1 |
| 2.2 traceId 一次性消费 | seckill-gateway | 低 | P1 |
| 2.3 购买限次排序优化 | seckill-gateway (Worker Pool) | 中 | P1 |
| 2.4 stackRelease 预扣机制 | stock-service | 高 | P2 |
| 2.5 队列容量限流 + 优雅拒绝 | seckill-gateway | 低 | P2 |
| 2.6 服务重启恢复 | seckill-processor, seckill-job | 中 | P2 |

### Phase 3：进阶功能

> 目标：完整商业能力，可支撑真实运营

| 任务 | 涉及服务 | 预估复杂度 | 优先级 |
|------|---------|-----------|--------|
| 3.1 真实支付集成（微信/支付宝） | support-service | 高 | P3 |
| 3.2 退款处理 | order-service, support-service | 中 | P3 |
| 3.3 渠道级开关 | seckill-gateway, support-service | 低 | P3 |
| 3.4 值班统计面板 | seckill-admin | 中 | P3 |
| 3.5 限量/限时模式切换 | stock-service, seckill-gateway | 中 | P3 |

---

## 四、已有 Comet Changes 状态

| Change | 状态 | 说明 |
|--------|------|------|
| freecard-domain-consolidation | archived | 注释修正 + domain 层收缩 |
| gateway-admin-routes | archived | gateway 管理 API 路由 |
| seckill-high-concurrency-optimization | phase: build | 高并发性能优化（Worker Pool、分片限流、本地缓存） |
| stock-aggregate-ddd-refactor | archived | 库存聚合根 DDD 重构 |
| order-service-deadlock-fix | archived | 订单服务死锁修复 |
| db-order-sharding | archived | 订单分库分表 |
| fix-lint-issues | archived | Lint 问题修复 |

---

## 五、任务分配建议

| 开发者 | Phase 1 任务 | Phase 2 任务 | 备注 |
|--------|------------|------------|------|
| **后端 A** | 1.1 运营后台骨架 | 2.1 三重幂等 | 负责 admin 服务 + 幂等体系 |
| **后端 B** | 1.2 动态配置 + 1.3 人工干预 | 2.4 stackRelease | 配置中心 + 库存策略 |
| **后端 C** | 1.4 对账补偿 | 2.6 重启恢复 | 可靠性专家 |
| **后端 D** | 1.5 自由卡账户 + 策略模式 | 2.2-2.3 traceId/限次 | 支付域专家 |

> 以上分配为建议方案，实际安排根据团队情况调整。每个任务建议走独立 comet change 流程。

---

## 六、当前活跃 Change：seckill-high-concurrency-optimization

此 change 正在进行中（phase: build），涵盖：
- Worker Pool 启用
- 分片限流器
- 本地缓存（活动信息）
- Redis Pipeline 优化

完成后将为 Phase 2 的健壮性加固提供性能基线。
