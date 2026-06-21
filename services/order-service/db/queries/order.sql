-- name: CreateOrder :exec
INSERT INTO sk_order (
    order_no, user_id, activity_no, sku_no, quantity,
    total_amount, discount_amount, pay_amount, order_status,
    trace_id, remark
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11
);

-- name: CreateOrderItem :exec
INSERT INTO sk_order_item (
    order_no, activity_no, sku_no, product_name,
    quantity, price, total_amount
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7
);

-- name: GetOrder :one
SELECT * FROM sk_order
WHERE order_no = $1 AND is_deleted = 0;

-- name: GetOrderByUserAndOrderNo :one
SELECT * FROM sk_order
WHERE user_id = $1 AND order_no = $2 AND is_deleted = 0;

-- name: ListOrdersByUser :many
SELECT * FROM sk_order
WHERE user_id = $1 AND is_deleted = 0
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListOrdersByActivity :many
SELECT * FROM sk_order
WHERE activity_no = $1 AND is_deleted = 0
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListOrdersByActivities :many
SELECT * FROM sk_order
WHERE activity_no = ANY($1::varchar[])
  AND is_deleted = 0
  AND created_at > NOW() - INTERVAL '7 days'
ORDER BY created_at DESC
LIMIT 10000;

-- name: MarkOrderPaid :exec
UPDATE sk_order
SET order_status = 'PAID',
    transaction_no = $2,
    paid_at = NOW(),
    updated_at = NOW()
WHERE order_no = $1 AND order_status = 'PENDING_PAY' AND is_deleted = 0;

-- name: CloseOrder :exec
UPDATE sk_order
SET order_status = 'CLOSED',
    closed_at = NOW(),
    updated_at = NOW()
WHERE order_no = $1 AND order_status = 'PENDING_PAY' AND is_deleted = 0;

-- name: SoftDeleteOrder :exec
UPDATE sk_order
SET is_deleted = 1, updated_at = NOW()
WHERE order_no = $1;

-- name: CountOrdersByActivity :one
SELECT COUNT(*) FROM sk_order
WHERE activity_no = $1 AND is_deleted = 0;
