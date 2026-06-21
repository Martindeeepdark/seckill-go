#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ACTIVITY_NO="${ACTIVITY_NO:-1001}"
SKU_NO="${SKU_NO:-2001}"
QUANTITY="${QUANTITY:-1}"
USER_ID="${USER_ID:-$((100000 + $(date +%s) % 900000))}"
PAY_CHANNEL="${PAY_CHANNEL:-mock}"
POLL_ATTEMPTS="${POLL_ATTEMPTS:-30}"
POLL_INTERVAL="${POLL_INTERVAL:-1}"
HEALTH_ATTEMPTS="${HEALTH_ATTEMPTS:-30}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-1}"
SEED_STOCK="${SEED_STOCK:-1}"
STOCK_SEED="${STOCK_SEED:-100}"
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
TRACE_ID="${TRACE_ID:-}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

log() {
	printf '[smoke] %s\n' "$*"
}

fail() {
	printf '[smoke] ERROR: %s\n' "$*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

generate_trace_id() {
	if command -v openssl >/dev/null 2>&1; then
		openssl rand -hex 16
		return
	fi
	date +%s%N | shasum -a 256 | awk '{print substr($1, 1, 32)}'
}

json_get() {
	local file="$1"
	local path="$2"
	python3 - "$file" "$path" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as f:
    data = json.load(f)
value = data
for part in sys.argv[2].split("."):
    if isinstance(value, dict):
        value = value.get(part)
    else:
        value = None
    if value is None:
        break
if value is None:
    sys.exit(1)
if isinstance(value, bool):
    print("true" if value else "false")
else:
    print(value)
PY
}

http_request() {
	local method="$1"
	local url="$2"
	local body="${3:-}"
	local output="$4"
	local status
	if [[ -n "$body" ]]; then
		if ! status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" "$url" \
			-H 'Content-Type: application/json' \
			-H "X-User-Id: $USER_ID" \
			-H "X-Trace-Id: $TRACE_ID" \
			--data "$body")"; then
			cat "$output" >&2 2>/dev/null || true
			fail "$method $url failed"
		fi
	else
		if ! status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" "$url" \
			-H "X-User-Id: $USER_ID" \
			-H "X-Trace-Id: $TRACE_ID")"; then
			cat "$output" >&2 2>/dev/null || true
			fail "$method $url failed"
		fi
	fi
	if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
		cat "$output" >&2 || true
		fail "$method $url returned HTTP $status"
	fi
}

wait_for_health() {
	local output="$1"
	local status
	for ((i = 1; i <= HEALTH_ATTEMPTS; i++)); do
		if status="$(curl -sS -o "$output" -w '%{http_code}' "$BASE_URL/healthz" 2>/dev/null)" &&
			[[ "$status" -ge 200 && "$status" -lt 300 ]]; then
			return
		fi
		log "waiting for gateway health attempt=$i"
		sleep "$HEALTH_INTERVAL"
	done
	fail "gateway health check failed after $HEALTH_ATTEMPTS attempts"
}

redis_cmd() {
	local host="${REDIS_ADDR%:*}"
	local port="${REDIS_ADDR##*:}"
	if command -v redis-cli >/dev/null 2>&1; then
		redis-cli -h "$host" -p "$port" "$@" >/dev/null
		return
	fi
	if command -v docker >/dev/null 2>&1; then
		docker compose exec -T redis redis-cli "$@" >/dev/null 2>&1
		return
	fi
	return 1
}

seed_stock() {
	if [[ "$SEED_STOCK" != "1" ]]; then
		return
	fi
	local stock_key="seckill:stock:${ACTIVITY_NO}:${SKU_NO}"
	local purchase_key="seckill:purchase:${USER_ID}:${ACTIVITY_NO}:${SKU_NO}"
	if redis_cmd SET "$stock_key" "$STOCK_SEED" && redis_cmd DEL "$purchase_key"; then
		log "seeded Redis stock $stock_key=$STOCK_SEED"
		return
	fi
	log "skip Redis stock seed; redis-cli/docker compose redis is unavailable"
}

need curl
need python3
if [[ -z "$TRACE_ID" ]]; then
	TRACE_ID="$(generate_trace_id)"
fi

log "base=$BASE_URL user=$USER_ID activity=$ACTIVITY_NO sku=$SKU_NO trace=$TRACE_ID"
seed_stock

health_file="$tmpdir/health.json"
wait_for_health "$health_file"
log "gateway health ok"

precheck_file="$tmpdir/precheck.json"
http_request GET "$BASE_URL/api/seckill/pre-check?activityNo=$ACTIVITY_NO" "" "$precheck_file"
passed="$(json_get "$precheck_file" "data.passed" || true)"
if [[ "$passed" != "true" ]]; then
	cat "$precheck_file" >&2
	fail "pre-check did not pass"
fi
log "pre-check passed"

partin_file="$tmpdir/partin.json"
partin_body="{\"activityNo\":\"$ACTIVITY_NO\",\"skuNo\":\"$SKU_NO\",\"quantity\":$QUANTITY,\"machineToken\":\"smoke\"}"
http_request POST "$BASE_URL/api/seckill/part-in" "$partin_body" "$partin_file"
trace_id="$(json_get "$partin_file" "data.traceId" || true)"
if [[ -z "$trace_id" ]]; then
	cat "$partin_file" >&2
	fail "part-in response did not contain data.traceId"
fi
log "part-in queued traceId=$trace_id"

order_no=""
for ((i = 1; i <= POLL_ATTEMPTS; i++)); do
	check_file="$tmpdir/check-$i.json"
	http_request POST "$BASE_URL/api/seckill/queue/check" "{\"traceId\":\"$trace_id\"}" "$check_file"
	poll_status="$(json_get "$check_file" "data.pollStatus" || true)"
	case "$poll_status" in
		2)
			order_no="$(json_get "$check_file" "data.orderNo" || true)"
			[[ -n "$order_no" ]] || fail "queue success without orderNo"
			log "queue success orderNo=$order_no"
			break
			;;
		0)
			reason="$(json_get "$check_file" "data.reason" || true)"
			cat "$check_file" >&2
			fail "queue stopped: ${reason:-unknown}"
			;;
		*)
			log "queue pending attempt=$i"
			sleep "$POLL_INTERVAL"
			;;
	esac
