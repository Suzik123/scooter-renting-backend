-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING token_id, user_id, token_hash, expires_at, used_at, created_at;

-- name: GetPasswordResetTokenByHash :one
SELECT token_id, user_id, token_hash, expires_at, used_at, created_at
FROM password_reset_tokens
WHERE token_hash = $1;

-- name: InvalidatePasswordResetTokensForUser :execrows
UPDATE password_reset_tokens
SET used_at = NOW()
WHERE user_id = $1 AND used_at IS NULL;

-- name: MarkPasswordResetTokenUsed :execrows
UPDATE password_reset_tokens
SET used_at = NOW()
WHERE token_id = $1 AND used_at IS NULL;
