#!/usr/bin/env bash
set -euo pipefail

# smoke.sh — 秒杀压测脚本（wrk + Redis HINCRBY counter 聚合）
# 用法: make smoke  或  SMOKE_GRADIENT=1 make smoke
# 前置: make smoke-setup
# 可覆盖: BASE_URL ACTIVITY_NO SKU_NO

BASE_URL="${BASE_URL:-http://localhost:8080}"
ACTIVITY_NO="${ACTIVITY_NO:-1001}"
SKU_NO="${SKU_NO:-SKU001}"
SMOKE_GRADIENT="${SMOKE_GRADIENT:-0}"
SMOKE_BASE_USER="${SMOKE_BASE_USER:-100000}"
SMOKE_USER_COUNT="${SMOKE_USER_COUNT:-100}"
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-16379}"
SMOKE_MODE="${SMOKE_MODE:-wrk}"

# ---- functional smoke (legacy) ----
if [[ "$SMOKE_MODE" == "func" ]]; then
    exec bash "$(dirname "$0")/smoke-func.sh"
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

log()  { printf '[smoke] %s\n' "$*"; }
fail() { printf '[smoke] ERROR: %s\n' "$*" >&2; exit 1; }
warn() { printf '[smoke] WARN: %s\n' "$*" >&2; }

need() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }

# ---- Redis helper ----
redis_cmd() {
    if command -v redis-cli >/dev/null 2>&1; then
        redis-cli -h "$REDIS_ADDR" -p "$REDIS_PORT" "$@"
        return
    fi
    if command -v docker >/dev/null 2>&1; then
        docker compose exec -T redis redis-cli -p 6379 "$@"
        return
    fi
    warn "redis-cli not available (install redis-cli or run docker compose up)"
    return 1
}

# ---- Metrics aggregation ----
aggregate() {
    local run_id="$1" label="$2"
    local key="seckill:metrics:${run_id}"

    log "aggregating $label..."

    # Collect all fields
    local all_fields
    all_fields=$(redis_cmd HGETALL "$key" 2>/dev/null || true)
    if [[ -z "$all_fields" ]]; then
        warn "no metrics found for run_id=$run_id"
        return
    fi

    # Parse HGETALL output (compatible with bash 3.2+)
    local rate_limit=0 risk=0 stock_empty=0 success=0 other=0
    local field=""
    while IFS= read -r line; do
        if [[ -z "$field" ]]; then
            field="$line"
        else
            case "$field" in
                rate-limit)  rate_limit="$line" ;;
                risk)        risk="$line" ;;
                stock-empty) stock_empty="$line" ;;
                success)     success="$line" ;;
                other)       other="$line" ;;
            esac
            field=""
        fi
    done <<< "$all_fields"

    local total_rejected=$((rate_limit + risk + stock_empty + other))
    local total=$((total_rejected + success))

    if [[ "$total" -eq 0 ]]; then
        warn "zero total requests — check wrk output"
        return
    fi

    # wrk 输出里提取 QPS (Requests/sec)，用 awk 避免 GNU grep -P 的平台差异
    local qps="N/A"
    local wrk_output="$tmpdir/wrk-${run_id}.txt"
    if [[ -f "$wrk_output" ]]; then
        qps=$(awk '/Requests\/sec/{for(i=1;i<=NF;i++)if($i~/^[0-9.]+$/){print $i;exit}}' "$wrk_output" 2>/dev/null)
        [[ -z "$qps" ]] && qps="N/A"
    fi

    # TPS = success per second
    local tps="N/A"
    local duration=30
    if [[ -f "$wrk_output" ]]; then
        local dur_sec
        # wrk 输出 "NNNNN requests in 30.03s, ..." 提取秒数
        dur_sec=$(awk '/requests in/{for(i=1;i<=NF;i++)if($i=="in"){print $(i+1);exit}}' "$wrk_output" 2>/dev/null | tr -d 's,')
        if [[ -n "$dur_sec" ]]; then
            duration=$(echo "$dur_sec" | awk '{print int($1 + 0.5)}')
        fi
    fi
    if [[ "$duration" -gt 0 ]]; then
        tps=$(echo "scale=1; $success / $duration" | bc 2>/dev/null || echo "N/A")
    fi

    # Peak-shaving ratio
    local ps_ratio="N/A"
    if [[ "$qps" != "N/A" && "$tps" != "N/A" ]] && [[ "$(echo "$qps > 0" | bc 2>/dev/null || echo 0)" -eq 1 ]]; then
        ps_ratio=$(echo "scale=1; ($qps - $tps) / $qps * 100" | bc 2>/dev/null || echo "N/A")
        ps_ratio="${ps_ratio}%"
    fi

    # Build output line
    local parts=""
    parts+="rate-limit=${rate_limit}($(pct "$rate_limit" "$total")) "
    parts+="risk=${risk}($(pct "$risk" "$total")) "
    parts+="stock-empty=${stock_empty}($(pct "$stock_empty" "$total")) "
    parts+="other=${other}($(pct "$other" "$total"))"

    printf '[SMOKE] %-20s QPS: %-8s | TPS: %-8s | Peak-shaving: %-8s | Rejected: %s| Total: %d\n' \
        "$label" "$qps" "$tps" "$ps_ratio" "$parts" "$total"
}

