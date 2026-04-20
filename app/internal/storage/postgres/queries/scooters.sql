-- name: CreateScooter :one
INSERT INTO scooters (code, model, battery_pct, status, zone_id, lat, lng)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, code, model, battery_pct, status, zone_id, lat, lng, created_at, updated_at, deleted_at;

-- name: GetScooter :one
SELECT id, code, model, battery_pct, status, zone_id, lat, lng, created_at, updated_at, deleted_at
FROM scooters
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListScooters :many
SELECT id, code, model, battery_pct, status, zone_id, lat, lng, created_at, updated_at, deleted_at
FROM scooters
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('zone_id')::uuid IS NULL OR zone_id = sqlc.narg('zone_id'))
ORDER BY created_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountScooters :one
SELECT COUNT(*) FROM scooters
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('zone_id')::uuid IS NULL OR zone_id = sqlc.narg('zone_id'));

-- name: UpdateScooter :one
UPDATE scooters
SET model = COALESCE(sqlc.narg('model'), model),
    status = COALESCE(sqlc.narg('status'), status),
    zone_id = CASE WHEN sqlc.arg('zone_id_set')::boolean THEN sqlc.narg('zone_id') ELSE zone_id END,
    battery_pct = COALESCE(sqlc.narg('battery_pct'), battery_pct),
    lat = CASE WHEN sqlc.arg('lat_set')::boolean THEN sqlc.narg('lat') ELSE lat END,
    lng = CASE WHEN sqlc.arg('lng_set')::boolean THEN sqlc.narg('lng') ELSE lng END,
    updated_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, code, model, battery_pct, status, zone_id, lat, lng, created_at, updated_at, deleted_at;

-- name: RetireScooter :execrows
UPDATE scooters
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: FindNearbyScooters :many
SELECT id, code, model, battery_pct, status, zone_id, lat, lng, created_at, updated_at, deleted_at
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
WHERE id = sqlc.arg('id')
  AND status = sqlc.arg('from_status')
  AND deleted_at IS NULL
RETURNING id;
