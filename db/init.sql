-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- ============================================================
-- Static tables
-- ============================================================

CREATE TABLE IF NOT EXISTS vehicle_types (
    vehicle_type_id TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    form_factor     TEXT NOT NULL DEFAULT 'bicycle',
    propulsion_type TEXT NOT NULL,
    max_range_meters INT
);

CREATE TABLE IF NOT EXISTS stations (
    station_id              TEXT PRIMARY KEY,
    name                    TEXT NOT NULL,
    lat                     DOUBLE PRECISION NOT NULL,
    lon                     DOUBLE PRECISION NOT NULL,
    capacity                INT,
    vehicle_type_capacity   JSONB,
    is_virtual_station      BOOLEAN DEFAULT FALSE,
    is_charging_station     BOOLEAN DEFAULT FALSE,
    last_updated            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bikes (
    bike_id         TEXT PRIMARY KEY,
    vehicle_type_id TEXT REFERENCES vehicle_types(vehicle_type_id),
    first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- Time-series hypertables
-- ============================================================

CREATE TABLE IF NOT EXISTS bike_snapshots (
    time                 TIMESTAMPTZ      NOT NULL,
    bike_id              TEXT             NOT NULL,
    lat                  DOUBLE PRECISION,
    lon                  DOUBLE PRECISION,
    station_id           TEXT,
    is_reserved          BOOLEAN          DEFAULT FALSE,
    is_disabled          BOOLEAN          DEFAULT FALSE,
    current_range_meters INT
);

SELECT create_hypertable('bike_snapshots', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

CREATE INDEX IF NOT EXISTS idx_bike_snapshots_bike_time
    ON bike_snapshots (bike_id, time DESC);

CREATE TABLE IF NOT EXISTS station_snapshots (
    time                    TIMESTAMPTZ NOT NULL,
    station_id              TEXT        NOT NULL,
    num_bikes_available     INT         DEFAULT 0,
    num_docks_available     INT         DEFAULT 0,
    is_installed            BOOLEAN     DEFAULT TRUE,
    is_renting              BOOLEAN     DEFAULT TRUE,
    is_returning            BOOLEAN     DEFAULT TRUE,
    vehicle_docks_available JSONB
);

SELECT create_hypertable('station_snapshots', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

CREATE INDEX IF NOT EXISTS idx_station_snapshots_station_time
    ON station_snapshots (station_id, time DESC);

-- ============================================================
-- Trips (derived by the poller)
-- ============================================================

CREATE TABLE IF NOT EXISTS trips (
    id               BIGSERIAL PRIMARY KEY,
    bike_id          TEXT             NOT NULL,
    start_time       TIMESTAMPTZ      NOT NULL,
    end_time         TIMESTAMPTZ,
    start_station_id TEXT,
    end_station_id   TEXT,
    start_lat        DOUBLE PRECISION,
    start_lon        DOUBLE PRECISION,
    end_lat          DOUBLE PRECISION,
    end_lon          DOUBLE PRECISION,
    distance_meters  INT,
    battery_start    INT,
    battery_end      INT,
    battery_delta    INT
);

CREATE INDEX IF NOT EXISTS idx_trips_bike_start ON trips (bike_id, start_time DESC);
CREATE INDEX IF NOT EXISTS idx_trips_start_station ON trips (start_station_id) WHERE start_station_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_trips_end_station ON trips (end_station_id) WHERE end_station_id IS NOT NULL;

-- ============================================================
-- Continuous aggregate: daily bike stats
-- ============================================================

CREATE MATERIALIZED VIEW IF NOT EXISTS bike_daily_stats
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', time) AS bucket,
    bike_id,
    AVG(current_range_meters)::INT  AS avg_battery,
    MIN(current_range_meters)       AS min_battery,
    MAX(current_range_meters)       AS max_battery,
    COUNT(*)                        AS snapshot_count,
    COUNT(*) FILTER (WHERE is_disabled) AS disabled_count,
    COUNT(*) FILTER (WHERE is_reserved) AS reserved_count
FROM bike_snapshots
GROUP BY 1, 2
WITH NO DATA;

SELECT add_continuous_aggregate_policy('bike_daily_stats',
    start_offset => INTERVAL '3 days',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE
);

-- ============================================================
-- Compression policy (after 7 days)
-- ============================================================

ALTER TABLE bike_snapshots SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'time DESC',
    timescaledb.compress_segmentby = 'bike_id'
);

SELECT add_compression_policy('bike_snapshots',
    INTERVAL '7 days',
    if_not_exists => TRUE
);

ALTER TABLE station_snapshots SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'time DESC',
    timescaledb.compress_segmentby = 'station_id'
);

SELECT add_compression_policy('station_snapshots',
    INTERVAL '7 days',
    if_not_exists => TRUE
);
