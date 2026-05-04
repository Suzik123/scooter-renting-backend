-- name: CreateScooter :one
INSERT INTO scooters (qr_code, battery_level, status, zone_id, model, lat, lng)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING scooter_id, qr_code, battery_level, status, zone_id, model, lat, lng, created_at, updated_at, deleted_at;

-- name: GetScooter :one
SELECT scooter_id, qr_code, battery_level, status, zone_id, model, lat, lng, created_at, updated_at, deleted_at
FROM scooters
WHERE scooter_id = $1 AND deleted_at IS NULL;

-- name: ListScooters :many
SELECT scooter_id, qr_code, battery_level, status, zone_id, model, lat, lng, created_at, updated_at, deleted_at
FROM scooters
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('zone_id')::uuid IS NULL OR zone_id = sqlc.narg('zone_id'))
ORDER BY created_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountScooters :one
SELECT COUNT(*) FROM scooters
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('zone_id')::uuid IS NULL OR zone_id = sqlc.narg('zone_id'));

-- name: UpdateScooter :one
UPDATE scooters
SET model         = COALESCE(sqlc.narg('model'), model),
    status        = COALESCE(sqlc.narg('status'), status),
    zone_id       = CASE WHEN sqlc.arg('zone_id_set')::boolean THEN sqlc.narg('zone_id') ELSE zone_id END,
    battery_level = COALESCE(sqlc.narg('battery_level'), battery_level),
    lat           = CASE WHEN sqlc.arg('lat_set')::boolean THEN sqlc.narg('lat') ELSE lat END,
    lng           = CASE WHEN sqlc.arg('lng_set')::boolean THEN sqlc.narg('lng') ELSE lng END,
    updated_at    = NOW()
WHERE scooter_id = sqlc.arg('scooter_id') AND deleted_at IS NULL
RETURNING scooter_id, qr_code, battery_level, status, zone_id, model, lat, lng, created_at, updated_at, deleted_at;

-- name: RetireScooter :execrows
UPDATE scooters
SET deleted_at = NOW(),
    status     = 'retired',
    updated_at = NOW()
WHERE scooter_id = $1 AND deleted_at IS NULL;

-- name: FindNearbyScooters :many
SELECT scooter_id, qr_code, battery_level, status, zone_id, model, lat, lng, created_at, updated_at, deleted_at
FROM scooters
WHERE status = 'available'
  AND deleted_at IS NULL
  AND lat IS NOT NULL
  AND lng IS NOT NULL
  AND (point(lng::float8, lat::float8) <@> point(sqlc.arg('lng_p')::float8, sqlc.arg('lat_p')::float8)) * 1609.344 <= sqlc.arg('radius_m')::float8
ORDER BY (point(lng::float8, lat::float8) <@> point(sqlc.arg('lng_p')::float8, sqlc.arg('lat_p')::float8)) * 1609.344 ASC
LIMIT sqlc.arg('lim');

-- name: SetScooterStatus :one
UPDATE scooters
SET status = sqlc.arg('to_status'),
    updated_at = NOW()
WHERE scooter_id = sqlc.arg('scooter_id')
  AND status = sqlc.arg('from_status')
  AND deleted_at IS NULL
RETURNING scooter_id;
