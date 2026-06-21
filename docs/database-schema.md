# 数据库表结构说明

## 数据库架构

项目采用 **多数据库拆分** + **分区表** 设计：

```
seckill_activity    # 活动服务数据库
seckill_order       # 订单服务数据库（分区表）
seckill_risk        # 风控服务数据库
seckill_support     # 支撑服务数据库（用户、支付、免单卡）
```

## 订单服务（seckill_order）

### 1. sk_order（秒杀订单表）

**分区策略：** 按 `user_id` HASH 分区，4 个子分区（`sk_order_p0` ~ `sk_order_p3`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGSERIAL | 订单主键 ID |
| `order_no` | VARCHAR(32) | 订单编号（唯一，32 字符） |
| `user_id` | BIGINT | 用户 ID（分区键） |
| `activity_no` | VARCHAR(32) | 活动编号 |
| `sku_no` | VARCHAR(32) | SKU 编号 |
| `quantity` | BIGINT | 购买数量 |
| `total_amount` | BIGINT | 总金额（单位：分） |
| `discount_amount` | BIGINT | 优惠金额（单位：分） |
| `pay_amount` | BIGINT | 实付金额（单位：分） |
| `order_status` | VARCHAR(20) | 订单状态：PENDING_PAY（待支付）、PAID（已支付）、CLOSED（已关闭） |
| `paid_at` | TIMESTAMPTZ | 支付时间 |
| `closed_at` | TIMESTAMPTZ | 关闭时间 |
| `transaction_no` | VARCHAR(64) | 支付交易号 |
| `trace_id` | VARCHAR(64) | 链路追踪 ID（用于幂等性保证） |
| `remark` | VARCHAR(200) | 备注信息 |
| `created_at` | TIMESTAMPTZ | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 更新时间 |
| `is_deleted` | SMALLINT | 软删除标识（0：未删除，1：已删除） |

**索引：**
- PRIMARY KEY: `(id, user_id)`
- UNIQUE: `(user_id, order_no)` — 用户维度订单号唯一
- UNIQUE: `uk_sk_order_user_trace (user_id, trace_id)` — 幂等性保证
- INDEX: `idx_order_activity (activity_no)` — 活动维度查询
- INDEX: `idx_order_created (created_at)` — 时间范围查询
- INDEX: `idx_order_status (order_status)` — 状态过滤

### 2. sk_order_item（秒杀订单明细表）

**分区策略：** 按 `order_no` HASH 分区，4 个子分区

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGSERIAL | 明细主键 ID |
| `order_no` | VARCHAR(32) | 订单编号（分区键） |
| `activity_no` | VARCHAR(32) | 活动编号 |
| `sku_no` | VARCHAR(32) | SKU 编号 |
| `product_name` | VARCHAR(100) | 商品名称 |
| `quantity` | BIGINT | 购买数量 |
| `price` | BIGINT | 单价（单位：分） |
| `total_amount` | BIGINT | 小计金额（单位：分） |
| `created_at` | TIMESTAMPTZ | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 更新时间 |
| `is_deleted` | SMALLINT | 软删除标识 |

---

## 支撑服务（seckill_support）

### 1. t_user（用户信息表）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGSERIAL | 用户主键 ID |
| `username` | VARCHAR(50) | 用户名（唯一） |
| `phone` | VARCHAR(20) | 手机号（唯一） |
| `nickname` | VARCHAR(50) | 昵称 |
| `member_level` | SMALLINT | 会员等级：0（普通）、1（VIP1）、2（VIP2）等 |
| `status` | SMALLINT | 用户状态：0（正常）、1（冻结） |
| `created_at` | TIMESTAMPTZ | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 更新时间 |

### 2. t_payment（支付信息表）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGSERIAL | 支付主键 ID |
| `payment_no` | VARCHAR(32) | 支付单号（唯一） |
| `order_no` | VARCHAR(32) | 关联订单号 |
| `user_id` | BIGINT | 用户 ID |
| `pay_amount` | BIGINT | 支付金额（单位：分） |
| `pay_channel` | VARCHAR(20) | 支付渠道：MOCK（模拟）、ALIPAY、WECHAT 等 |
| `pay_status` | SMALLINT | 支付状态：0（待支付）、1（已支付）、2（已取消）、3（已退款） |
| `transaction_no` | VARCHAR(64) | 第三方支付平台交易号 |
| `paid_at` | TIMESTAMPTZ | 支付完成时间 |
| `created_at` | TIMESTAMPTZ | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 更新时间 |