done
[[ -n "$order_no" ]] || fail "queue did not produce an order after $POLL_ATTEMPTS attempts"

prepay_file="$tmpdir/prepay.json"
http_request POST "$BASE_URL/api/pay/prepay?orderNo=$order_no&payChannel=$PAY_CHANNEL" "" "$prepay_file"
prepay_id="$(json_get "$prepay_file" "data.prepayId" || true)"
[[ -n "$prepay_id" ]] || fail "prepay response did not contain data.prepayId"
log "prepay created prepayId=$prepay_id"

transaction_no="smoke_tx_${order_no}_$(date +%s)"
notify_file="$tmpdir/notify.txt"
http_request POST "$BASE_URL/api/pay/notify/$PAY_CHANNEL" "{\"orderNo\":\"$order_no\",\"transactionNo\":\"$transaction_no\"}" "$notify_file"
if ! grep -q SUCCESS "$notify_file"; then
	cat "$notify_file" >&2
	fail "notify did not return SUCCESS"
fi
log "payment notify success transactionNo=$transaction_no"

order_file="$tmpdir/order.json"
http_request GET "$BASE_URL/api/orders/$order_no" "" "$order_file"
status="$(json_get "$order_file" "data.status" || true)"
if [[ "$status" != "PAID" ]]; then
	cat "$order_file" >&2
	fail "order status = ${status:-missing}, want PAID"
fi
log "order paid"
log "smoke passed"
