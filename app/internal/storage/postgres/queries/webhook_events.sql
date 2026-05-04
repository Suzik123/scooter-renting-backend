-- name: InsertWebhookEvent :one
INSERT INTO webhook_events (event_id, type, payload)
VALUES ($1, $2, $3)
ON CONFLICT (event_id) DO NOTHING
RETURNING event_id;
