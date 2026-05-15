-- name: CreatePayment :one
INSERT INTO payments (user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason)
VALUES (
    $1,
    $2,
    $3,
    COALESCE(sqlc.narg('currency'), 'USD'),
    COALESCE(sqlc.narg('payment_method'), 'card'),
    COALESCE(sqlc.narg('status'), 'pending'),
    sqlc.narg('provider_payment_id'),
    sqlc.narg('failure_reason')
)
RETURNING payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key;

-- name: CreateOfflinePayment :one
INSERT INTO payments (
    user_id, rental_id, amount, currency, payment_method, status,
    offline_approved_by, offline_approved_at, idempotency_key
) VALUES (
    sqlc.arg('user_id'),
    sqlc.arg('rental_id'),
    sqlc.arg('amount'),
    sqlc.arg('currency'),
    'offline',
    'succeeded',
    sqlc.arg('offline_approved_by'),
    NOW(),
    sqlc.narg('idempotency_key')
)
ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key;

-- name: GetPaymentByIdempotencyKey :one
SELECT payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key
FROM payments
WHERE idempotency_key = $1;

-- name: GetPaymentByID :one
SELECT payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key
FROM payments
WHERE payment_id = $1;

-- name: GetPaymentByProviderID :one
SELECT payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key
FROM payments
WHERE provider_payment_id = $1;

-- name: AttachPaymentIntent :one
UPDATE payments
SET provider_payment_id = sqlc.arg('provider_payment_id'),
    status              = COALESCE(sqlc.narg('status'), status),
    failure_reason      = sqlc.narg('failure_reason'),
    updated_at          = NOW()
WHERE payment_id = sqlc.arg('payment_id')
RETURNING payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key;

-- name: MarkPaymentByIDFailed :one
UPDATE payments
SET status         = 'failed',
    failure_reason = sqlc.narg('failure_reason'),
    updated_at     = NOW()
WHERE payment_id = sqlc.arg('payment_id')
RETURNING payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key;

-- name: MarkPaymentSucceeded :one
UPDATE payments
SET status     = 'succeeded',
    updated_at = NOW()
WHERE provider_payment_id = $1
  AND status = 'pending'
RETURNING payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key;

-- name: MarkPaymentFailed :one
UPDATE payments
SET status         = 'failed',
    failure_reason = sqlc.narg('failure_reason'),
    updated_at     = NOW()
WHERE provider_payment_id = sqlc.arg('provider_payment_id')
  AND status = 'pending'
RETURNING payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key;

-- name: ListPaymentsByUser :many
SELECT payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key
FROM payments
WHERE user_id = $1
ORDER BY transaction_date DESC
LIMIT $2 OFFSET $3;

-- name: ListPaymentsByUserSince :many
SELECT payment_id, user_id, rental_id, amount, currency, payment_method, status, provider_payment_id, failure_reason, transaction_date, updated_at, offline_approved_by, offline_approved_at, idempotency_key
FROM payments
WHERE user_id = sqlc.arg('user_id')
  AND transaction_date >= sqlc.arg('since')
ORDER BY transaction_date DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountPaymentsByUserSince :one
SELECT COUNT(*) FROM payments
WHERE user_id = sqlc.arg('user_id')
  AND transaction_date >= sqlc.arg('since');

-- name: CountPaymentsByUser :one
SELECT COUNT(*) FROM payments WHERE user_id = $1;

-- name: HasUnpaidRentals :one
SELECT EXISTS (
    SELECT 1
    FROM rentals r
    LEFT JOIN LATERAL (
        SELECT status
        FROM payments p
        WHERE p.rental_id = r.rental_id
        ORDER BY p.transaction_date DESC
        LIMIT 1
    ) lp ON TRUE
    WHERE r.user_id = $1
      AND (
            lp.status IN ('pending','failed')
        OR (lp.status IS NULL AND r.status = 'completed' AND r.total_cost > 0)
      )
) AS has_unpaid;
