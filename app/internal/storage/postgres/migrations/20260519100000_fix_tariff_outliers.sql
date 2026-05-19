-- +goose Up
-- +goose StatementBegin
-- One-shot guard: any price_model with a per-minute rate outside the
-- recommended band 0.05-2.00 was almost certainly seeded by mistake
-- (e.g. confusing dollars with a per-minute rate). Snap them back to a
-- sane default of 0.20. The service-layer band check enforces the same
-- guard going forward, with `?force=true` for intentional overrides.
UPDATE price_models
SET price_per_minute = 0.20, updated_at = NOW()
WHERE price_per_minute > 2.00 OR price_per_minute < 0.05;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- no-op: we cannot recover the original bad values.
SELECT 1;
-- +goose StatementEnd
