-- +migrate Up
ALTER TABLE t_user
    ADD COLUMN IF NOT EXISTS avatar TEXT;
CREATE INDEX IF NOT EXISTS idx_user_phone ON t_user(phone);

ALTER TABLE t_payment
    ADD COLUMN IF NOT EXISTS subject TEXT,
    ADD COLUMN IF NOT EXISTS expire_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS prepay_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS nonce_str VARCHAR(64),
    ADD COLUMN IF NOT EXISTS time_stamp VARCHAR(32),
    ADD COLUMN IF NOT EXISTS sign TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS uk_payment_order ON t_payment(order_no);

CREATE UNIQUE INDEX IF NOT EXISTS uk_free_card_order ON t_free_card(order_no);
CREATE INDEX IF NOT EXISTS idx_free_card_user ON t_free_card(user_id);

CREATE TABLE IF NOT EXISTS t_synced_order (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(32) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    order_source VARCHAR(32) NOT NULL,
    total_amount BIGINT NOT NULL,
    discount_amount BIGINT NOT NULL DEFAULT 0,
    pay_amount BIGINT NOT NULL,
    order_status SMALLINT NOT NULL DEFAULT 1,
    paid_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    transaction_no VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_synced_order_user ON t_synced_order(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_synced_order_created ON t_synced_order(created_at DESC);

-- +migrate Down
DROP TABLE IF EXISTS t_synced_order;
DROP INDEX IF EXISTS idx_free_card_user;
DROP INDEX IF EXISTS uk_free_card_order;
DROP INDEX IF EXISTS uk_payment_order;
ALTER TABLE t_payment
    DROP COLUMN IF EXISTS sign,
    DROP COLUMN IF EXISTS time_stamp,
    DROP COLUMN IF EXISTS nonce_str,
    DROP COLUMN IF EXISTS prepay_id,
    DROP COLUMN IF EXISTS expire_at,
    DROP COLUMN IF EXISTS subject;
DROP INDEX IF EXISTS idx_user_phone;
ALTER TABLE t_user
    DROP COLUMN IF EXISTS avatar;
