-- +migrate Up
CREATE TABLE IF NOT EXISTS sk_risk_record (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    action_type VARCHAR(20) NOT NULL,
    risk_level SMALLINT NOT NULL DEFAULT 0,
    request_ip VARCHAR(50),
    request_info VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_risk_user ON sk_risk_record(user_id);
CREATE INDEX IF NOT EXISTS idx_risk_created ON sk_risk_record(created_at);

-- +migrate Down
DROP TABLE IF EXISTS sk_risk_record;
