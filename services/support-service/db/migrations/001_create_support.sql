-- +migrate Up
CREATE TABLE IF NOT EXISTS t_user (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    phone VARCHAR(20) UNIQUE,
    nickname VARCHAR(50),
    member_level SMALLINT NOT NULL DEFAULT 0,
    status SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS t_payment (
    id BIGSERIAL PRIMARY KEY,
    payment_no VARCHAR(32) NOT NULL UNIQUE,
    order_no VARCHAR(32) NOT NULL,
    user_id BIGINT NOT NULL,
    pay_amount BIGINT NOT NULL,
    pay_channel VARCHAR(20) NOT NULL DEFAULT 'MOCK',
    pay_status SMALLINT NOT NULL DEFAULT 0,
    transaction_no VARCHAR(64),
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_payment_order ON t_payment(order_no);
CREATE INDEX IF NOT EXISTS idx_payment_user ON t_payment(user_id);

CREATE TABLE IF NOT EXISTS t_free_card (
    id BIGSERIAL PRIMARY KEY,
    card_no VARCHAR(32) NOT NULL UNIQUE,
    card_name VARCHAR(100),
    face_value BIGINT NOT NULL,
    user_id BIGINT,
    order_no VARCHAR(32),
    status SMALLINT NOT NULL DEFAULT 0,
    valid_days INT NOT NULL DEFAULT 365,
    activated_at TIMESTAMPTZ,
    expire_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +migrate Down
DROP TABLE IF EXISTS t_free_card;
DROP TABLE IF EXISTS t_payment;
DROP TABLE IF EXISTS t_user;
