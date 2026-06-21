-- +migrate Up

-- 1. DROP 旧单表（空表，安全操作）
DROP TABLE IF EXISTS sk_order_item;
DROP TABLE IF EXISTS sk_order;

-- 2. 创建 sk_order 分区父表
CREATE TABLE sk_order (
    id BIGSERIAL,
    order_no VARCHAR(32) NOT NULL,
    user_id BIGINT NOT NULL,
    activity_no VARCHAR(32) NOT NULL,
    sku_no VARCHAR(32) NOT NULL,
    quantity BIGINT NOT NULL,
    total_amount BIGINT NOT NULL DEFAULT 0,
    discount_amount BIGINT NOT NULL DEFAULT 0,
    pay_amount BIGINT NOT NULL,
    order_status VARCHAR(20) NOT NULL DEFAULT 'PENDING_PAY',
    paid_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    transaction_no VARCHAR(64),
    trace_id VARCHAR(64),
    remark VARCHAR(200) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_deleted SMALLINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id, user_id),
    UNIQUE (user_id, order_no)
) PARTITION BY HASH(user_id);

-- 3. 创建 sk_order 子分区
CREATE TABLE sk_order_p0 PARTITION OF sk_order FOR VALUES WITH (MODULUS 4, REMAINDER 0);
CREATE TABLE sk_order_p1 PARTITION OF sk_order FOR VALUES WITH (MODULUS 4, REMAINDER 1);
CREATE TABLE sk_order_p2 PARTITION OF sk_order FOR VALUES WITH (MODULUS 4, REMAINDER 2);
CREATE TABLE sk_order_p3 PARTITION OF sk_order FOR VALUES WITH (MODULUS 4, REMAINDER 3);

-- 4. 创建 sk_order 索引（在父表上创建，自动应用到所有分区）
CREATE INDEX idx_order_user ON sk_order(user_id);
CREATE INDEX idx_order_activity ON sk_order(activity_no);
CREATE INDEX idx_order_status ON sk_order(order_status);
CREATE INDEX idx_order_created ON sk_order(created_at);

-- 5. 创建 sk_order_item 分区父表
CREATE TABLE sk_order_item (
    id BIGSERIAL,
    order_no VARCHAR(32) NOT NULL,
    activity_no VARCHAR(32) NOT NULL,
    sku_no VARCHAR(32) NOT NULL,
    product_name VARCHAR(100) NOT NULL,
    quantity BIGINT NOT NULL,
    price BIGINT NOT NULL,
    total_amount BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_deleted SMALLINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id, order_no)
) PARTITION BY HASH(order_no);

-- 6. 创建 sk_order_item 子分区
CREATE TABLE sk_order_item_p0 PARTITION OF sk_order_item FOR VALUES WITH (MODULUS 4, REMAINDER 0);
CREATE TABLE sk_order_item_p1 PARTITION OF sk_order_item FOR VALUES WITH (MODULUS 4, REMAINDER 1);
CREATE TABLE sk_order_item_p2 PARTITION OF sk_order_item FOR VALUES WITH (MODULUS 4, REMAINDER 2);
CREATE TABLE sk_order_item_p3 PARTITION OF sk_order_item FOR VALUES WITH (MODULUS 4, REMAINDER 3);

-- 7. 创建 sk_order_item 索引
CREATE INDEX idx_item_order ON sk_order_item(order_no);
CREATE INDEX idx_item_sku ON sk_order_item(sku_no);

-- +migrate Down
DROP TABLE IF EXISTS sk_order_item;
DROP TABLE IF EXISTS sk_order;
