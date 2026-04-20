-- name: CreateZone :one
INSERT INTO zones (name, boundary)
VALUES ($1, $2)
RETURNING id, name, boundary, created_at, updated_at;

-- name: GetZone :one
SELECT id, name, boundary, created_at, updated_at
FROM zones
WHERE id = $1;

-- name: ListZones :many
SELECT id, name, boundary, created_at, updated_at
FROM zones
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountZones :one
SELECT COUNT(*) FROM zones;

-- name: UpdateZone :one
UPDATE zones
SET name = COALESCE(sqlc.narg('name'), name),
    boundary = COALESCE(sqlc.narg('boundary'), boundary),
    updated_at = NOW()
WHERE id = sqlc.arg('id')
RETURNING id, name, boundary, created_at, updated_at;

-- name: DeleteZone :execrows
DELETE FROM zones WHERE id = $1;