pct() {
    local num="$1" denom="$2"
    if [[ "$denom" -eq 0 ]]; then
        echo "0.0%"
    else
        echo "scale=1; $num * 100 / $denom" | bc 2>/dev/null | sed 's/$/%/'
    fi
}

# ---- Single run ----
run_smoke() {
    local run_id="$1" connections="$2" duration="$3" threads="$4" label="$5"

    # Clean baseline
    redis_cmd DEL "seckill:metrics:${run_id}" >/dev/null 2>&1 || true

    # Cache warm
    curl -sS "$BASE_URL/api/seckill/activity/$ACTIVITY_NO" >/dev/null 2>&1 || true

    log "running wrk: -c${connections} -t${threads} -d${duration}s run_id=$run_id"
    wrk -c"$connections" -t"$threads" -d"${duration}s" \
        -s "$(dirname "$0")/wrk-part-in.lua" \
        "$BASE_URL" \
        > "$tmpdir/wrk-${run_id}.txt" 2>&1 || warn "wrk exited non-zero"

    aggregate "$run_id" "$label"
    echo ""
}

# ---- Gradient mode ----
run_gradient() {
    local levels=(50 100 200 500)
    local duration=30
    local threads=8

    export SMOKE_ACTIVITY_NO="$ACTIVITY_NO"
    export SMOKE_SKU_NO="$SKU_NO"
    export SMOKE_BASE_USER="$SMOKE_BASE_USER"
    export SMOKE_USER_COUNT="$SMOKE_USER_COUNT"

    log "=== Gradient smoke (no-gap) ==="
    for level in "${levels[@]}"; do
        local run_id="smoke-$(date +%Y%m%d-%H%M%S)-g${level}-$$"
        export SMOKE_RUN_ID="$run_id"
        run_smoke "$run_id" "$level" "$duration" "$threads" "c=$level"
    done
}

# ---- Main ----
need curl
need bc || warn "bc not found, TPS/Peak-shaving will show N/A"

# Cache warm once
curl -sS "$BASE_URL/api/seckill/activity/$ACTIVITY_NO" >/dev/null 2>&1 || warn "cache warm failed"

if [[ "$SMOKE_GRADIENT" == "1" ]]; then
    need wrk
    run_gradient
else
    need wrk
    run_id="smoke-$(date +%Y%m%d-%H%M%S)-$$"
    export SMOKE_RUN_ID="$run_id"
    export SMOKE_ACTIVITY_NO="$ACTIVITY_NO"
    export SMOKE_SKU_NO="$SKU_NO"
    export SMOKE_BASE_USER="$SMOKE_BASE_USER"
    export SMOKE_USER_COUNT="$SMOKE_USER_COUNT"
    run_smoke "$run_id" 200 30 8 "default"
fi
