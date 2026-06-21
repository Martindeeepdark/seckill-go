#!/bin/bash
# 将各服务的 config.docker.yaml 转为 JSON 并导入 etcd
# 用法: ./scripts/etcd-init-config.sh [etcd-endpoint]
#
# 依赖: yq (brew install yq)
# etcd-endpoint 默认 http://localhost:12379

set -euo pipefail

ETCD_ENDPOINT="${1:-http://localhost:12379}"
ETCD_PREFIX="/seckill/config"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# 检查 yq
if ! command -v yq &>/dev/null; then
  echo "Error: yq is required. Install with: brew install yq"
  exit 1
fi

# 检查 etcdctl
if ! command -v etcdctl &>/dev/null; then
  echo "Error: etcdctl is required. Install with: brew install etcd"
  exit 1
fi

export ETCDCTL_API=3

echo "=== Importing configs to etcd at ${ETCD_ENDPOINT} ==="

services=(
  "activity-service"
  "stock-service"
  "risk-service"
  "order-service"
  "support-service"
  "seckill-gateway"
  "seckill-job"
  "seckill-processor"
)

for svc in "${services[@]}"; do
  config_file="${PROJECT_DIR}/services/${svc}/configs/config.docker.yaml"
  if [ ! -f "$config_file" ]; then
    echo "SKIP: ${config_file} not found"
    continue
  fi

  # YAML → JSON
  json=$(yq -o=json '.' "$config_file")
  key="${ETCD_PREFIX}/${svc}"

  # 写入 etcd
  echo "$json" | etcdctl --endpoints="$ETCD_ENDPOINT" put "$key"
  echo "OK: ${key} ($(echo "$json" | wc -c | tr -d ' ') bytes)"
done

echo ""
echo "=== Verify ==="
for svc in "${services[@]}"; do
  key="${ETCD_PREFIX}/${svc}"
  count=$(etcdctl --endpoints="$ETCD_ENDPOINT" get "$key" --print-value-only | wc -c | tr -d ' ')
  echo "  ${key}: ${count} bytes"
done

echo ""
echo "Done! View at http://localhost:18080 (etcdkeeper)"
