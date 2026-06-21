# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
# Proto code generation (run after any .proto change)
make init          # install protoc-gen-go, protoc-gen-go-grpc, golangci-lint
make api           # generate *.pb.go and *_grpc.pb.go from api/**/*.proto

# Build & test
make build         # compile all modules
make test          # run tests across all modules
make lint          # golangci-lint across workspace

# Run single service locally (each runs its own cmd/main.go)
make gateway       # seckill-gateway on :8080
make processor     # seckill-processor (NATS consumer)
make activity      # activity-service gRPC on :9001
make stock         # stock-service gRPC on :9002
make risk          # risk-service gRPC on :9003
make order         # order-service gRPC on :9004
make support       # support-service gRPC on :9005
make job           # seckill-job (cron scheduler)

# Run tests for a single module
cd services/seckill-gateway && go test -count=1 ./...

# Docker
make docker-build  # cross-compile Linux binaries + build images
make docker-up     # build + docker compose up -d
make docker-down

# End-to-end smoke test (requires running stack)
make smoke          # wrk 压测 + Redis counter 聚合输出 QPS/TPS/拒绝原因
make smoke-setup    # 准备测试数据（活动+SKU+Redis库存）
make smoke-func     # 旧版单请求功能 smoke
make smoke          # 梯度模式: SMOKE_GRADIENT=1 make smoke

# Infrastructure only
make redis postgres  # start just Redis + Postgres via docker compose
```

## Architecture

Go microservice seckill (flash-sale) system built with Go 1.25 workspace mode. Translated from a Java reference implementation.

### Go Workspace Layout

```
go.work
├── api/              # Shared protobuf definitions and generated Go code
├── common/           # Shared library (config, discovery, DB, Redis, tracing, eventbus, errors)
└── services/
    ├── activity-service/     # Activity & SKU query gRPC
    ├── stock-service/        # Stock snapshot, deduction, release gRPC
    ├── risk-service/         # Blacklist, risk records, risk evaluation gRPC
    ├── order-service/        # Order creation and query gRPC
    ├── support-service/      # Payment, member, free-card, order-sync gRPC
    ├── seckill-gateway/      # External Gin HTTP gateway (auth, rate-limit, orchestration, queue publish)
    ├── seckill-processor/    # NATS JetStream consumer (validate, deduct stock, create order, result writeback)
    └── seckill-job/          # Cron: activity state transitions, payment timeout, stock cleanup, reconciliation
```

### Service Tiers

- **Tier 1 (leaf services)**: activity, stock, risk, order, support — each exposes gRPC only, no cross-service calls
- **Tier 2 (consumer services)**: gateway, processor, job — call Tier 1 via gRPC, no outbound HTTP

### Internal Package Convention (per service)

```
cmd/main.go              # Entry point, reads config path from os.Args[1]
internal/
  config/                # YAML config loading (uses go-common/config)
  domain/                # Domain entities and repository ports (DDD)
  application/           # Use cases and business orchestration
  infrastructure/        # Redis, queue adapters, RPC client adapters, cache
  server/                # Gin HTTP handlers or Kratos gRPC server registration
```

### Key Data Flow: Seckill Request

1. Client → `seckill-gateway` (Gin HTTP, JWT auth, rate-limit, machine check)
2. Gateway → WorkerPool (async validation: activity cache + risk evaluation in parallel via errgroup)
3. WorkerPool → NATS JetStream `seckill.orders`
4. `seckill-processor` consumes → second validation → deduct stock (Redis atomic) → create order (gRPC) → write result to Redis
5. Client polls gateway for queue result

### User Rate Limiting

**Implementation:**
- Redis ZSET sliding window algorithm
- Limit: 10 requests / 10 seconds (any 10-second window)
- Shared counter across multiple Gateway instances

**Configuration:**
```yaml
gateway:
  rate_limit:
    user_enabled: true
    user_rate: 10
    user_interval: 10s
```

**Degradation Strategy:**
- Fail-open when Redis is unavailable (allow requests)
- Log errors and trigger alerts

**Performance:**
- p99 latency < 10ms
- Redis QPS increase ~2000 per 1000 gateway throughput

### Infrastructure Dependencies

- **Redis**: Service discovery (etcd preferred), rate limiting, result storage, stock atomic ops, distributed locks, compensation queues
- **NATS JetStream**: Primary message queue for seckill async peak-shaving. Redis Stream is legacy fallback.
- **PostgreSQL**: Databases per domain (`seckill_activity`, `seckill_order`, `seckill_risk`, `seckill_support`), accessed via pgx
- **etcd**: Service registry and discovery (Kratos registrar), dynamic config

### Service Discovery

Services register gRPC endpoints via etcd (or Redis legacy). Clients use Kratos `registry.Discovery` with `discovery:///service-name` URIs. Static fallback only when `static_fallback: true`.

### Idempotency: Three-Layer Defense (consumer-order-idempotency)

| Layer | Mechanism | Where |
|---|---|---|
| 1. Redis SETNX | `seckill:processor:idem:<traceId>` SetNX → PROCESSING(60s) / orderNo(5min) / fail-reason(5min) | `services/seckill-processor/internal/application/usecase/submit_seckill.go` |
| 2. DB UNIQUE INDEX | `uk_sk_order_user_trace(user_id, trace_id)` partial UNIQUE on `sk_order` | `services/order-service/db/migrations/003_*` + `persistence/postgres.go::GetByUserAndTrace` |
| 3. State machine | `WHERE order_status = ...` conditional UPDATE (payment callback / stock release — separate change) | (future Change 2) |

**Processor uses an independent idem key** (`seckill:processor:idem:`), NOT shared with gateway (`seckill:order:result:`). TTLs differ (60s crash-recovery vs 5min poll window); gateway-PROCESSING-shadow would always make processor SetNX fail. MarkSuccess/MarkFail writes both keys (gateway first, processor second). Release only deletes the processor key (Lua CAS: only if value=PROCESSING), gateway key keeps showing PROCESSING until retry writes final result.

### Config

Each service has `configs/config.yaml` (local) and `configs/config.docker.yaml` (Docker). Loaded via `github.com/Martindeeepdark/go-common/config`. Config path passed as first CLI argument.

### Proto Contracts

`api/<service>/v1/*.proto` — each domain service has its own proto package. `api/common/v1/common.proto` for cross-service types. Generated code lives alongside proto files. Run `make api` after any proto change.

### Module Naming

Each service is its own Go module: `seckill-<service-name>-service`. The shared library is `seckill-common`. The api module is `seckill`.

### External Dependencies

- `github.com/Martindeeepdark/go-common` — local replace at `/Users/wenyz/GolandProjects/common` (config, logger, snowflake ID, Redis helpers)
- `github.com/go-kratos/kratos/v2` — gRPC framework, registry, circuit breaker
- `github.com/gin-gonic/gin` — HTTP framework (gateway only)
- `github.com/nats-io/nats.go` — NATS JetStream client
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `github.com/redis/go-redis/v9` — Redis client
- `go.etcd.io/etcd/client/v3` — etcd client

### Tracing

`X-Trace-Id` propagates from gateway HTTP through gRPC metadata and async messages. W3C `traceparent` header also supported. OpenTelemetry SDK integrated via common tracing package.

## Development Notes

- Go workspace mode: changes in `common/` or `api/` are immediately visible to all services without `go mod edit` or `replace`
- Each service compiles independently (`go build ./...` inside service dir)
- The `application.test` file at project root is a pre-built test binary, not source
- Docker ports: Postgres 15432, Redis 16379, NATS 14222, etcd 12379, gateway HTTP 8080
