-- Schema for milestone 3 (persistence, geofences, audit). Not used by milestone 1 code yet.
-- Apply with: task migrate

-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE fleets (
    id          text PRIMARY KEY,
    name        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE drones (
    id          text PRIMARY KEY,
    fleet_id    text NOT NULL REFERENCES fleets(id),
    model       text,
    home        geography(Point, 4326),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE geofences (
    id          bigserial PRIMARY KEY,
    fleet_id    text REFERENCES fleets(id),       -- NULL = applies to every fleet
    name        text NOT NULL,
    kind        text NOT NULL CHECK (kind IN ('keep_in', 'keep_out')),
    area        geography(Polygon, 4326) NOT NULL,
    max_alt_m   double precision,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX geofences_area_idx ON geofences USING gist (area);

CREATE TABLE missions (
    id          text PRIMARY KEY,
    fleet_id    text NOT NULL REFERENCES fleets(id),
    kind        text NOT NULL,                     -- survey, formation, ...
    spec        jsonb NOT NULL,
    status      text NOT NULL,
    created_by  text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Append-only audit log of every command and its lifecycle.
CREATE TABLE command_log (
    command_id  text NOT NULL,
    drone_id    text NOT NULL,
    status      text NOT NULL,
    reason      text,
    actor       text,
    payload     bytea,
    at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX command_log_drone_idx ON command_log (drone_id, at DESC);

-- Downsampled telemetry history for replay.
CREATE TABLE telemetry (
    ts           timestamptz NOT NULL,
    drone_id     text NOT NULL,
    lat          double precision NOT NULL,
    lon          double precision NOT NULL,
    alt_m        real NOT NULL,
    heading_deg  real,
    speed_mps    real,
    battery_pct  real,
    state        smallint
);
SELECT create_hypertable('telemetry', 'ts', chunk_time_interval => interval '1 day');
CREATE INDEX telemetry_drone_ts_idx ON telemetry (drone_id, ts DESC);
SELECT add_retention_policy('telemetry', interval '14 days');

CREATE TABLE drone_events (
    ts        timestamptz NOT NULL,
    drone_id  text NOT NULL,
    severity  text NOT NULL,
    code      text NOT NULL,
    message   text
);
SELECT create_hypertable('drone_events', 'ts');

-- +goose Down
DROP TABLE drone_events;
DROP TABLE telemetry;
DROP TABLE command_log;
DROP TABLE missions;
DROP TABLE geofences;
DROP TABLE drones;
DROP TABLE fleets;
