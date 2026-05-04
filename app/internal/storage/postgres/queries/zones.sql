-- name: CreateZone :one
INSERT INTO zones (name, center_lat, center_lon, radius_meters, zone_type)
VALUES ($1, $2, $3, $4, COALESCE(sqlc.narg('zone_type'), 'service'))
RETURNING zone_id, name, center_lat, center_lon, radius_meters, zone_type, created_at, updated_at;

-- name: GetZone :one
SELECT zone_id, name, center_lat, center_lon, radius_meters, zone_type, created_at, updated_at
FROM zones
WHERE zone_id = $1;

-- name: ListZones :many
SELECT zone_id, name, center_lat, center_lon, radius_meters, zone_type, created_at, updated_at
FROM zones
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountZones :one
SELECT COUNT(*) FROM zones;

-- name: UpdateZone :one
UPDATE zones
SET name          = COALESCE(sqlc.narg('name'), name),
    center_lat    = COALESCE(sqlc.narg('center_lat'), center_lat),
    center_lon    = COALESCE(sqlc.narg('center_lon'), center_lon),
    radius_meters = COALESCE(sqlc.narg('radius_meters'), radius_meters),
    zone_type     = COALESCE(sqlc.narg('zone_type'), zone_type),
    updated_at    = NOW()
WHERE zone_id = sqlc.arg('zone_id')
RETURNING zone_id, name, center_lat, center_lon, radius_meters, zone_type, created_at, updated_at;

-- name: DeleteZone :execrows
DELETE FROM zones WHERE zone_id = $1;
