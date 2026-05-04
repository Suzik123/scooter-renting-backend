-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS cube;
CREATE EXTENSION IF NOT EXISTS earthdistance;

-- Drop legacy tables and indexes from the initial schema (FK order).
DROP TABLE IF EXISTS maintenance CASCADE;
DROP TABLE IF EXISTS rentals CASCADE;
DROP TABLE IF EXISTS scooters CASCADE;
DROP TABLE IF EXISTS price_models CASCADE;
DROP TABLE IF EXISTS zones CASCADE;
DROP TABLE IF EXISTS users CASCADE;

CREATE TABLE users (
    user_id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    first_name           text NOT NULL,
    last_name            text NOT NULL DEFAULT '',
    email                text NOT NULL,
    phone_number         text,
    registration_date    timestamptz NOT NULL DEFAULT NOW(),
    status               text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deleted')),
    role                 text NOT NULL DEFAULT 'user' CHECK (role IN ('user','admin')),
    password_hash        text,
    oauth_provider       text,
    oauth_subject        text,
    stripe_customer_id   text,
    updated_at           timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX users_email_lower_uq ON users (LOWER(email)) WHERE status <> 'deleted';
CREATE UNIQUE INDEX users_oauth_uq ON users (oauth_provider, oauth_subject)
    WHERE oauth_provider IS NOT NULL AND oauth_subject IS NOT NULL AND status <> 'deleted';

CREATE TABLE zones (
    zone_id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name           text NOT NULL,
    center_lat     numeric(9,6) NOT NULL,
    center_lon     numeric(9,6) NOT NULL,
    radius_meters  integer NOT NULL CHECK (radius_meters > 0),
    zone_type      text NOT NULL DEFAULT 'service' CHECK (zone_type IN ('service','no_park','reduced_speed')),
    created_at     timestamptz NOT NULL DEFAULT NOW(),
    updated_at     timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE scooters (
    scooter_id     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    qr_code        text NOT NULL UNIQUE,
    battery_level  integer NOT NULL DEFAULT 100 CHECK (battery_level BETWEEN 0 AND 100),
    status         text NOT NULL DEFAULT 'available' CHECK (status IN ('available','rented','maintenance','retired')),
    zone_id        uuid REFERENCES zones(zone_id) ON DELETE SET NULL,
    model          text NOT NULL DEFAULT '',
    lat            numeric(9,6),
    lng            numeric(9,6),
    created_at     timestamptz NOT NULL DEFAULT NOW(),
    updated_at     timestamptz NOT NULL DEFAULT NOW(),
    deleted_at     timestamptz
);
CREATE INDEX idx_scooters_status ON scooters (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_scooters_ll ON scooters USING gist (ll_to_earth(lat::float8, lng::float8))
  WHERE lat IS NOT NULL AND lng IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE price_models (
    price_model_id     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name               text NOT NULL,
    unlock_fee         numeric(10,2) NOT NULL DEFAULT 0,
    price_per_minute   numeric(10,4) NOT NULL,
    currency           text NOT NULL DEFAULT 'USD',
    daily_cap          numeric(10,2),
    created_at         timestamptz NOT NULL DEFAULT NOW(),
    updated_at         timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE rentals (
    rental_id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL REFERENCES users(user_id),
    scooter_id       uuid NOT NULL REFERENCES scooters(scooter_id),
    price_model_id   uuid NOT NULL REFERENCES price_models(price_model_id),
    start_time       timestamptz NOT NULL DEFAULT NOW(),
    end_time         timestamptz,
    start_lat        numeric(9,6),
    start_lon        numeric(9,6),
    end_lat          numeric(9,6),
    end_lon          numeric(9,6),
    total_cost       numeric(10,2) NOT NULL DEFAULT 0,
    status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active','completed','cancelled')),
    distance_m       integer NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT NOW(),
    updated_at       timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX rentals_one_active_per_scooter ON rentals (scooter_id) WHERE status = 'active';
CREATE UNIQUE INDEX rentals_one_active_per_user    ON rentals (user_id)    WHERE status = 'active';
CREATE INDEX idx_rentals_user ON rentals (user_id, start_time DESC);

CREATE TABLE maintenance (
    maintenance_id     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scooter_id         uuid NOT NULL REFERENCES scooters(scooter_id),
    technician_name    text NOT NULL DEFAULT '',
    issue_description  text NOT NULL,
    repair_cost        numeric(10,2),
    start_date         timestamptz NOT NULL DEFAULT NOW(),
    end_date           timestamptz,
    status             text NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed'))
);
CREATE INDEX idx_maintenance_scooter ON maintenance (scooter_id, start_date DESC);

CREATE TABLE payments (
    payment_id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              uuid NOT NULL REFERENCES users(user_id),
    rental_id            uuid REFERENCES rentals(rental_id),
    amount               numeric(10,2) NOT NULL,
    currency             text NOT NULL DEFAULT 'USD',
    payment_method       text NOT NULL DEFAULT 'card' CHECK (payment_method IN ('card','apple_pay','google_pay')),
    status               text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','succeeded','failed','refunded')),
    provider_payment_id  text UNIQUE,
    failure_reason       text,
    transaction_date     timestamptz NOT NULL DEFAULT NOW(),
    updated_at           timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_payments_user   ON payments (user_id, transaction_date DESC);
CREATE INDEX idx_payments_rental ON payments (rental_id);

CREATE TABLE webhook_events (
    event_id     text PRIMARY KEY,
    type         text NOT NULL,
    received_at  timestamptz NOT NULL DEFAULT NOW(),
    payload      jsonb NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS maintenance;
DROP TABLE IF EXISTS rentals;
DROP TABLE IF EXISTS scooters;
DROP TABLE IF EXISTS price_models;
DROP TABLE IF EXISTS zones;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
