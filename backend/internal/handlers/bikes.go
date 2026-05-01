package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/annecybike/backend/internal/db"
	"github.com/annecybike/backend/internal/models"
	"github.com/gin-gonic/gin"
)

func GetBikesLive(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := pool.Query(c.Request.Context(), `
			SELECT DISTINCT ON (bs.bike_id)
				bs.bike_id, b.vehicle_type_id,
				bs.lat, bs.lon, bs.station_id,
				bs.is_reserved, bs.is_disabled, bs.current_range_meters
			FROM bike_snapshots bs
			JOIN bikes b ON b.bike_id = bs.bike_id
			WHERE bs.time > NOW() - INTERVAL '2 minutes'
			ORDER BY bs.bike_id, bs.time DESC
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		bikes := make([]models.BikeLive, 0, 200)
		for rows.Next() {
			var b models.BikeLive
			if err := rows.Scan(&b.BikeID, &b.VehicleTypeID,
				&b.Lat, &b.Lon, &b.StationID,
				&b.IsReserved, &b.IsDisabled, &b.CurrentRangeMeters); err != nil {
				continue
			}
			b.BatteryPct = batteryPct(b.CurrentRangeMeters)
			bikes = append(bikes, b)
		}
		c.JSON(http.StatusOK, bikes)
	}
}

func GetBikes(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		if limit > 500 {
			limit = 500
		}

		rows, err := pool.Query(c.Request.Context(), `
			SELECT bike_id, vehicle_type_id, first_seen, last_seen
			FROM bikes
			ORDER BY last_seen DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		bikes := make([]models.Bike, 0, limit)
		for rows.Next() {
			var b models.Bike
			if err := rows.Scan(&b.BikeID, &b.VehicleTypeID, &b.FirstSeen, &b.LastSeen); err != nil {
				continue
			}
			bikes = append(bikes, b)
		}
		c.JSON(http.StatusOK, bikes)
	}
}

func GetBike(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		bikeID := c.Param("id")
		var b models.Bike
		err := pool.QueryRow(c.Request.Context(),
			`SELECT bike_id, vehicle_type_id, first_seen, last_seen FROM bikes WHERE bike_id = $1`, bikeID).
			Scan(&b.BikeID, &b.VehicleTypeID, &b.FirstSeen, &b.LastSeen)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "bike not found"})
			return
		}
		c.JSON(http.StatusOK, b)
	}
}

func GetBikeHistory(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		bikeID := c.Param("id")
		from, to := parseTimeRange(c, 24*time.Hour)
		resolution := c.DefaultQuery("resolution", "raw")

		var rows interface{ Close() }
		var err error

		switch resolution {
		case "1h":
			rows, err = pool.Query(c.Request.Context(), `
				SELECT time_bucket('1 hour', time) AS time,
					AVG(lat) AS lat, AVG(lon) AS lon,
					MODE() WITHIN GROUP (ORDER BY station_id) AS station_id,
					BOOL_OR(is_disabled) AS is_disabled,
					AVG(current_range_meters)::INT AS current_range_meters
				FROM bike_snapshots
				WHERE bike_id = $1 AND time BETWEEN $2 AND $3
				GROUP BY 1 ORDER BY 1
			`, bikeID, from, to)
		case "1d":
			rows, err = pool.Query(c.Request.Context(), `
				SELECT time_bucket('1 day', time) AS time,
					AVG(lat) AS lat, AVG(lon) AS lon,
					MODE() WITHIN GROUP (ORDER BY station_id) AS station_id,
					BOOL_OR(is_disabled) AS is_disabled,
					AVG(current_range_meters)::INT AS current_range_meters
				FROM bike_snapshots
				WHERE bike_id = $1 AND time BETWEEN $2 AND $3
				GROUP BY 1 ORDER BY 1
			`, bikeID, from, to)
		default:
			rows, err = pool.Query(c.Request.Context(), `
				SELECT time, lat, lon, station_id, is_disabled, current_range_meters
				FROM bike_snapshots
				WHERE bike_id = $1 AND time BETWEEN $2 AND $3
				ORDER BY time DESC
				LIMIT 2000
			`, bikeID, from, to)
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// rows is pgx.Rows in all cases
		type pgRows interface {
			Next() bool
			Scan(...any) error
			Close()
		}
		pgr := rows.(pgRows)
		defer pgr.Close()

		result := make([]models.BikeSnapshot, 0)
		for pgr.Next() {
			var s models.BikeSnapshot
			s.BikeID = bikeID
			if err := pgr.Scan(&s.Time, &s.Lat, &s.Lon, &s.StationID, &s.IsDisabled, &s.CurrentRangeMeters); err != nil {
				continue
			}
			result = append(result, s)
		}
		c.JSON(http.StatusOK, result)
	}
}

func GetBikeStats(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		bikeID := c.Param("id")
		ctx := c.Request.Context()

		var stats models.BikeStats
		stats.BikeID = bikeID

		// Total trips and distance
		_ = pool.QueryRow(ctx, `
			SELECT COUNT(*), COALESCE(SUM(distance_meters), 0) / 1000.0
			FROM trips WHERE bike_id = $1 AND end_time IS NOT NULL
		`, bikeID).Scan(&stats.TotalTrips, &stats.TotalDistanceKm)

		// Average battery %
		var avgRange *float64
		_ = pool.QueryRow(ctx, `
			SELECT AVG(current_range_meters) FROM bike_snapshots
			WHERE bike_id = $1 AND time > NOW() - INTERVAL '7 days'
		`, bikeID).Scan(&avgRange)
		if avgRange != nil {
			stats.AvgBatteryPct = batteryPct(int(*avgRange))
		}

		// Availability and disabled ratios (last 7 days)
		var total, disabled int
		_ = pool.QueryRow(ctx, `
			SELECT COUNT(*), COUNT(*) FILTER (WHERE is_disabled)
			FROM bike_snapshots
			WHERE bike_id = $1 AND time > NOW() - INTERVAL '7 days'
		`, bikeID).Scan(&total, &disabled)
		if total > 0 {
			stats.AvailabilityPct = float64(total-disabled) / float64(total) * 100
			stats.DisabledPct = float64(disabled) / float64(total) * 100
		}

		c.JSON(http.StatusOK, stats)
	}
}

// batteryPct converts current_range_meters to a percentage (max 45000m).
func batteryPct(rangeMeters int) int {
	const maxRange = 45_000
	if rangeMeters <= 0 {
		return 0
	}
	pct := rangeMeters * 100 / maxRange
	if pct > 100 {
		return 100
	}
	return pct
}
