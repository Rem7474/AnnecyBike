package jobs

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/annecybike/poller/internal/db"
	"github.com/annecybike/poller/internal/gbfs"
	"github.com/annecybike/poller/internal/trip"
)

func PollBikes(ctx context.Context, client *gbfs.Client, pool *db.Pool, detector *trip.Detector) {
	now := time.Now().UTC()

	// Refresh vehicle types (rarely changes but keep up to date)
	vtFeed, err := client.VehicleTypes(ctx)
	if err != nil {
		slog.Error("fetch vehicle_types failed", "err", err)
	} else {
		for _, vt := range vtFeed.Data.VehicleTypes {
			if err := pool.UpsertVehicleType(ctx, vt.VehicleTypeID, vt.Name, vt.FormFactor, vt.PropulsionType, vt.MaxRangeMeters); err != nil {
				slog.Error("upsert vehicle_type failed", "id", vt.VehicleTypeID, "err", err)
			}
		}
	}

	bikeFeed, err := client.FreeBikeStatus(ctx)
	if err != nil {
		slog.Error("fetch free_bike_status failed", "err", err)
		return
	}

	// Station coords used as fallback when a docked bike omits lat/lon (spec allows this).
	stationCoords, err := pool.QueryStationCoords(ctx)
	if err != nil {
		slog.Warn("could not load station coords", "err", err)
		stationCoords = map[string][2]float64{}
	}

	snaps := make([]db.BikeSnapshot, 0, len(bikeFeed.Data.Bikes))
	currentState := make(map[string]trip.BikeState, len(bikeFeed.Data.Bikes))

	for _, b := range bikeFeed.Data.Bikes {
		// Ensure bike is registered
		if err := pool.UpsertBike(ctx, b.BikeID, b.VehicleTypeID); err != nil {
			slog.Error("upsert bike failed", "bike", b.BikeID, "err", err)
		}

		var stationID *string
		if b.StationID != "" {
			sid := b.StationID
			stationID = &sid
		}

		// spec: lat/lon are conditional — docked bikes may omit them; fall back to station coords.
		lat, lon := b.Lat, b.Lon
		if lat == 0 && lon == 0 && stationID != nil {
			if coords, ok := stationCoords[*stationID]; ok {
				lat, lon = coords[0], coords[1]
			}
		}

		rangeMeters := int(math.Round(b.CurrentRangeMeters))

		snaps = append(snaps, db.BikeSnapshot{
			Time:               now,
			BikeID:             b.BikeID,
			Lat:                lat,
			Lon:                lon,
			StationID:          stationID,
			IsReserved:         b.IsReserved,
			IsDisabled:         b.IsDisabled,
			CurrentRangeMeters: rangeMeters,
		})

		currentState[b.BikeID] = trip.BikeState{
			VehicleTypeID: b.VehicleTypeID,
			StationID:     stationID,
			Lat:           lat,
			Lon:           lon,
			Battery:       rangeMeters,
			IsDisabled:    b.IsDisabled,
			SeenAt:        now,
		}
	}

	if err := pool.BulkInsertBikeSnapshots(ctx, snaps); err != nil {
		slog.Error("bulk insert bike snapshots failed", "err", err)
		return
	}

	detector.Process(ctx, now, currentState)
	slog.Info("bikes polled", "count", len(snaps))
}
