-- name: CreateActivity :exec
INSERT INTO sk_activity (
    activity_no, activity_name,
    start_time, end_time,
    effective_type, effective_days, effective_start, effective_end,
    activity_status, purchase_limit, remark
) VALUES (
    $1, $2,
    $3, $4,
    $5, $6, $7, $8,
    $9, $10, $11
);

-- name: UpdateActivity :exec
UPDATE sk_activity
SET activity_name   = $2,
    start_time      = $3,
    end_time        = $4,
    activity_status = $5,
    purchase_limit  = $6,
    remark          = $7,
    updated_at      = NOW()
WHERE activity_no = $1
  AND is_deleted = 0;

-- name: GetActivity :one
SELECT *
FROM sk_activity
WHERE activity_no = $1
  AND is_deleted = 0;

-- name: ListActivities :many
SELECT *
FROM sk_activity
WHERE (sqlc.narg('activity_status') IS NULL OR activity_status = @activity_status)
  AND is_deleted = 0
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateActivityStatus :exec
UPDATE sk_activity
SET activity_status = $2,
    updated_at      = NOW()
WHERE activity_no = $1
  AND is_deleted = 0;

-- ------------------------------------------------------------------
-- Product queries
-- ------------------------------------------------------------------

-- name: CreateProduct :exec
INSERT INTO sk_activity_product (
    activity_no, product_name, product_image,
    original_price, discount_type, discount_price, sort_order
) VALUES (
    $1, $2, $3,
    $4, $5, $6, $7
);

-- name: ListProductsByActivity :many
SELECT *
FROM sk_activity_product
WHERE activity_no = $1
  AND is_deleted = 0
ORDER BY sort_order ASC;

-- ------------------------------------------------------------------
-- SKU queries
-- ------------------------------------------------------------------

-- name: CreateSKU :exec
INSERT INTO sk_activity_product_sku (
    activity_no, product_id, sku_no,
    activity_stock, discount_type, discount_percent, discount_price
) VALUES (
    $1, $2, $3,
    $4, $5, $6, $7
);

-- name: ListSKUsByActivity :many
SELECT *
FROM sk_activity_product_sku
WHERE activity_no = $1
  AND is_deleted = 0
ORDER BY id ASC;

-- name: GetSKU :one
SELECT *
FROM sk_activity_product_sku
WHERE sku_no = $1
  AND is_deleted = 0;

-- ------------------------------------------------------------------
-- Runtime product queries
-- ------------------------------------------------------------------

-- name: CreateRuntimeProduct :exec
INSERT INTO sk_product (
    activity_no, product_name, product_image, sku_no,
    original_price, seckill_price,
    total_stock, available_stock, limit_quantity
) VALUES (
    $1, $2, $3, $4,
    $5, $6,
    $7, $8, $9
);

-- name: GetRuntimeProduct :one
SELECT *
FROM sk_product
WHERE sku_no = $1
  AND is_deleted = 0;

-- name: UpdateRuntimeStock :exec
UPDATE sk_product
SET available_stock = available_stock - $2,
    updated_at      = NOW()
WHERE sku_no        = $1
  AND is_deleted     = 0
  AND available_stock >= $2;
