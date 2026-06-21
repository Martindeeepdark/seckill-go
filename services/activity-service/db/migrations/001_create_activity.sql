-- +migrate Up
CREATE TABLE IF NOT EXISTS sk_activity (
    id              BIGSERIAL       PRIMARY KEY,
    activity_no     VARCHAR(32)     NOT NULL,
    activity_name   VARCHAR(50)     NOT NULL,
    start_time      TIMESTAMPTZ     NOT NULL,
    end_time        TIMESTAMPTZ     NOT NULL,
    effective_type  SMALLINT        NOT NULL DEFAULT 0,
    effective_days  VARCHAR(20),
    effective_start VARCHAR(20),
    effective_end   VARCHAR(20),
    activity_status SMALLINT        NOT NULL DEFAULT 0,
    purchase_limit  INT             NOT NULL DEFAULT 0,
    remark          VARCHAR(500),
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    is_deleted      SMALLINT        NOT NULL DEFAULT 0,
    CONSTRAINT uq_activity_no UNIQUE (activity_no)
);

COMMENT ON TABLE  sk_activity            IS 'activity main table';
COMMENT ON COLUMN sk_activity.activity_no    IS 'unique business key for the activity';
COMMENT ON COLUMN sk_activity.effective_type IS '0: whole period, 1: specific days/time range';
COMMENT ON COLUMN sk_activity.effective_days IS 'comma-separated day numbers, e.g. 1,2,3';
COMMENT ON COLUMN sk_activity.effective_start IS 'daily effective start time, stored as HH:MM';
COMMENT ON COLUMN sk_activity.effective_end   IS 'daily effective end time, stored as HH:MM';
COMMENT ON COLUMN sk_activity.activity_status IS '0: draft, 1: pending, 2: live, 3: ended, 4: cancelled';
COMMENT ON COLUMN sk_activity.is_deleted      IS 'soft delete flag, 0: active, 1: deleted';

-- ------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS sk_activity_product (
    id              BIGSERIAL       PRIMARY KEY,
    activity_no     VARCHAR(32)     NOT NULL,
    product_name    VARCHAR(100)    NOT NULL,
    product_image   VARCHAR(500),
    original_price  BIGINT          NOT NULL,
    discount_type   SMALLINT        NOT NULL DEFAULT 0,
    discount_price  BIGINT,
    sort_order      INT             NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    is_deleted      SMALLINT        NOT NULL DEFAULT 0
);

CREATE INDEX idx_activity_product_activity_no ON sk_activity_product (activity_no);

COMMENT ON TABLE  sk_activity_product                IS 'product config per activity';
COMMENT ON COLUMN sk_activity_product.discount_type   IS '0: none, 1: percent, 2: fixed price';
COMMENT ON COLUMN sk_activity_product.discount_price  IS 'discount amount in cents';

-- ------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS sk_activity_product_sku (
    id                BIGSERIAL       PRIMARY KEY,
    activity_no       VARCHAR(32)     NOT NULL,
    product_id        BIGINT          NOT NULL,
    sku_no            VARCHAR(32)     NOT NULL,
    activity_stock    INT             NOT NULL,
    discount_type     SMALLINT        NOT NULL DEFAULT 0,
    discount_percent  INT,
    discount_price    BIGINT,
    created_at        TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    is_deleted        SMALLINT        NOT NULL DEFAULT 0,
    CONSTRAINT uq_sku_no UNIQUE (sku_no)
);

CREATE INDEX idx_sku_activity_no ON sk_activity_product_sku (activity_no);
CREATE INDEX idx_sku_product_id  ON sk_activity_product_sku (product_id);
CREATE INDEX idx_sku_sku_no      ON sk_activity_product_sku (sku_no);

COMMENT ON TABLE  sk_activity_product_sku                  IS 'SKU-level config (price, stock) per activity product';
COMMENT ON COLUMN sk_activity_product_sku.discount_type     IS '0: none, 1: percent, 2: fixed price';
COMMENT ON COLUMN sk_activity_product_sku.discount_percent  IS 'percentage discount, e.g. 20 means 20% off';
COMMENT ON COLUMN sk_activity_product_sku.discount_price    IS 'fixed discount price in cents';

-- ------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS sk_product (
    id               BIGSERIAL       PRIMARY KEY,
    activity_no      VARCHAR(32)     NOT NULL,
    product_name     VARCHAR(100)    NOT NULL,
    product_image    VARCHAR(500),
    sku_no           VARCHAR(32)     NOT NULL,
    original_price   BIGINT          NOT NULL,
    seckill_price    BIGINT          NOT NULL,
    total_stock      INT             NOT NULL,
    available_stock  INT             NOT NULL,
    limit_quantity   INT             NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    is_deleted       SMALLINT        NOT NULL DEFAULT 0,
    CONSTRAINT uq_product_sku_no UNIQUE (sku_no)
);

CREATE INDEX idx_product_activity_no ON sk_product (activity_no);

COMMENT ON TABLE  sk_product                  IS 'runtime product state generated when activity goes live';
COMMENT ON COLUMN sk_product.seckill_price     IS 'final seckill price in cents';
COMMENT ON COLUMN sk_product.available_stock   IS 'remaining stock available for purchase';
COMMENT ON COLUMN sk_product.limit_quantity    IS 'max quantity per user per activity';

-- +migrate Down
DROP TABLE IF EXISTS sk_product;
DROP TABLE IF EXISTS sk_activity_product_sku;
DROP TABLE IF EXISTS sk_activity_product;
DROP TABLE IF EXISTS sk_activity;
