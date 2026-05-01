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

	client := gbfs.NewClient(cfg.GBFSBaseURL)
	detector := trip.NewDetector(pool)

	slog.Info("poller started", "interval", cfg.PollInterval)

	// Run immediately on startup, then on ticker
	runPoll := func() {
		pollCtx, pollCancel := context.WithTimeout(ctx, 30*time.Second)
		defer pollCancel()

		// Run both jobs; station job first to ensure station rows exist before bike FK refs
		jobs.PollStations(pollCtx, client, pool)
		jobs.PollBikes(pollCtx, client, pool, detector)

		// Notify backend via PostgreSQL NOTIFY
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
