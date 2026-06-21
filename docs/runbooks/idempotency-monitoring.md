# Runbook: Idempotency Monitoring

Scope: alerts and operational checks for the three-layer idempotency introduced by `consumer-order-idempotency`.

## 1. sk_order trace_id UNIQUE INDEX

**Index**: `uk_sk_order_user_trace(user_id, trace_id) WHERE trace_id IS NOT NULL AND trace_id != ''`
**Location**: `services/order-service/db/migrations/003_add_unique_index_sk_order_trace_id.sql`

### Alert: duplicate-key violation rate

- **Metric**: `order_service_create_order_duplicate_total` (counter on `*pgconn.PgError` with `Code="23505"`)
- **Trigger**: rate() > 1/min sustained for 5 min in non-release hours
- **Meaning**: Layer 1 (Redis SETNX) is leaking through consistently. Either:
  - Redis lost the idem key (eviction / failover / AOF fsync issue)
  - Processor idem key TTL (60s) is shorter than end-to-end processing time (sustained overload)
- **Action**:
  1. Check Redis memory / eviction: `redis-cli INFO memory | grep used_memory_peak`
  2. Check p99 latency of `processor.submit_seckill.execute` — if > 55s, raise TTL in `cmd/main.go` `processorStore.TryStart`
  3. Inspect a sample `trace_id` — does the second message arrive within 60s of the first? If yes → TTL too short.

### Alert: DuplicateKey lookup failure

- **Metric**: `processor_duplicatekey_lookup_failure_total` (counter)
- **Trigger**: any non-zero value
- **Meaning**: INSERT got 23505 but `GetByUserAndTrace` returned `ErrNotFound`. Theoretically impossible without external DELETE on `sk_order`. Indicates either:
  - Race with order-sync job deleting the just-inserted row
  - DB-level inconsistency (broken replica / split-brain)
- **Action**: page SRE; treat as data-integrity incident. Pull `trace_id`, `user_id` from log and query `sk_order` directly.

## 2. Processor idem key TTL budget

| State | TTL | Source |
|---|---|---|
| PROCESSING | 60s | `submit_seckill.go` `processorStore.TryStart(traceID, 60*time.Second)` |
| Final (orderNo / fail-reason) | 5min | `application/seckill.go::markTraceSuccess/markTraceFail` |

### Alert: processor retry storm

- **Metric**: distinct `trace_id` count in `seckill:processor:idem:*` keys normalized by message rate
- **Trigger**: ratio > 1.2 sustained for 10 min (more idem keys than unique input messages)
- **Meaning**: messages being redelivered faster than processing completes (NATS max_deliver hit).
- **Action**: check downstream service latency (stock / order gRPC), consider increasing NATS ack wait.

## 3. Operational tasks

### Pre-deployment: clean historical duplicate trace_id

Before running migration `003_*` on production:

```sql
-- Scan
SELECT user_id, trace_id, COUNT(*) FROM sk_order
WHERE trace_id IS NOT NULL AND trace_id != ''
GROUP BY user_id, trace_id HAVING COUNT(*) > 1;

-- Clean (preserve earliest by created_at; blank the rest)
UPDATE sk_order SET trace_id = ''
WHERE trace_id != '' AND (user_id, trace_id, created_at) NOT IN (
    SELECT user_id, trace_id, MIN(created_at) FROM sk_order
    WHERE trace_id != ''
    GROUP BY user_id, trace_id
    HAVING COUNT(*) > 1
) AND (user_id, trace_id) IN (
    SELECT user_id, trace_id FROM sk_order
    WHERE trace_id != ''
    GROUP BY user_id, trace_id
    HAVING COUNT(*) > 1
);
```

### Verify migration applied

```sql
SELECT indexname FROM pg_indexes
WHERE schemaname = 'public' AND tablename = 'sk_order'
  AND indexname = 'uk_sk_order_user_trace';
-- Expect: one row across the parent + each HASH partition (p0-p3)
```

## 4. Dashboards

- **Grafana panel "Idempotency leak"**: rate of `order_service_create_order_duplicate_total` overlaid on processor message rate
- **Redis panel**: `db0:keys=seckill_processor_idem_*` count vs `db0:keys=seckill_order_result_*` count — should track each other within 10%
