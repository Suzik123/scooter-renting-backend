-- name: CreateRental :one
INSERT INTO rentals (user_id, scooter_id, price_model_id, start_time, start_lat, start_lon, status)
VALUES (
    $1,
    $2,
    $3,
    COALESCE(sqlc.narg('start_time'), NOW()),
    sqlc.narg('start_lat'),
    sqlc.narg('start_lon'),
    COALESCE(sqlc.narg('status'), 'active')
)
RETURNING rental_id, user_id, scooter_id, price_model_id, start_time, end_time, start_lat, start_lon, end_lat, end_lon, total_cost, status, distance_m, created_at, updated_at;

-- name: GetRental :one
SELECT rental_id, user_id, scooter_id, price_model_id, start_time, end_time, start_lat, start_lon, end_lat, end_lon, total_cost, status, distance_m, created_at, updated_at
FROM rentals
WHERE rental_id = $1;

-- name: GetRentalForUpdate :one
SELECT rental_id, user_id, scooter_id, price_model_id, start_time, end_time, start_lat, start_lon, end_lat, end_lon, total_cost, status, distance_m, created_at, updated_at
FROM rentals
WHERE rental_id = $1
FOR UPDATE;

-- name: EndRental :one
UPDATE rentals
SET status     = 'completed',
    end_time   = sqlc.arg('end_time'),
    end_lat    = sqlc.narg('end_lat'),
    end_lon    = sqlc.narg('end_lon'),
    distance_m = sqlc.arg('distance_m'),
    total_cost = sqlc.arg('total_cost'),
    updated_at = NOW()
WHERE rental_id = sqlc.arg('rental_id') AND status = 'active'
RETURNING rental_id, user_id, scooter_id, price_model_id, start_time, end_time, start_lat, start_lon, end_lat, end_lon, total_cost, status, distance_m, created_at, updated_at;

-- name: CancelRental :execrows
UPDATE rentals
SET status     = 'cancelled',
    updated_at = NOW()
WHERE rental_id = $1 AND status = 'active';

-- name: ListRentalsByUser :many
SELECT rental_id, user_id, scooter_id, price_model_id, start_time, end_time, start_lat, start_lon, end_lat, end_lon, total_cost, status, distance_m, created_at, updated_at
FROM rentals
WHERE user_id = $1
ORDER BY start_time DESC
LIMIT $2 OFFSET $3;

-- name: CountRentalsByUser :one
SELECT COUNT(*) FROM rentals WHERE user_id = $1;