### 3. t_free_card（免单卡信息表）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGSERIAL | 免单卡主键 ID |
| `card_no` | VARCHAR(32) | 免单卡卡号（唯一） |
| `card_name` | VARCHAR(100) | 免单卡名称 |
| `face_value` | BIGINT | 面值（单位：分） |
| `user_id` | BIGINT | 绑定用户 ID（NULL 表示未绑定） |
| `order_no` | VARCHAR(32) | 使用的订单号（NULL 表示未使用） |
| `status` | SMALLINT | 卡状态：0（未激活）、1（已激活）、2（已使用）、3（已过期） |
| `valid_days` | INT | 有效天数（默认 365 天） |
| `activated_at` | TIMESTAMPTZ | 激活时间 |
| `expire_at` | TIMESTAMPTZ | 过期时间 |
| `created_at` | TIMESTAMPTZ | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 更新时间 |

---

## 风控服务（seckill_risk）

### sk_risk_record（风控记录表）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGSERIAL | 风控记录主键 ID |
| `user_id` | BIGINT | 用户 ID |
| `action_type` | VARCHAR(20) | 行为类型：SECKILL、LOGIN、REGISTER 等 |
| `risk_level` | SMALLINT | 风险等级：0（正常）、1（低风险）、2（中风险）、3（高风险） |
| `request_ip` | VARCHAR(50) | 请求来源 IP 地址 |
| `request_info` | VARCHAR(500) | 请求详细信息（JSON 格式） |
| `created_at` | TIMESTAMPTZ | 创建时间 |

---

## 活动服务（seckill_activity）

详见 [services/activity-service/db/migrations/001_create_activity.sql](../services/activity-service/db/migrations/001_create_activity.sql)

**核心表：**
- `sk_activity` — 活动主表（已有英文注释）
- `sk_activity_product` — 活动商品配置
- `sk_activity_product_sku` — SKU 级别配置
- `sk_product` — 运行时商品状态

---

## 初始化数据库

```bash
# 1. 启动 Docker Compose
make docker-up

# 2. Migration 会自动执行（通过 sql-migrate 或 golang-migrate）

# 3. 手动应用注释（如果 migration 未包含）
docker exec -i seckill-postgres-1 psql -U seckill -d seckill_order < services/order-service/db/migrations/004_add_table_comments.sql
docker exec -i seckill-postgres-1 psql -U seckill -d seckill_support < services/support-service/db/migrations/003_add_table_comments.sql
docker exec -i seckill-postgres-1 psql -U seckill -d seckill_risk < services/risk-service/db/migrations/002_add_table_comments.sql
```

---

## 分区表设计原理

### 为什么只有订单表分区？

1. **订单表是高并发写入热点** — 秒杀场景订单创建 TPS 极高
2. **分区降低锁竞争** — 不同用户的订单落在不同分区，减少锁冲突
3. **查询路由优化** — 按 `user_id` 查询可以直接定位到单个分区

### 为什么是 HASH 分区而非 RANGE 分区？

- **数据均匀分布** — HASH 分区保证每个分区数据量相近
- **避免热点分区** — RANGE 按时间分区会导致新分区成为热点
- **用户维度隔离** — HASH(user_id) 实现用户维度的天然隔离

### 4 个分区够用吗？

- **起步阶段足够** — 单表百万级别数据，4 分区已有明显性能提升
- **可扩展** — 后续可以增加分区（需要重建表）
- **平衡复杂度与收益** — 太多分区增加管理复杂度

---

## 性能优化要点

1. **分区剪枝** — 查询带 `user_id` 条件时，PostgreSQL 只扫描对应分区
2. **并行查询** — 跨分区查询自动启用并行工作进程
3. **时间窗口过滤** — `ListOrdersByActivities` 添加 7 天时间窗口，避免全表扫描
4. **LIMIT 保护** — 限制最大返回 10000 条，防止 OOM

---

## 注释完整性检查

```sql
-- 查看表注释
SELECT 
    schemaname, 
    tablename, 
    obj_description((schemaname || '.' || tablename)::regclass) AS table_comment
FROM pg_tables 
WHERE schemaname = 'public';

-- 查看列注释
SELECT 
    a.attname AS column_name,
    col_description(a.attrelid, a.attnum) AS column_comment
FROM pg_attribute a
WHERE a.attrelid = 'sk_order'::regclass 
  AND a.attnum > 0 
  AND NOT a.attisdropped
ORDER BY a.attnum;
```

---

**最后更新：** 2026-06-21
