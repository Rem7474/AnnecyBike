package jobs

import (
	"context"
	"log/slog"

	"github.com/annecybike/poller/internal/db"
	"github.com/annecybike/poller/internal/gbfs"
)

// PollGeofencing fetches geofencing_zones.json and persists it.
// Called once at startup and then hourly — zones rarely change.
func PollGeofencing(ctx context.Context, client *gbfs.Client, pool *db.Pool) {
	feed, err := client.GeofencingZones(ctx)
	if err != nil {
		slog.Warn("fetch geofencing_zones failed", "err", err)
		return
	}
	if err := pool.UpsertGeofencingZones(ctx, feed.Data.GeofencingZones); err != nil {
		slog.Error("upsert geofencing_zones failed", "err", err)
		return
	}
	slog.Info("geofencing zones updated")
}
