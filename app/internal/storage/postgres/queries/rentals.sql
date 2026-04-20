-- name: CreateRental :one
INSERT INTO rentals (user_id, scooter_id, price_model_id, started_at, status)
VALUES ($1, $2, $3, COALESCE(sqlc.narg('started_at'), NOW()), COALESCE(sqlc.narg('status'), 'active'))
RETURNING id, user_id, scooter_id, price_model_id, started_at, ended_at, distance_m, total_cost, status, created_at, updated_at;

-- name: GetRental :one
SELECT id, user_id, scooter_id, price_model_id, started_at, ended_at, distance_m, total_cost, status, created_at, updated_at
FROM rentals
WHERE id = $1;

-- name: GetRentalForUpdate :one
SELECT id, user_id, scooter_id, price_model_id, started_at, ended_at, distance_m, total_cost, status, created_at, updated_at
FROM rentals
WHERE id = $1
FOR UPDATE;

-- name: EndRental :one
UPDATE rentals
SET status = 'completed',
    ended_at = sqlc.arg('ended_at'),
    distance_m = sqlc.arg('distance_m'),
    total_cost = sqlc.arg('total_cost'),
    updated_at = NOW()
WHERE id = sqlc.arg('id') AND status = 'active'
RETURNING id, user_id, scooter_id, price_model_id, started_at, ended_at, distance_m, total_cost, status, created_at, updated_at;

-- name: CancelRental :execrows
UPDATE rentals
SET status = 'cancelled',
    updated_at = NOW()
WHERE id = $1 AND status = 'active';

-- name: ListRentalsByUser :many
SELECT id, user_id, scooter_id, price_model_id, started_at, ended_at, distance_m, total_cost, status, created_at, updated_at
FROM rentals
WHERE user_id = $1
ORDER BY started_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRentalsByUser :one
SELECT COUNT(*) FROM rentals WHERE user_id = $1;
