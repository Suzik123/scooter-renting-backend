-- name: CreateMaintenance :one
INSERT INTO maintenance (scooter_id, technician_name, issue_description, repair_cost, status)
VALUES ($1, $2, $3, $4, COALESCE(sqlc.narg('status'), 'open'))
RETURNING maintenance_id, scooter_id, technician_name, issue_description, repair_cost, start_date, end_date, status;

-- name: GetMaintenance :one
SELECT maintenance_id, scooter_id, technician_name, issue_description, repair_cost, start_date, end_date, status
FROM maintenance WHERE maintenance_id = $1;

-- name: CloseMaintenance :one
UPDATE maintenance
SET status   = 'closed',
    end_date = sqlc.arg('end_date')
WHERE maintenance_id = sqlc.arg('maintenance_id') AND status = 'open'
RETURNING maintenance_id, scooter_id, technician_name, issue_description, repair_cost, start_date, end_date, status;

-- name: ListMaintenanceByScooter :many
SELECT maintenance_id, scooter_id, technician_name, issue_description, repair_cost, start_date, end_date, status
FROM maintenance
WHERE scooter_id = $1
ORDER BY start_date DESC
LIMIT $2 OFFSET $3;

-- name: CountMaintenanceByScooter :one
SELECT COUNT(*) FROM maintenance WHERE scooter_id = $1;
