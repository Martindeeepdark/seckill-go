# Smoke Loadtest Runbook

## 快速开始

```bash
# 1. 启动完整 stack
make docker-up

# 2. 准备测试数据（活动 + SKU + 库存）
make smoke-setup

# 3. 跑压测
make smoke
```

## 输出解读

```
[SMOKE] default              QPS: 1234.5   | TPS: 234.5    | Peak-shaving: 81.0%  | Rejected: rate-limit=800(64.8%) risk=100(8.1%) stock-empty=100(8.1%) other=0(0.0%) | Total: 1234
```

| 字段 | 含义 |
|---|---|
| QPS | wrk 实测 Requests/sec（HTTP 入口速率，含被拒） |
| TPS | 真正建单成功速率 = success / duration |
| Peak-shaving | 削峰效率 = (QPS - TPS) / QPS × 100%，越高说明网关层挡掉越多 |
| rate-limit | 用户级限流拒绝的请求数 |
| risk | 风控黑名单拒绝的请求数 |
| stock-empty | 库存空拒绝的请求数 |
| other | 机审失败/队列满等拒绝的请求数 |
| Total | 总请求数（wrk 发出） |

## 梯度压测

```bash
SMOKE_GRADIENT=1 make smoke
```

四档连续跑：c=50 → c=100 → c=200 → c=500，每档独立汇总。

## 功能 Smoke（旧版）

```bash
make smoke-func
```

保留的原有单请求功能验证，curl 一次全链路。

## 自定义参数

```bash
ACTIVITY_NO=2001 SKU_NO=SKU002 make smoke-setup
ACTIVITY_NO=2001 SKU_NO=SKU002 make smoke
```

## Redis Counter

- Key 格式: `seckill:metrics:<run-id>`
- TTL: 1h
- 字段: rate-limit / risk / stock-empty / success / other
- smoke 起始自动 DEL 保证基线干净

## 常见问题

**Q: wrk 报大量 `get sku: not found`？**
A: 先跑 `make smoke-setup` 创建测试活动+SKU。

**Q: 没有 redis-cli？**
A: 脚本会自动 fallback 到 `docker compose exec redis redis-cli`。

**Q: TPS 显示 N/A？**
A: 安装 `bc`：`brew install bc`（macOS）或 `apt install bc`。

**Q: 如何调优？**
- 调小令牌桶：改 gateway config 的 `rate_limit.user_rate`
- 加黑名单：`redis-cli SET seckill:risk:user:<userId> blacklist EX 300`
- 库存设小：`REDIS_ADDR=127.0.0.1 redis-cli SET seckill:stock:<activityNo>:<skuNo> 10`
