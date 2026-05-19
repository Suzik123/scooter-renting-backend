-- +goose Up
-- +goose StatementBegin
CREATE TABLE password_reset_tokens (
    token_id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX password_reset_tokens_user_idx ON password_reset_tokens(user_id);
CREATE UNIQUE INDEX password_reset_tokens_hash_uq ON password_reset_tokens(token_hash);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS password_reset_tokens;
-- +goose StatementEnd
