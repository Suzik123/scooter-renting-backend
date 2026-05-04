-- name: CreatePriceModel :one
INSERT INTO price_models (name, unlock_fee, price_per_minute, currency, daily_cap)
VALUES ($1, $2, $3, $4, $5)
RETURNING price_model_id, name, unlock_fee, price_per_minute, currency, daily_cap, created_at, updated_at;

-- name: GetPriceModel :one
SELECT price_model_id, name, unlock_fee, price_per_minute, currency, daily_cap, created_at, updated_at
FROM price_models
WHERE price_model_id = $1;

-- name: ListPriceModels :many
SELECT price_model_id, name, unlock_fee, price_per_minute, currency, daily_cap, created_at, updated_at
FROM price_models
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPriceModels :one
SELECT COUNT(*) FROM price_models;

-- name: UpdatePriceModel :one
UPDATE price_models
SET name             = COALESCE(sqlc.narg('name'), name),
    price_per_minute = COALESCE(sqlc.narg('price_per_minute'), price_per_minute),
    unlock_fee       = COALESCE(sqlc.narg('unlock_fee'), unlock_fee),
    daily_cap        = CASE WHEN sqlc.arg('daily_cap_set')::boolean THEN sqlc.narg('daily_cap') ELSE daily_cap END,
    currency         = COALESCE(sqlc.narg('currency'), currency),
    updated_at       = NOW()
WHERE price_model_id = sqlc.arg('price_model_id')
RETURNING price_model_id, name, unlock_fee, price_per_minute, currency, daily_cap, created_at, updated_at;

-- name: DeletePriceModel :execrows
DELETE FROM price_models WHERE price_model_id = $1;
