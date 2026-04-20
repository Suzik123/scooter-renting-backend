-- name: CreateUser :one
INSERT INTO users (email, name, phone, password_hash, oauth_id, role, wallet_balance)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, email, name, phone, password_hash, oauth_id, role, wallet_balance, created_at, updated_at, deleted_at;

-- name: GetUserByID :one
SELECT id, email, name, phone, password_hash, oauth_id, role, wallet_balance, created_at, updated_at, deleted_at
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT id, email, name, phone, password_hash, oauth_id, role, wallet_balance, created_at, updated_at, deleted_at
FROM users
WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE(sqlc.narg('name'), name),
    phone = COALESCE(sqlc.narg('phone'), phone),
    updated_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, email, name, phone, password_hash, oauth_id, role, wallet_balance, created_at, updated_at, deleted_at;

-- name: SoftDeleteUser :execrows
UPDATE users
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: AdjustWallet :one
UPDATE users
SET wallet_balance = wallet_balance + sqlc.arg('delta'),
    updated_at = NOW()
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL
  AND wallet_balance + sqlc.arg('delta') >= 0
RETURNING wallet_balance;

-- name: GetWallet :one
SELECT wallet_balance
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: SetUserRole :execrows
UPDATE users
SET role = sqlc.arg('role'),
    updated_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;
