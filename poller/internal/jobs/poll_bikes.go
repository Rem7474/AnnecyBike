package jobs

import (
	"context"
	"log/slog"
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

		snaps = append(snaps, db.BikeSnapshot{
			Time:               now,
			BikeID:             b.BikeID,
			Lat:                b.Lat,
			Lon:                b.Lon,
			StationID:          stationID,
			IsReserved:         b.IsReserved,
			IsDisabled:         b.IsDisabled,
			CurrentRangeMeters: b.CurrentRangeMeters,
		})

		currentState[b.BikeID] = trip.BikeState{
			StationID:  stationID,
			Lat:        b.Lat,
			Lon:        b.Lon,
			Battery:    b.CurrentRangeMeters,
			IsDisabled: b.IsDisabled,
			SeenAt:     now,
		}
	}

	if err := pool.BulkInsertBikeSnapshots(ctx, snaps); err != nil {
		slog.Error("bulk insert bike snapshots failed", "err", err)
		return
	}

	detector.Process(ctx, now, currentState)
	slog.Info("bikes polled", "count", len(snaps))
}
