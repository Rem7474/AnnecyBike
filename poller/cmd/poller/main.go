package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/annecybike/poller/internal/config"
	"github.com/annecybike/poller/internal/db"
	"github.com/annecybike/poller/internal/gbfs"
	"github.com/annecybike/poller/internal/jobs"
	"github.com/annecybike/poller/internal/routing"
	"github.com/annecybike/poller/internal/trip"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Wait for DB to be ready (Docker health check should handle this, but be defensive)
	var pool *db.Pool
	for i := range 20 {
		pool, err = db.Connect(ctx, cfg.DBURL)
		if err == nil {
			break
		}
		slog.Warn("DB not ready, retrying...", "attempt", i+1, "err", err)
		select {
		case <-ctx.Done():
			os.Exit(0)
		case <-time.After(3 * time.Second):
		}
	}
	if pool == nil {
		slog.Error("could not connect to DB after retries")
		os.Exit(1)
	}
	defer pool.Close()

	// GBFS auto-discovery: fetch gbfs.json to resolve all feed URLs.
	client, err := gbfs.NewClient(ctx, cfg.GBFSURL)
	if err != nil {
		slog.Error("GBFS discovery failed", "url", cfg.GBFSURL, "err", err)
		os.Exit(1)
	}
	slog.Info("GBFS discovery complete", "url", cfg.GBFSURL)

	detector := trip.NewDetector(pool)
	if err := detector.HydrateState(ctx); err != nil {
		slog.Warn("could not hydrate detector state from DB", "err", err)
	}

	// Build OSRM routing matrix for accurate travel-time matching (ID rotation).
	buildMatrix := func() {
		mCtx, mCancel := context.WithTimeout(ctx, 30*time.Second)
		defer mCancel()
		stations, err := pool.FetchAllStationCoords(mCtx)
		if err != nil {
			slog.Warn("could not fetch station coords for routing matrix", "err", err)
			return
		}
		pts := make([]routing.StationPoint, len(stations))
		for i, s := range stations {
			pts[i] = routing.StationPoint{ID: s.ID, Lat: s.Lat, Lon: s.Lon}
		}
		m, err := routing.BuildMatrix(mCtx, cfg.OSRMURL, pts)
		if err != nil {
			slog.Warn("OSRM matrix unavailable, falling back to haversine", "err", err)
			return
		}
		detector.SetMatrix(m)
		slog.Info("OSRM routing matrix built", "stations", len(stations), "osrm", cfg.OSRMURL)
	}
	buildMatrix()

	slog.Info("poller started", "interval", cfg.PollInterval)

	// Fetch geofencing zones once at startup, then every hour.
	// Also rebuild the routing matrix hourly (stations rarely change).
	jobs.PollGeofencing(ctx, client, pool)
	geoTicker := time.NewTicker(time.Hour)
	defer geoTicker.Stop()
	go func() {
		for {
			select {
			case <-geoTicker.C:
				pollCtx, pollCancel := context.WithTimeout(ctx, 30*time.Second)
				jobs.PollGeofencing(pollCtx, client, pool)
				pollCancel()
				buildMatrix()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Regular poll: stations first (ensures FK rows exist before bike snapshots).
	runPoll := func() {
		pollCtx, pollCancel := context.WithTimeout(ctx, 30*time.Second)
		defer pollCancel()

		jobs.PollStations(pollCtx, client, pool)
		jobs.PollBikes(pollCtx, client, pool, detector)

		if err := pool.NotifyPollDone(ctx); err != nil {
			slog.Warn("pg_notify failed", "err", err)
		}
	}

	runPoll()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runPoll()
		case <-ctx.Done():
			slog.Info("poller shutting down")
			return
		}
	}
}
