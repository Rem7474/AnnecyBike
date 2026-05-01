package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/annecybike/backend/internal/db"
	"github.com/annecybike/backend/internal/models"
	"github.com/gin-gonic/gin"
)

func GetTrips(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		if limit > 200 {
			limit = 200
		}

		bikeID := c.Query("bike_id")
		stationID := c.Query("station_id")
		from, to := parseTimeRange(c, 7*24*time.Hour)

		query := `
			SELECT t.id, t.bike_id, t.start_time, t.end_time,
				t.start_station_id, t.end_station_id,
				ss.name AS start_station_name, es.name AS end_station_name,
				t.start_lat, t.start_lon, t.end_lat, t.end_lon,
				t.distance_meters, t.battery_start, t.battery_end, t.battery_delta,
				EXTRACT(EPOCH FROM (t.end_time - t.start_time)) / 60.0 AS duration_minutes
			FROM trips t
			LEFT JOIN stations ss ON ss.station_id = t.start_station_id
			LEFT JOIN stations es ON es.station_id = t.end_station_id
			WHERE t.start_time BETWEEN $1 AND $2
		`
		args := []any{from, to}
		argIdx := 3
		if bikeID != "" {
			query += ` AND bike_id = $` + strconv.Itoa(argIdx)
			args = append(args, bikeID)
			argIdx++
		}
		if stationID != "" {
			query += ` AND (start_station_id = $` + strconv.Itoa(argIdx) + ` OR end_station_id = $` + strconv.Itoa(argIdx) + `)`
			args = append(args, stationID)
			argIdx++
		}
		query += ` ORDER BY start_time DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
		args = append(args, limit, offset)

		rows, err := pool.Query(c.Request.Context(), query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		trips := make([]models.Trip, 0, limit)
		for rows.Next() {
			t := scanTrip(rows)
			trips = append(trips, t)
		}
		c.JSON(http.StatusOK, trips)
	}
}

func GetBikeTrips(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		bikeID := c.Param("id")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		if limit > 200 {
			limit = 200
		}

		rows, err := pool.Query(c.Request.Context(), `
			SELECT t.id, t.bike_id, t.start_time, t.end_time,
				t.start_station_id, t.end_station_id,
				ss.name AS start_station_name, es.name AS end_station_name,
				t.start_lat, t.start_lon, t.end_lat, t.end_lon,
				t.distance_meters, t.battery_start, t.battery_end, t.battery_delta,
				EXTRACT(EPOCH FROM (t.end_time - t.start_time)) / 60.0 AS duration_minutes
			FROM trips t
			LEFT JOIN stations ss ON ss.station_id = t.start_station_id
			LEFT JOIN stations es ON es.station_id = t.end_station_id
			WHERE t.bike_id = $1
			ORDER BY t.start_time DESC
			LIMIT $2 OFFSET $3
		`, bikeID, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		trips := make([]models.Trip, 0, limit)
		for rows.Next() {
			trips = append(trips, scanTrip(rows))
		}
		c.JSON(http.StatusOK, trips)
	}
}

type scanner interface {
	Scan(...any) error
}

func scanTrip(row scanner) models.Trip {
	var t models.Trip
	_ = row.Scan(
		&t.ID, &t.BikeID, &t.StartTime, &t.EndTime,
		&t.StartStationID, &t.EndStationID,
		&t.StartStationName, &t.EndStationName,
		&t.StartLat, &t.StartLon, &t.EndLat, &t.EndLon,
		&t.DistanceMeters, &t.BatteryStart, &t.BatteryEnd, &t.BatteryDelta,
		&t.DurationMinutes,
	)
	return t
}
