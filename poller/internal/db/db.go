package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	*pgxpool.Pool
}

func Connect(ctx context.Context, dbURL string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}
	cfg.MinConns = 2
	cfg.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Pool{pool}, nil
}

// UpsertVehicleType inserts or updates a vehicle type.
func (p *Pool) UpsertVehicleType(ctx context.Context, id, name, form, propulsion string, maxRange int) error {
	_, err := p.Exec(ctx, `
		INSERT INTO vehicle_types (vehicle_type_id, name, form_factor, propulsion_type, max_range_meters)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (vehicle_type_id) DO UPDATE SET
			name = EXCLUDED.name,
			form_factor = EXCLUDED.form_factor,
			propulsion_type = EXCLUDED.propulsion_type,
			max_range_meters = EXCLUDED.max_range_meters
	`, id, name, form, propulsion, maxRange)
	return err
}

// UpsertStation inserts or updates a station.
func (p *Pool) UpsertStation(ctx context.Context, id, name string, lat, lon float64, capacity int, vtCap map[string]int, isVirtual, isCharging bool) error {
	capJSON, err := json.Marshal(vtCap)
	if err != nil {
		return err
	}
	_, err = p.Exec(ctx, `
		INSERT INTO stations (station_id, name, lat, lon, capacity, vehicle_type_capacity, is_virtual_station, is_charging_station, last_updated)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (station_id) DO UPDATE SET
			name = EXCLUDED.name,
			lat = EXCLUDED.lat,
			lon = EXCLUDED.lon,
			capacity = EXCLUDED.capacity,
			vehicle_type_capacity = EXCLUDED.vehicle_type_capacity,
			is_virtual_station = EXCLUDED.is_virtual_station,
			is_charging_station = EXCLUDED.is_charging_station,
			last_updated = NOW()
	`, id, name, lat, lon, capacity, capJSON, isVirtual, isCharging)
	return err
}

// UpsertBike registers a new bike or updates last_seen.
func (p *Pool) UpsertBike(ctx context.Context, bikeID, vehicleTypeID string) error {
	_, err := p.Exec(ctx, `
		INSERT INTO bikes (bike_id, vehicle_type_id, first_seen, last_seen)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (bike_id) DO UPDATE SET
			last_seen = NOW(),
			vehicle_type_id = EXCLUDED.vehicle_type_id
	`, bikeID, vehicleTypeID)
	return err
}

type BikeSnapshot struct {
	Time                time.Time
	BikeID              string
	Lat                 float64
	Lon                 float64
	StationID           *string
	IsReserved          bool
	IsDisabled          bool
	CurrentRangeMeters  int
}

// BulkInsertBikeSnapshots writes all bike snapshots in a single COPY.
func (p *Pool) BulkInsertBikeSnapshots(ctx context.Context, snapshots []BikeSnapshot) error {
	rows := make([][]any, len(snapshots))
	for i, s := range snapshots {
		rows[i] = []any{s.Time, s.BikeID, s.Lat, s.Lon, s.StationID, s.IsReserved, s.IsDisabled, s.CurrentRangeMeters}
	}
	_, err := p.CopyFrom(ctx,
		pgx.Identifier{"bike_snapshots"},
		[]string{"time", "bike_id", "lat", "lon", "station_id", "is_reserved", "is_disabled", "current_range_meters"},
		pgx.CopyFromRows(rows),
	)
	return err
}

type StationSnapshot struct {
	Time               time.Time
	StationID          string
	NumBikesAvailable  int
	NumDocksAvailable  int
	IsInstalled        bool
	IsRenting          bool
	IsReturning        bool
	VehicleDocksJSON   []byte
}

// BulkInsertStationSnapshots writes all station snapshots in a single COPY.
func (p *Pool) BulkInsertStationSnapshots(ctx context.Context, snapshots []StationSnapshot) error {
	rows := make([][]any, len(snapshots))
	for i, s := range snapshots {
		rows[i] = []any{s.Time, s.StationID, s.NumBikesAvailable, s.NumDocksAvailable, s.IsInstalled, s.IsRenting, s.IsReturning, s.VehicleDocksJSON}
	}
	_, err := p.CopyFrom(ctx,
		pgx.Identifier{"station_snapshots"},
		[]string{"time", "station_id", "num_bikes_available", "num_docks_available", "is_installed", "is_renting", "is_returning", "vehicle_docks_available"},
		pgx.CopyFromRows(rows),
	)
	return err
}

// InsertTrip writes a completed trip.
func (p *Pool) InsertTrip(ctx context.Context,
	bikeID string,
	startTime, endTime time.Time,
	startStationID, endStationID *string,
	startLat, startLon, endLat, endLon float64,
	distanceMeters, batteryStart, batteryEnd int,
) error {
	batteryDelta := batteryEnd - batteryStart
	_, err := p.Exec(ctx, `
		INSERT INTO trips (bike_id, start_time, end_time,
			start_station_id, end_station_id,
			start_lat, start_lon, end_lat, end_lon,
			distance_meters, battery_start, battery_end, battery_delta)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, bikeID, startTime, endTime,
		startStationID, endStationID,
		startLat, startLon, endLat, endLon,
		distanceMeters, batteryStart, batteryEnd, batteryDelta)
	return err
}

// BikeStateRow holds the latest snapshot data for a single bike, used to
// hydrate the trip detector after a restart.
type BikeStateRow struct {
	BikeID      string
	Lat         float64
	Lon         float64
	StationID   *string
	IsDisabled  bool
	RangeMeters int
	SeenAt      time.Time
}

// FetchLatestBikeStates returns the most recent snapshot per bike for bikes
// seen within the last 2 hours. The 2-hour window matches the runtime state
// cleanup threshold so that hydrated state and in-memory state stay consistent.
func (p *Pool) FetchLatestBikeStates(ctx context.Context) ([]BikeStateRow, error) {
	rows, err := p.Pool.Query(ctx, `
		SELECT DISTINCT ON (bike_id)
			bike_id, lat, lon, station_id, is_disabled, current_range_meters, time
		FROM bike_snapshots
		WHERE time > NOW() - INTERVAL '2 hours'
		ORDER BY bike_id, time DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []BikeStateRow
	for rows.Next() {
		var r BikeStateRow
		if err := rows.Scan(&r.BikeID, &r.Lat, &r.Lon, &r.StationID, &r.IsDisabled, &r.RangeMeters, &r.SeenAt); err == nil {
			result = append(result, r)
		}
	}
	return result, rows.Err()
}

// FetchBikePathPoints returns ordered (lat, lon) pairs for free-floating snapshots
// of a given bike between from and to, used for GPS-path distance calculation.
func (p *Pool) FetchBikePathPoints(ctx context.Context, bikeID string, from, to time.Time) ([][2]float64, error) {
	rows, err := p.Pool.Query(ctx, `
		SELECT lat, lon FROM bike_snapshots
		WHERE bike_id = $1
		  AND time >= $2 AND time <= $3
		  AND station_id IS NULL
		ORDER BY time ASC
	`, bikeID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pts [][2]float64
	for rows.Next() {
		var lat, lon float64
		if err := rows.Scan(&lat, &lon); err == nil {
			pts = append(pts, [2]float64{lat, lon})
		}
	}
	return pts, rows.Err()
}

// NotifyPollDone sends a PostgreSQL NOTIFY so the backend can push WS updates.
func (p *Pool) NotifyPollDone(ctx context.Context) error {
	conn, err := p.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	_, err = conn.Exec(ctx, "SELECT pg_notify('poll_done', '')")
	return err
}
