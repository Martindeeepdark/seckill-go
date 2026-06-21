#!/usr/bin/env bash
set -euo pipefail

# 本地编译 + 构建最小 Docker 镜像
# 用法: ./scripts/docker-build.sh [service-name]
# 示例: ./scripts/docker-build.sh stock-service
#       ./scripts/docker-build.sh          (构建全部)

SERVICES=(
  activity-service stock-service risk-service order-service
  support-service seckill-gateway seckill-processor seckill-job
)
BUILDDIR="build"
LDFLAGS="-s -w"

build_binary() {
  local svc=$1
  echo "==> 编译 $svc"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "$LDFLAGS" -o "$BUILDDIR/$svc" "./services/$svc/cmd"
}

build_image() {
  local svc=$1
  echo "==> 构建镜像 $svc"
  docker build \
    --build-arg SERVICE="$svc" \
    -f Dockerfile \
    -t "seckill/$svc:latest" \
    .
}

mkdir -p "$BUILDDIR"

if [ $# -gt 0 ]; then
  build_binary "$1"
  build_image "$1"
else
  for svc in "${SERVICES[@]}"; do
    build_binary "$svc"
    build_image "$svc"
  done
fi

echo "==> 完成"
