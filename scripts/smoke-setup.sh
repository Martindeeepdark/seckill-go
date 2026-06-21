#!/usr/bin/env bash
set -euo pipefail

# smoke-setup.sh — 为 smoke 压测准备测试数据（活动 + SKU + Redis 库存）
# 用法: make smoke-setup 或 bash scripts/smoke-setup.sh
# 可覆盖: BASE_URL ACTIVITY_NO SKU_NO STOCK_SEED ADMIN_TOKEN

BASE_URL="${BASE_URL:-http://localhost:8080}"
ACTIVITY_NO="${ACTIVITY_NO:-1001}"
SKU_NO="${SKU_NO:-SKU001}"
STOCK_SEED="${STOCK_SEED:-10000}"
ADMIN_TOKEN="${ADMIN_TOKEN:-}"
HEALTH_ATTEMPTS="${HEALTH_ATTEMPTS:-30}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-1}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

log()  { printf '[smoke-setup] %s\n' "$*"; }
fail() { printf '[smoke-setup] ERROR: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }

# ---- 网关健康检查 ----
wait_for_health() {
    local output="$1"
    for ((i = 1; i <= HEALTH_ATTEMPTS; i++)); do
        if status=$(curl -sS -o "$output" -w '%{http_code}' "$BASE_URL/healthz" 2>/dev/null) && \
            [[ "$status" -ge 200 && "$status" -lt 300 ]]; then
            return
        fi
        log "waiting for gateway health attempt=$i"
        sleep "$HEALTH_INTERVAL"
    done
    fail "gateway health check failed after $HEALTH_ATTEMPTS attempts"
}

# ---- Admin API helper ----
admin_post() {
    local url="$1" body="$2" output="$3"
    local status
    local auth_header=""
    [[ -n "$ADMIN_TOKEN" ]] && auth_header="-H Authorization: Bearer $ADMIN_TOKEN"
    if ! status=$(curl -sS -o "$output" -w '%{http_code}' -X POST "$url" \
        -H 'Content-Type: application/json' \
        -H 'X-User-Role: admin' \
        $auth_header \
        --data "$body"); then
        cat "$output" >&2 2>/dev/null || true
        fail "POST $url failed"
    fi
    if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
        cat "$output" >&2 || true
        fail "POST $url returned HTTP $status"
    fi
}

# ---- 主流程 ----
need curl

health_file="$tmpdir/health.json"
wait_for_health "$health_file"
log "gateway health ok"

# 创建活动（幂等：已存在则跳过）
activity_body="{\"activityNo\":\"$ACTIVITY_NO\",\"activityName\":\"smoke-test-activity\",\"startTime\":\"2025-01-01T00:00:00Z\",\"endTime\":\"2030-01-01T00:00:00Z\"}"
admin_post "$BASE_URL/api/admin/activities" "$activity_body" "$tmpdir/activity.json"
log "activity $ACTIVITY_NO created (or already exists)"

# 添加 SKU
sku_body="{\"skuNo\":\"$SKU_NO\",\"skuName\":\"smoke-test-sku\",\"seckillPrice\":99,\"originalPrice\":199,\"activityStock\":99999,\"limitQuantity\":5}"
admin_post "$BASE_URL/api/admin/activities/$ACTIVITY_NO/products" "$sku_body" "$tmpdir/sku.json"
log "sku $SKU_NO added to activity $ACTIVITY_NO"

# 预热缓存
curl -sS "$BASE_URL/api/seckill/activity/$ACTIVITY_NO" >/dev/null 2>&1 || true
log "activity cache warmed"

# 写入 Redis 库存
redis_cmd() {
    local host="${REDIS_ADDR:-127.0.0.1}" port="${REDIS_PORT:-6379}"
    if command -v redis-cli >/dev/null 2>&1; then
        redis-cli -h "$host" -p "$port" "$@" >/dev/null 2>&1
        return
    fi
    if command -v docker >/dev/null 2>&1; then
        docker compose exec -T redis redis-cli "$@" >/dev/null 2>&1
        return
    fi
    return 1
}
if redis_cmd SET "seckill:stock:${ACTIVITY_NO}:${SKU_NO}" "$STOCK_SEED"; then
    log "redis stock seeded: seckill:stock:${ACTIVITY_NO}:${SKU_NO}=$STOCK_SEED"
else
    log "WARNING: redis-cli unavailable, stock not seeded (set REDIS_ADDR or run docker compose up)"
fi

log "setup complete"
echo ""
echo "Run smoke test:"
echo "  export ACTIVITY_NO=$ACTIVITY_NO SKU_NO=$SKU_NO"
echo "  make smoke"
