-- name: CreateRiskRecord :exec
INSERT INTO sk_risk_record (user_id, action_type, risk_level, request_ip, request_info)
VALUES ($1, $2, $3, $4, $5);

-- name: ListRiskRecordsByUser :many
SELECT * FROM sk_risk_record
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: CountRecentActions :one
SELECT COUNT(*) FROM sk_risk_record
WHERE user_id = $1 AND action_type = $2 AND created_at > $3;

-- name: HasHighRiskRecord :one
SELECT EXISTS(
    SELECT 1 FROM sk_risk_record
    WHERE user_id = $1 AND risk_level >= 2 AND created_at > $2
);

-- name: CleanupExpiredRecords :exec
DELETE FROM sk_risk_record WHERE created_at < $1;
