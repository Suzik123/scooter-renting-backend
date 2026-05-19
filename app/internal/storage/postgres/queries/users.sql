-- name: CreateUser :one
INSERT INTO users (first_name, last_name, email, phone_number, password_hash, oauth_provider, oauth_subject, role, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE(sqlc.narg('status'), 'active'))
RETURNING user_id, first_name, last_name, email, phone_number, registration_date, status, role, password_hash, oauth_provider, oauth_subject, stripe_customer_id, updated_at, last_logout_at;

-- name: GetUserByID :one
SELECT user_id, first_name, last_name, email, phone_number, registration_date, status, role, password_hash, oauth_provider, oauth_subject, stripe_customer_id, updated_at, last_logout_at
FROM users
WHERE user_id = $1 AND status <> 'deleted';

-- name: GetUserByEmail :one
SELECT user_id, first_name, last_name, email, phone_number, registration_date, status, role, password_hash, oauth_provider, oauth_subject, stripe_customer_id, updated_at, last_logout_at
FROM users
WHERE LOWER(email) = LOWER($1) AND status <> 'deleted';

-- name: GetUserByOAuth :one
SELECT user_id, first_name, last_name, email, phone_number, registration_date, status, role, password_hash, oauth_provider, oauth_subject, stripe_customer_id, updated_at, last_logout_at
FROM users
WHERE oauth_provider = $1 AND oauth_subject = $2 AND status <> 'deleted';

-- name: UpdateUser :one
UPDATE users
SET first_name   = COALESCE(sqlc.narg('first_name'), first_name),
    last_name    = COALESCE(sqlc.narg('last_name'), last_name),
    phone_number = COALESCE(sqlc.narg('phone_number'), phone_number),
    updated_at   = NOW()
WHERE user_id = sqlc.arg('user_id') AND status <> 'deleted'
RETURNING user_id, first_name, last_name, email, phone_number, registration_date, status, role, password_hash, oauth_provider, oauth_subject, stripe_customer_id, updated_at, last_logout_at;

-- name: SoftDeleteUser :execrows
UPDATE users
SET status = 'deleted',
    updated_at = NOW()
WHERE user_id = $1 AND status <> 'deleted';

-- name: SetUserRole :execrows
UPDATE users
SET role = sqlc.arg('role'),
    updated_at = NOW()
WHERE user_id = sqlc.arg('user_id') AND status <> 'deleted';

-- name: LinkUserOAuth :one
UPDATE users
SET oauth_provider = sqlc.arg('oauth_provider'),
    oauth_subject  = sqlc.arg('oauth_subject'),
    updated_at     = NOW()
WHERE user_id = sqlc.arg('user_id') AND status <> 'deleted'
RETURNING user_id, first_name, last_name, email, phone_number, registration_date, status, role, password_hash, oauth_provider, oauth_subject, stripe_customer_id, updated_at, last_logout_at;

-- name: SetStripeCustomerID :one
UPDATE users
SET stripe_customer_id = sqlc.arg('stripe_customer_id'),
    updated_at = NOW()
WHERE user_id = sqlc.arg('user_id') AND status <> 'deleted'
RETURNING user_id, first_name, last_name, email, phone_number, registration_date, status, role, password_hash, oauth_provider, oauth_subject, stripe_customer_id, updated_at, last_logout_at;

-- name: ResetUserPassword :execrows
UPDATE users
SET password_hash   = sqlc.arg('password_hash'),
    last_logout_at  = NOW(),
    updated_at      = NOW()
WHERE user_id = sqlc.arg('user_id') AND status <> 'deleted';
