#!/bin/bash
# 多实例集成测试脚本 - 验证多 Gateway 实例共享 Redis 限流计数器

set -e

echo "=== Redis 用户限流多实例集成测试 ==="

# 清理函数
cleanup() {
    echo "4. 清理环境..."
    if [ -n "$PID1" ]; then
        kill $PID1 2>/dev/null || true
    fi
    if [ -n "$PID2" ]; then
        kill $PID2 2>/dev/null || true
    fi
    docker stop test-redis 2>/dev/null || true
    docker rm test-redis 2>/dev/null || true
}

# 注册清理函数
trap cleanup EXIT

# 启动 Redis
echo "1. 启动 Redis..."
docker rm -f test-redis 2>/dev/null || true
docker run -d --name test-redis -p 6379:6379 redis:7-alpine
sleep 2

# 检查 Redis 是否启动成功
if ! docker ps | grep -q test-redis; then
    echo "✗ Redis 启动失败"
    exit 1
fi

# 构建 Gateway
echo "2. 构建 Gateway 实例..."
cd /Users/wenyz/Documents/seckill/services/seckill-gateway
go build -o /tmp/gateway-test cmd/main.go

# 启动 2 个 Gateway 实例（端口 8080 和 8081）
echo "3. 启动 2 个 Gateway 实例..."

# 复制配置文件并修改端口
cp configs/config.yaml /tmp/config-8080.yaml
cp configs/config.yaml /tmp/config-8081.yaml

# 修改实例 2 的端口为 8081
sed -i.bak 's/addr: :8080/addr: :8081/' /tmp/config-8081.yaml

# 启动实例 1 (端口 8080)
/tmp/gateway-test /tmp/config-8080.yaml > /tmp/gateway-8080.log 2>&1 &
PID1=$!

# 启动实例 2 (端口 8081)
/tmp/gateway-test /tmp/config-8081.yaml > /tmp/gateway-8081.log 2>&1 &
PID2=$!

# 等待 Gateway 启动
echo "   等待 Gateway 启动..."
sleep 5

# 检查实例是否启动成功
if ! ps -p $PID1 > /dev/null; then
    echo "✗ Gateway 实例 1 启动失败"
    cat /tmp/gateway-8080.log
    exit 1
fi

if ! ps -p $PID2 > /dev/null; then
    echo "✗ Gateway 实例 2 启动失败"
    cat /tmp/gateway-8081.log
    exit 1
fi

# 清空 Redis 限流计数器
echo "4. 清空 Redis 限流计数器..."
docker exec test-redis redis-cli FLUSHALL > /dev/null

# 测试用户限流
echo "5. 测试用户限流（10 次 / 10 秒）..."
USER_ID=10001
TOKEN="test-token"

SUCCESS_COUNT=0
RATE_LIMITED_COUNT=0

# 向实例 1 发送 5 次请求
echo "   向实例 1 (8080) 发送 5 次请求..."
for i in {1..5}; do
    CODE=$(curl -s -X POST http://localhost:8080/api/seckill/part-in \
        -H "X-User-Id: $USER_ID" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"activityNo": "ACT001"}' | jq -r '.code' || echo "error")

    if [ "$CODE" == "success" ] || [ "$CODE" == "0" ]; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    elif [ "$CODE" == "rate_limited" ]; then
        RATE_LIMITED_COUNT=$((RATE_LIMITED_COUNT + 1))
    fi
    echo "   请求 $i: code=$CODE"
done

# 向实例 2 发送 5 次请求
echo "   向实例 2 (8081) 发送 5 次请求..."
for i in {6..10}; do
    CODE=$(curl -s -X POST http://localhost:8081/api/seckill/part-in \
        -H "X-User-Id: $USER_ID" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"activityNo": "ACT001"}' | jq -r '.code' || echo "error")

    if [ "$CODE" == "success" ] || [ "$CODE" == "0" ]; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    elif [ "$CODE" == "rate_limited" ]; then
        RATE_LIMITED_COUNT=$((RATE_LIMITED_COUNT + 1))
    fi
    echo "   请求 $i: code=$CODE"
done

# 第 11 次请求应该被拒绝
echo "6. 发送第 11 次请求（应该被拒绝）..."
RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST http://localhost:8080/api/seckill/part-in \
    -H "X-User-Id: $USER_ID" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"activityNo": "ACT001"}')

HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | grep -v "HTTP_CODE:")
CODE=$(echo "$BODY" | jq -r '.code' 2>/dev/null || echo "error")

echo "   HTTP 状态码: $HTTP_CODE"
echo "   响应 code: $CODE"

# 验证结果
echo ""
echo "=== 测试结果 ==="
echo "前 10 次请求: 成功 $SUCCESS_COUNT, 限流 $RATE_LIMITED_COUNT"
echo "第 11 次请求: HTTP $HTTP_CODE, code=$CODE"

if [ "$HTTP_CODE" == "429" ] && [ "$CODE" == "rate_limited" ]; then
    echo "✓ 第 11 次请求被正确拒绝"
    echo "✓ 多实例限流共享验证通过"
    exit 0
else
    echo "✗ 第 11 次请求未被正确拒绝"
    echo "  期望: HTTP 429, code=rate_limited"
    echo "  实际: HTTP $HTTP_CODE, code=$CODE"
    echo ""
    echo "Gateway 实例 1 日志:"
    tail -20 /tmp/gateway-8080.log
    echo ""
    echo "Gateway 实例 2 日志:"
    tail -20 /tmp/gateway-8081.log
    exit 1
fi
