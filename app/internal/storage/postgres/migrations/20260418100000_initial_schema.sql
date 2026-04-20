-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS cube;
CREATE EXTENSION IF NOT EXISTS earthdistance;

CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email          VARCHAR(255) NOT NULL,
    name           VARCHAR(100) NOT NULL,
    phone          VARCHAR(20),
    password_hash  TEXT,
    oauth_id       VARCHAR(255),
    role           VARCHAR(20) NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    wallet_balance NUMERIC(10, 2) NOT NULL DEFAULT 0 CHECK (wallet_balance >= 0),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

CREATE UNIQUE INDEX users_email_unique
    ON users (LOWER(email))
    WHERE deleted_at IS NULL;

CREATE TABLE zones (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    boundary   TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE price_models (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    per_minute_rate NUMERIC(10, 4) NOT NULL CHECK (per_minute_rate >= 0),
    unlock_fee      NUMERIC(10, 2) NOT NULL DEFAULT 0 CHECK (unlock_fee >= 0),
    daily_cap       NUMERIC(10, 2) CHECK (daily_cap IS NULL OR daily_cap >= 0),
    currency        VARCHAR(3) NOT NULL DEFAULT 'PLN',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE scooters (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(20) NOT NULL UNIQUE,
    model       VARCHAR(100) NOT NULL,
    battery_pct INT NOT NULL DEFAULT 100 CHECK (battery_pct BETWEEN 0 AND 100),
    status      VARCHAR(20) NOT NULL DEFAULT 'available'
                CHECK (status IN ('available', 'rented', 'maintenance')),
    zone_id     UUID REFERENCES zones(id) ON DELETE SET NULL,
    lat         NUMERIC(9, 6),
    lng         NUMERIC(9, 6),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE rentals (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id),
    scooter_id     UUID NOT NULL REFERENCES scooters(id),
    price_model_id UUID NOT NULL REFERENCES price_models(id),
    started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at       TIMESTAMPTZ,
    distance_m     INT NOT NULL DEFAULT 0,
    total_cost     NUMERIC(10, 2) NOT NULL DEFAULT 0,
    status         VARCHAR(20) NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'completed', 'cancelled')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX rentals_one_active_per_scooter
    ON rentals (scooter_id)
    WHERE status = 'active';

CREATE UNIQUE INDEX rentals_one_active_per_user
    ON rentals (user_id)
    WHERE status = 'active';

CREATE TABLE maintenance (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scooter_id    UUID NOT NULL REFERENCES scooters(id),
    description   TEXT NOT NULL,
    opened_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at     TIMESTAMPTZ,
    technician_id UUID REFERENCES users(id),
    status        VARCHAR(20) NOT NULL DEFAULT 'open'
                  CHECK (status IN ('open', 'closed'))
);

CREATE INDEX idx_scooters_status
    ON scooters (status)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_scooters_zone
    ON scooters (zone_id);

CREATE INDEX idx_scooters_ll
    ON scooters USING gist (ll_to_earth(lat::float8, lng::float8))
    WHERE lat IS NOT NULL AND lng IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX idx_rentals_user
    ON rentals (user_id, started_at DESC);

CREATE INDEX idx_rentals_scooter
    ON rentals (scooter_id, started_at DESC);

CREATE INDEX idx_maintenance_scooter
    ON maintenance (scooter_id, opened_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS maintenance;
DROP TABLE IF EXISTS rentals;
DROP TABLE IF EXISTS scooters;
DROP TABLE IF EXISTS price_models;
DROP TABLE IF EXISTS zones;
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS earthdistance;
DROP EXTENSION IF EXISTS cube;
DROP EXTENSION IF EXISTS pgcrypto;
DROP EXTENSION IF EXISTS "uuid-ossp";
-- +goose StatementEnd
