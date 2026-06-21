-- +migrate Up
CREATE TABLE IF NOT EXISTS sk_order (
    id BIGSERIAL PRIMARY KEY,
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
    CONSTRAINT uk_order_no UNIQUE (order_no)
);

CREATE INDEX IF NOT EXISTS idx_order_user ON sk_order(user_id);
CREATE INDEX IF NOT EXISTS idx_order_activity ON sk_order(activity_no);
CREATE INDEX IF NOT EXISTS idx_order_status ON sk_order(order_status);
CREATE INDEX IF NOT EXISTS idx_order_created ON sk_order(created_at);

CREATE TABLE IF NOT EXISTS sk_order_item (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(32) NOT NULL,
    activity_no VARCHAR(32) NOT NULL,
    sku_no VARCHAR(32) NOT NULL,
    product_name VARCHAR(100) NOT NULL,
    quantity BIGINT NOT NULL,
    price BIGINT NOT NULL,
    total_amount BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_deleted SMALLINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_item_order ON sk_order_item(order_no);
CREATE INDEX IF NOT EXISTS idx_item_sku ON sk_order_item(sku_no);

-- +migrate Down
DROP TABLE IF EXISTS sk_order_item;
DROP TABLE IF EXISTS sk_order;
