-- name: CreatePriceModel :one
INSERT INTO price_models (name, per_minute_rate, unlock_fee, daily_cap, currency)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, per_minute_rate, unlock_fee, daily_cap, currency, created_at, updated_at;

-- name: GetPriceModel :one
SELECT id, name, per_minute_rate, unlock_fee, daily_cap, currency, created_at, updated_at
FROM price_models
WHERE id = $1;

-- name: ListPriceModels :many
SELECT id, name, per_minute_rate, unlock_fee, daily_cap, currency, created_at, updated_at
FROM price_models
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPriceModels :one
SELECT COUNT(*) FROM price_models;

-- name: UpdatePriceModel :one
UPDATE price_models
SET name = COALESCE(sqlc.narg('name'), name),
    per_minute_rate = COALESCE(sqlc.narg('per_minute_rate'), per_minute_rate),
    unlock_fee = COALESCE(sqlc.narg('unlock_fee'), unlock_fee),
    daily_cap = CASE WHEN sqlc.arg('daily_cap_set')::boolean THEN sqlc.narg('daily_cap') ELSE daily_cap END,
    currency = COALESCE(sqlc.narg('currency'), currency),
    updated_at = NOW()
WHERE id = sqlc.arg('id')
RETURNING id, name, per_minute_rate, unlock_fee, daily_cap, currency, created_at, updated_at;

-- name: DeletePriceModel :execrows
DELETE FROM price_models WHERE id = $1;
