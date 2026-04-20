-- name: CreateMaintenance :one
INSERT INTO maintenance (scooter_id, description, technician_id, status)
VALUES ($1, $2, $3, COALESCE(sqlc.narg('status'), 'open'))
RETURNING id, scooter_id, description, opened_at, closed_at, technician_id, status;

-- name: GetMaintenance :one
SELECT id, scooter_id, description, opened_at, closed_at, technician_id, status
FROM maintenance WHERE id = $1;

-- name: CloseMaintenance :one
UPDATE maintenance
SET status = 'closed',
    closed_at = sqlc.arg('closed_at')
WHERE id = sqlc.arg('id') AND status = 'open'
RETURNING id, scooter_id, description, opened_at, closed_at, technician_id, status;

-- name: ListMaintenanceByScooter :many
SELECT id, scooter_id, description, opened_at, closed_at, technician_id, status
FROM maintenance
WHERE scooter_id = $1
ORDER BY opened_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMaintenanceByScooter :one
SELECT COUNT(*) FROM maintenance WHERE scooter_id = $1;
