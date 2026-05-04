// recalc retroactively recomputes distance_meters for every completed trip by
// summing consecutive haversine distances along the GPS path recorded in
// bike_snapshots (station_id IS NULL points only).  This replaces the original
// point-to-point haversine × 1.3 estimate that was stored when trips were first
// inserted.
//
// Usage (Docker):
//
//	docker compose run --rm poller /recalc
//	DRY_RUN=true docker compose run --rm poller /recalc   # preview only
package main

import (
	"context"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		slog.Error("DB_URL is required")
		os.Exit(1)
	}
	dryRun := os.Getenv("DRY_RUN") == "true"
	if dryRun {
		slog.Info("dry-run mode: no writes will be performed")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("DB connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Preload station coordinates for lat/lon=0 fallback.
	stationCoords := loadStationCoords(ctx, pool)
	slog.Info("station coords loaded", "count", len(stationCoords))

	// Count trips to process.
	var total int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM trips WHERE end_time IS NOT NULL`).Scan(&total); err != nil {
		slog.Error("count trips failed", "err", err)
		os.Exit(1)
	}
	slog.Info("trips to recalculate", "total", total, "dry_run", dryRun)

	const batchSize = 500
	updated, skipped := 0, 0
	offset := 0

	for {
		if ctx.Err() != nil {
			slog.Info("interrupted", "updated", updated, "skipped", skipped)
			return
		}

		rows, err := pool.Query(ctx, `
			SELECT id, bike_id, start_time, end_time,
			       start_lat, start_lon, start_station_id,
			       end_lat, end_lon, end_station_id
			FROM trips
			WHERE end_time IS NOT NULL
			ORDER BY start_time ASC
			LIMIT $1 OFFSET $2
		`, batchSize, offset)
		if err != nil {
			slog.Error("fetch trips batch failed", "offset", offset, "err", err)
			os.Exit(1)
		}

		type tripRow struct {
			id                           int64
			bikeID                       string
			startTime, endTime           time.Time
			startLat, startLon           float64
			startSID                     *string
			endLat, endLon               float64
			endSID                       *string
		}
		var batch []tripRow
		for rows.Next() {
			var t tripRow
			if err := rows.Scan(
				&t.id, &t.bikeID, &t.startTime, &t.endTime,
				&t.startLat, &t.startLon, &t.startSID,
				&t.endLat, &t.endLon, &t.endSID,
			); err != nil {
				slog.Warn("scan trip failed", "err", err)
				continue
			}
			batch = append(batch, t)
		}
		rows.Close()

		if len(batch) == 0 {
			break
		}

		// Compute distances and collect updates.
		type update struct {
			id   int64
			dist int
		}
		var updates []update

		for _, t := range batch {
			// Resolve 0,0 endpoints from station coordinates.
			startLat, startLon := resolveCoords(t.startLat, t.startLon, t.startSID, stationCoords)
			endLat, endLon := resolveCoords(t.endLat, t.endLon, t.endSID, stationCoords)

			dist := gpsPathDistance(ctx, pool, t.bikeID, t.startTime, t.endTime, startLat, startLon, endLat, endLon)
			updates = append(updates, update{t.id, dist})
		}

		if !dryRun && len(updates) > 0 {
			tx, err := pool.Begin(ctx)
			if err != nil {
				slog.Error("begin tx failed", "err", err)
				os.Exit(1)
			}
			for _, u := range updates {
				if _, err := tx.Exec(ctx, `UPDATE trips SET distance_meters = $1 WHERE id = $2`, u.dist, u.id); err != nil {
					_ = tx.Rollback(ctx)
					slog.Error("update trip failed", "id", u.id, "err", err)
					os.Exit(1)
				}
			}
			if err := tx.Commit(ctx); err != nil {
				slog.Error("commit failed", "err", err)
				os.Exit(1)
			}
			updated += len(updates)
		} else {
			skipped += len(updates)
		}

		if (updated+skipped)%5000 == 0 || len(batch) < batchSize {
			slog.Info("progress", "updated", updated, "skipped", skipped, "total", total)
		}

		offset += len(batch)
		if len(batch) < batchSize {
			break
		}
	}

	slog.Info("recalculation complete", "updated", updated, "skipped", skipped, "dry_run", dryRun)
}

// gpsPathDistance sums haversine distances along the GPS path stored in
// bike_snapshots for the given bike between startTime and endTime.
// Falls back to haversine(start, end) when fewer than 2 free-floating points exist.
func gpsPathDistance(ctx context.Context, pool *pgxpool.Pool, bikeID string, startTime, endTime time.Time, startLat, startLon, endLat, endLon float64) int {
	rows, err := pool.Query(ctx, `
		SELECT lat, lon FROM bike_snapshots
		WHERE bike_id = $1
		  AND time >= $2 AND time <= $3
		  AND station_id IS NULL
		  AND lat != 0 AND lon != 0
		ORDER BY time ASC
	`, bikeID, startTime, endTime)
	if err != nil {
		return haversine(startLat, startLon, endLat, endLon)
	}
	defer rows.Close()

	type pt struct{ lat, lon float64 }
	pts := []pt{{startLat, startLon}} // prepend departure coords
	for rows.Next() {
		var p pt
		if err := rows.Scan(&p.lat, &p.lon); err == nil && (p.lat != 0 || p.lon != 0) {
			pts = append(pts, p)
		}
	}
	pts = append(pts, pt{endLat, endLon}) // append arrival coords

	// Need at least start + 1 GPS point + end to be better than direct line.
	if len(pts) < 3 {
		return haversine(startLat, startLon, endLat, endLon)
	}

	var total float64
	for i := 1; i < len(pts); i++ {
		total += haversineRaw(pts[i-1].lat, pts[i-1].lon, pts[i].lat, pts[i].lon)
	}
	return int(total)
}

func loadStationCoords(ctx context.Context, pool *pgxpool.Pool) map[string][2]float64 {
	m := make(map[string][2]float64)
	rows, err := pool.Query(ctx, `SELECT station_id, lat, lon FROM stations`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var lat, lon float64
		if err := rows.Scan(&id, &lat, &lon); err == nil {
			m[id] = [2]float64{lat, lon}
		}
	}
	return m
}

func resolveCoords(lat, lon float64, sid *string, coords map[string][2]float64) (float64, float64) {
	if (lat != 0 || lon != 0) || sid == nil {
		return lat, lon
	}
	if c, ok := coords[*sid]; ok {
		return c[0], c[1]
	}
	return lat, lon
}

func haversineRaw(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6_371_000.0
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func haversine(lat1, lon1, lat2, lon2 float64) int {
	return int(haversineRaw(lat1, lon1, lat2, lon2) * 1.3)
}

// pgx.Rows is used implicitly via pgxpool; ensure import is used.
var _ pgx.Rows
