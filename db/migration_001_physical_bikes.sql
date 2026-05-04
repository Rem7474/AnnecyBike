-- Run this on existing deployments to add physical bike tracking
CREATE TABLE IF NOT EXISTS physical_bikes (
    id              BIGSERIAL PRIMARY KEY,
    vehicle_type_id TEXT REFERENCES vehicle_types(vehicle_type_id),
    fleet_number    TEXT,
    custom_name     TEXT,
    first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- If physical_bikes already exists (re-running migration), add columns idempotently
ALTER TABLE physical_bikes ADD COLUMN IF NOT EXISTS fleet_number TEXT;
ALTER TABLE physical_bikes ADD COLUMN IF NOT EXISTS custom_name  TEXT;

ALTER TABLE bikes ADD COLUMN IF NOT EXISTS physical_bike_id BIGINT REFERENCES physical_bikes(id);
ALTER TABLE trips ADD COLUMN IF NOT EXISTS physical_bike_id BIGINT REFERENCES physical_bikes(id);

CREATE INDEX IF NOT EXISTS idx_bikes_physical_bike_id ON bikes (physical_bike_id) WHERE physical_bike_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_trips_physical_bike_id ON trips (physical_bike_id) WHERE physical_bike_id IS NOT NULL;
