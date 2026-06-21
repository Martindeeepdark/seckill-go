# 集成测试说明

## 多实例限流测试

**脚本**: `rate_limit_multi_instance_test.sh`

**功能**: 验证多 Gateway 实例共享 Redis 限流计数器

### 前置条件

- Docker 已安装并运行
- Go 1.25+ 已安装
- `jq` 命令行工具已安装（用于解析 JSON 响应）

### 执行方式

```bash
cd /Users/wenyz/Documents/seckill/services/seckill-gateway/test/integration
chmod +x rate_limit_multi_instance_test.sh
./rate_limit_multi_instance_test.sh
```

### 测试场景

1. 启动 Redis 容器（端口 6379）
2. 启动 2 个 Gateway 实例（端口 8080 和 8081）
3. 向实例 1 发送 5 次请求（用户 ID: 10001）
4. 向实例 2 发送 5 次请求（同一用户）
5. 向实例 1 发送第 11 次请求

### 预期结果

- 前 10 次请求通过（HTTP 200，code=success 或 0）
- 第 11 次请求被限流（HTTP 429，code=rate_limited）
- 验证多实例共享 Redis 计数器

### 测试原理

- **限流规则**: 每个用户 10 次请求 / 10 秒
- **共享状态**: 两个 Gateway 实例共享同一个 Redis 实例
- **Redis Key**: `rate_limit:user:<user_id>`
- **验证点**: 跨实例的请求总数达到限流阈值时触发限流

### 清理

脚本会自动清理：
- 停止 2 个 Gateway 进程
- 停止并删除 Redis 容器
- 删除临时配置文件

### 故障排查

如果测试失败，脚本会输出：
- Gateway 实例 1 的最后 20 行日志
- Gateway 实例 2 的最后 20 行日志
- 实际的 HTTP 响应和 code

手动检查日志：
```bash
tail -f /tmp/gateway-8080.log
tail -f /tmp/gateway-8081.log
```

### 注意事项

- 测试会占用端口 6379（Redis）、8080 和 8081（Gateway）
- 确保这些端口未被占用
- 测试会自动清理资源，但如果中断请求手动清理：
  ```bash
  docker stop test-redis && docker rm test-redis
  pkill -f gateway-test
  ```
