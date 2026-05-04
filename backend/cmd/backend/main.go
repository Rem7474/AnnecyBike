package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/annecybike/backend/internal/config"
	"github.com/annecybike/backend/internal/db"
	"github.com/annecybike/backend/internal/handlers"
	"github.com/gin-gonic/gin"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var pool *db.Pool
	for i := range 20 {
		pool, err = db.Connect(ctx, cfg.DBURL)
		if err == nil {
			break
		}
		slog.Warn("DB not ready", "attempt", i+1, "err", err)
		select {
		case <-ctx.Done():
			os.Exit(0)
		case <-time.After(3 * time.Second):
		}
	}
	if pool == nil {
		slog.Error("cannot connect to DB")
		os.Exit(1)
	}
	defer pool.Close()

	hub := handlers.NewHub()
	handlers.StartListener(ctx, pool, hub)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	v1 := r.Group("/api/v1")
	{
		v1.GET("/bikes/live", handlers.GetBikesLive(pool))
		v1.GET("/bikes/nearest", handlers.GetNearestBikes(pool))
		v1.GET("/bikes", handlers.GetBikes(pool))
		v1.GET("/bikes/:id", handlers.GetBike(pool))
		v1.GET("/bikes/:id/history", handlers.GetBikeHistory(pool))
		v1.GET("/bikes/:id/trips", handlers.GetBikeTrips(pool))
		v1.GET("/bikes/:id/stats", handlers.GetBikeStats(pool))
		v1.GET("/bikes/:id/health", handlers.GetBikeHealth(pool))
		v1.PATCH("/bikes/:id/reassign", handlers.ReassignBike(pool))

		v1.GET("/stations/live", handlers.GetStationsLive(pool))
		v1.GET("/stations/nearest", handlers.GetNearestStations(pool))
		v1.GET("/stations/:id", handlers.GetStation(pool))
		v1.GET("/stations/:id/bikes", handlers.GetStationBikes(pool))
		v1.GET("/stations/:id/history", handlers.GetStationHistory(pool))
		v1.GET("/stations/:id/bike-history", handlers.GetStationBikeHistory(pool))

		v1.GET("/physical-bikes", handlers.GetPhysicalBikes(pool))
		v1.GET("/physical-bikes/:pid", handlers.GetPhysicalBike(pool))
		v1.PATCH("/physical-bikes/:pid", handlers.UpdatePhysicalBike(pool))
		v1.GET("/physical-bikes/:pid/trips", handlers.GetPhysicalBikeTrips(pool))
		v1.GET("/physical-bikes/:pid/history", handlers.GetPhysicalBikeHistory(pool))

		v1.GET("/trips", handlers.GetTrips(pool))

		v1.GET("/anomalies", handlers.GetAnomalies(pool))

		v1.GET("/geofencing", handlers.GetGeofencingZones(pool))

		v1.GET("/replay", handlers.GetReplay(pool))

		v1.GET("/stats/fleet", handlers.GetFleetStats(pool))
		v1.GET("/stats/heatmap", handlers.GetHeatmap(pool))
		v1.GET("/stats/trips-per-day", handlers.GetTripsPerDay(pool))
		v1.GET("/stats/battery-distribution", handlers.GetBatteryDistribution(pool))
		v1.GET("/stats/busiest-stations", handlers.GetBusiestStations(pool))
	}

	r.GET("/ws/live", handlers.WSHandler(hub))

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		slog.Info("backend listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	slog.Info("backend stopped")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
