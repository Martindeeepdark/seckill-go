-- name: GetUser :one
SELECT * FROM t_user WHERE id = $1;

-- name: CreateUser :exec
INSERT INTO t_user (username, phone, nickname, member_level, status)
VALUES ($1, $2, $3, $4, $5);

-- name: CreatePayment :exec
INSERT INTO t_payment (payment_no, order_no, user_id, pay_amount, pay_channel, pay_status)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetPayment :one
SELECT * FROM t_payment WHERE order_no = $1;

-- name: UpdatePaymentStatus :exec
UPDATE t_payment
SET pay_status = $2, transaction_no = $3, paid_at = $4, updated_at = NOW()
WHERE payment_no = $1;

-- name: CreateCard :exec
INSERT INTO t_free_card (card_no, card_name, face_value, valid_days)
VALUES ($1, $2, $3, $4);

-- name: GetCard :one
SELECT * FROM t_free_card WHERE card_no = $1;

-- name: ActivateCard :exec
UPDATE t_free_card
SET user_id = $2, order_no = $3, status = 1, activated_at = NOW(), expire_at = NOW() + (valid_days || ' days')::interval, updated_at = NOW()
WHERE card_no = $1;

-- name: ListCardsByUser :many
SELECT * FROM t_free_card
WHERE user_id = $1
ORDER BY created_at DESC;
