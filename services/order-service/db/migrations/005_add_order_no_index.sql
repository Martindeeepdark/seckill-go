-- +migrate Up
-- 添加 order_no 索引以优化 CloseOrder 性能
-- 问题：当前只有组合索引 (user_id, order_no)，按 order_no 查询时无法使用
-- 场景：支付超时处理、订单查询等场景需要按 order_no 快速定位
-- 注意：分区表不支持 CONCURRENTLY，直接创建索引
CREATE INDEX IF NOT EXISTS idx_order_order_no ON sk_order (order_no);

COMMENT ON INDEX idx_order_order_no IS '订单号索引，用于订单查询和状态更新（如支付超时关闭订单）';

-- +migrate Down
DROP INDEX IF EXISTS idx_order_order_no;
