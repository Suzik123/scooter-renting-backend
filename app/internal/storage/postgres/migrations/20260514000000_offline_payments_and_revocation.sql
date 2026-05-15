-- +goose Up
-- +goose StatementBegin
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_payment_method_check;
ALTER TABLE payments ADD CONSTRAINT payments_payment_method_check
    CHECK (payment_method IN ('card','apple_pay','google_pay','offline','transfer'));
ALTER TABLE payments ADD COLUMN IF NOT EXISTS offline_approved_by uuid REFERENCES users(user_id);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS offline_approved_at timestamptz;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS idempotency_key text;
CREATE UNIQUE INDEX IF NOT EXISTS payments_idem_uq
    ON payments(idempotency_key) WHERE idempotency_key IS NOT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_logout_at timestamptz;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS last_logout_at;
DROP INDEX IF EXISTS payments_idem_uq;
ALTER TABLE payments DROP COLUMN IF EXISTS idempotency_key;
ALTER TABLE payments DROP COLUMN IF EXISTS offline_approved_at;
ALTER TABLE payments DROP COLUMN IF EXISTS offline_approved_by;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_payment_method_check;
ALTER TABLE payments ADD CONSTRAINT payments_payment_method_check
    CHECK (payment_method IN ('card','apple_pay','google_pay'));
-- +goose StatementEnd
