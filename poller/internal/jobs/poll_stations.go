package jobs

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/annecybike/poller/internal/db"
	"github.com/annecybike/poller/internal/gbfs"
)

func PollStations(ctx context.Context, client *gbfs.Client, pool *db.Pool) {
	now := time.Now().UTC()

	info, err := client.StationInfo(ctx)
	if err != nil {
		slog.Error("fetch station_information failed", "err", err)
		return
	}
	status, err := client.StationStatus(ctx)
	if err != nil {
		slog.Error("fetch station_status failed", "err", err)
		return
	}

	// Upsert static station data
	for _, s := range info.Data.Stations {
		if err := pool.UpsertStation(ctx, s.StationID, s.Name, s.Lat, s.Lon, s.Capacity,
			s.VehicleTypeCapacity, s.IsVirtualStation, s.IsChargingStation); err != nil {
			slog.Error("upsert station failed", "station", s.StationID, "err", err)
		}
	}

	// Build snapshot rows
	snaps := make([]db.StationSnapshot, 0, len(status.Data.Stations))
	for _, s := range status.Data.Stations {
		vtJSON, _ := json.Marshal(s.VehicleTypesAvailable)
		snaps = append(snaps, db.StationSnapshot{
			Time:              now,
			StationID:         s.StationID,
			NumBikesAvailable: s.NumBikesAvailable,
			NumDocksAvailable: s.NumDocksAvailable,
			IsInstalled:       s.IsInstalled,
			IsRenting:         s.IsRenting,
			IsReturning:       s.IsReturning,
			VehicleDocksJSON:  vtJSON,
		})
	}
	if err := pool.BulkInsertStationSnapshots(ctx, snaps); err != nil {
		slog.Error("bulk insert station snapshots failed", "err", err)
		return
	}
	slog.Info("stations polled", "count", len(snaps))
}
