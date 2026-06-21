-- +migrate Up

-- 订单表注释
COMMENT ON TABLE sk_order IS '秒杀订单表（按 user_id HASH 分区，4 个分区）';

COMMENT ON COLUMN sk_order.id IS '订单主键 ID（BIGSERIAL）';
COMMENT ON COLUMN sk_order.order_no IS '订单编号（唯一，32 字符）';
COMMENT ON COLUMN sk_order.user_id IS '用户 ID（分区键）';
COMMENT ON COLUMN sk_order.activity_no IS '活动编号';
COMMENT ON COLUMN sk_order.sku_no IS 'SKU 编号';
COMMENT ON COLUMN sk_order.quantity IS '购买数量';
COMMENT ON COLUMN sk_order.total_amount IS '总金额（单位：分）';
COMMENT ON COLUMN sk_order.discount_amount IS '优惠金额（单位：分）';
COMMENT ON COLUMN sk_order.pay_amount IS '实付金额（单位：分）';
COMMENT ON COLUMN sk_order.order_status IS '订单状态：PENDING_PAY（待支付）、PAID（已支付）、CLOSED（已关闭）';
COMMENT ON COLUMN sk_order.paid_at IS '支付时间';
COMMENT ON COLUMN sk_order.closed_at IS '关闭时间';
COMMENT ON COLUMN sk_order.transaction_no IS '支付交易号';
COMMENT ON COLUMN sk_order.trace_id IS '链路追踪 ID（用于幂等性保证）';
COMMENT ON COLUMN sk_order.remark IS '备注信息';
COMMENT ON COLUMN sk_order.created_at IS '创建时间';
COMMENT ON COLUMN sk_order.updated_at IS '更新时间';
COMMENT ON COLUMN sk_order.is_deleted IS '软删除标识（0：未删除，1：已删除）';

-- 订单明细表注释
COMMENT ON TABLE sk_order_item IS '秒杀订单明细表（按 order_no HASH 分区，4 个分区）';

COMMENT ON COLUMN sk_order_item.id IS '明细主键 ID（BIGSERIAL）';
COMMENT ON COLUMN sk_order_item.order_no IS '订单编号（分区键）';
COMMENT ON COLUMN sk_order_item.activity_no IS '活动编号';
COMMENT ON COLUMN sk_order_item.sku_no IS 'SKU 编号';
COMMENT ON COLUMN sk_order_item.product_name IS '商品名称';
COMMENT ON COLUMN sk_order_item.quantity IS '购买数量';
COMMENT ON COLUMN sk_order_item.price IS '单价（单位：分）';
COMMENT ON COLUMN sk_order_item.total_amount IS '小计金额（单位：分）';
COMMENT ON COLUMN sk_order_item.created_at IS '创建时间';
COMMENT ON COLUMN sk_order_item.updated_at IS '更新时间';
COMMENT ON COLUMN sk_order_item.is_deleted IS '软删除标识（0：未删除，1：已删除）';

-- +migrate Down

COMMENT ON TABLE sk_order IS NULL;
COMMENT ON COLUMN sk_order.id IS NULL;
COMMENT ON COLUMN sk_order.order_no IS NULL;
COMMENT ON COLUMN sk_order.user_id IS NULL;
COMMENT ON COLUMN sk_order.activity_no IS NULL;
COMMENT ON COLUMN sk_order.sku_no IS NULL;
COMMENT ON COLUMN sk_order.quantity IS NULL;
COMMENT ON COLUMN sk_order.total_amount IS NULL;
COMMENT ON COLUMN sk_order.discount_amount IS NULL;
COMMENT ON COLUMN sk_order.pay_amount IS NULL;
COMMENT ON COLUMN sk_order.order_status IS NULL;
COMMENT ON COLUMN sk_order.paid_at IS NULL;
COMMENT ON COLUMN sk_order.closed_at IS NULL;
COMMENT ON COLUMN sk_order.transaction_no IS NULL;
COMMENT ON COLUMN sk_order.trace_id IS NULL;
COMMENT ON COLUMN sk_order.remark IS NULL;
COMMENT ON COLUMN sk_order.created_at IS NULL;
COMMENT ON COLUMN sk_order.updated_at IS NULL;
COMMENT ON COLUMN sk_order.is_deleted IS NULL;

COMMENT ON TABLE sk_order_item IS NULL;
COMMENT ON COLUMN sk_order_item.id IS NULL;
COMMENT ON COLUMN sk_order_item.order_no IS NULL;
COMMENT ON COLUMN sk_order_item.activity_no IS NULL;
COMMENT ON COLUMN sk_order_item.sku_no IS NULL;
COMMENT ON COLUMN sk_order_item.product_name IS NULL;
COMMENT ON COLUMN sk_order_item.quantity IS NULL;
COMMENT ON COLUMN sk_order_item.price IS NULL;
COMMENT ON COLUMN sk_order_item.total_amount IS NULL;
COMMENT ON COLUMN sk_order_item.created_at IS NULL;
COMMENT ON COLUMN sk_order_item.updated_at IS NULL;
COMMENT ON COLUMN sk_order_item.is_deleted IS NULL;
