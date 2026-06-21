-- +migrate Up
-- 添加订单号分片标识提取函数
-- 订单号格式：O{snowflakeID}{shardID}，最后一位是分片标识(0-3)
-- 用于优化按订单号查询时的分区裁剪
CREATE OR REPLACE FUNCTION extract_order_shard(order_no VARCHAR) RETURNS INT AS $$
BEGIN
    -- 提取订单号最后一位作为分片标识
    RETURN CAST(RIGHT(order_no, 1) AS INT);
EXCEPTION
    WHEN OTHERS THEN
        -- 如果提取失败（老订单号格式），返回 -1 表示无法裁剪
        RETURN -1;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

COMMENT ON FUNCTION extract_order_shard(VARCHAR) IS '从订单号提取分片标识，用于分区裁剪优化';

-- 为每个分区创建检查约束，帮助 PostgreSQL 进行分区裁剪
-- sk_order_p0: user_id % 4 = 0，订单号最后一位应该是 0
ALTER TABLE sk_order_p0 ADD CONSTRAINT chk_order_shard_p0
    CHECK (extract_order_shard(order_no) IN (-1, 0));

-- sk_order_p1: user_id % 4 = 1，订单号最后一位应该是 1
ALTER TABLE sk_order_p1 ADD CONSTRAINT chk_order_shard_p1
    CHECK (extract_order_shard(order_no) IN (-1, 1));

-- sk_order_p2: user_id % 4 = 2，订单号最后一位应该是 2
ALTER TABLE sk_order_p2 ADD CONSTRAINT chk_order_shard_p2
    CHECK (extract_order_shard(order_no) IN (-1, 2));

-- sk_order_p3: user_id % 4 = 3，订单号最后一位应该是 3
ALTER TABLE sk_order_p3 ADD CONSTRAINT chk_order_shard_p3
    CHECK (extract_order_shard(order_no) IN (-1, 3));

-- +migrate Down
ALTER TABLE sk_order_p0 DROP CONSTRAINT IF EXISTS chk_order_shard_p0;
ALTER TABLE sk_order_p1 DROP CONSTRAINT IF EXISTS chk_order_shard_p1;
ALTER TABLE sk_order_p2 DROP CONSTRAINT IF EXISTS chk_order_shard_p2;
ALTER TABLE sk_order_p3 DROP CONSTRAINT IF EXISTS chk_order_shard_p3;
DROP FUNCTION IF EXISTS extract_order_shard(VARCHAR);
