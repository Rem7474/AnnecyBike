package handlers

import (
	"net/http"
	"strconv"

	"github.com/annecybike/backend/internal/db"
	"github.com/annecybike/backend/internal/models"
	"github.com/gin-gonic/gin"
)

func GetFleetStats(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var stats models.FleetStats

		// Count only bikes seen in the last poll cycle — avoids counting retired bikes
		// whose IDs are still in the bikes table from previous fleet rotations.
		_ = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM bikes
			WHERE last_seen > NOW() - INTERVAL '2 minutes'
		`).Scan(&stats.TotalBikes)

		_ = pool.QueryRow(ctx, `
			SELECT
				COUNT(*) FILTER (WHERE NOT is_disabled AND NOT is_reserved),
				COUNT(*) FILTER (WHERE is_disabled),
				COUNT(*) FILTER (WHERE is_reserved)
			FROM (
				SELECT DISTINCT ON (bike_id) bike_id, is_disabled, is_reserved
				FROM bike_snapshots
				WHERE time > NOW() - INTERVAL '2 minutes'
				ORDER BY bike_id, time DESC
			) latest
		`).Scan(&stats.AvailableNow, &stats.DisabledNow, &stats.ReservedNow)

		_ = pool.QueryRow(ctx, `
			SELECT
				COUNT(*) FILTER (WHERE start_time >= CURRENT_DATE),
				COUNT(*) FILTER (WHERE start_time >= NOW() - INTERVAL '7 days')
			FROM trips
			WHERE NOT (
			    start_station_id IS NOT DISTINCT FROM end_station_id
			    AND end_time - start_time < INTERVAL '10 minutes'
			    AND COALESCE(distance_meters, 0) < 200
			)
		`).Scan(&stats.TripsToday, &stats.TripsWeek)

		c.JSON(http.StatusOK, stats)
	}
}

func GetTripsPerDay(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
		if days > 365 {
			days = 365
		}

		// TO_CHAR ensures the date is returned as text "YYYY-MM-DD", which pgx
		// scans cleanly into string without relying on implicit date codec behaviour.
		rows, err := pool.Query(c.Request.Context(), `
			SELECT TO_CHAR(DATE(start_time AT TIME ZONE 'Europe/Paris'), 'YYYY-MM-DD') AS date,
			       COUNT(*) AS count
			FROM trips
			WHERE start_time >= NOW() - make_interval(days => $1)
			  AND NOT (
			    start_station_id IS NOT DISTINCT FROM end_station_id
			    AND end_time - start_time < INTERVAL '10 minutes'
			    AND COALESCE(distance_meters, 0) < 200
			  )
			GROUP BY 1 ORDER BY 1
		`, days)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.DailyCount, 0, days)
		for rows.Next() {
			var d models.DailyCount
			if err := rows.Scan(&d.Date, &d.Count); err != nil {
				continue
			}
			result = append(result, d)
		}
		c.JSON(http.StatusOK, result)
	}
}

func GetBatteryDistribution(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		rows, err := pool.Query(ctx, `
			SELECT
				CASE
					WHEN current_range_meters < 4500  THEN '0-10%'
					WHEN current_range_meters < 9000  THEN '10-20%'
					WHEN current_range_meters < 13500 THEN '20-30%'
					WHEN current_range_meters < 18000 THEN '30-40%'
					WHEN current_range_meters < 22500 THEN '40-50%'
					WHEN current_range_meters < 27000 THEN '50-60%'
					WHEN current_range_meters < 31500 THEN '60-70%'
					WHEN current_range_meters < 36000 THEN '70-80%'
					WHEN current_range_meters < 40500 THEN '80-90%'
					ELSE '90-100%'
				END AS range,
				COUNT(*) AS count
			FROM (
				SELECT DISTINCT ON (bike_id) bike_id, current_range_meters
				FROM bike_snapshots
				WHERE time > NOW() - INTERVAL '2 minutes'
					AND current_range_meters > 0
				ORDER BY bike_id, time DESC
			) latest
			GROUP BY 1 ORDER BY 1
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.BatteryBucket, 0, 10)
		for rows.Next() {
			var b models.BatteryBucket
			if err := rows.Scan(&b.Range, &b.Count); err != nil {
				continue
			}
			result = append(result, b)
		}
		c.JSON(http.StatusOK, result)
	}
}

// GetHeatmap returns trip start+end points for the heatmap layer.
func GetHeatmap(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
		if days > 365 {
			days = 365
		}

		rows, err := pool.Query(c.Request.Context(), `
			SELECT lat, lon, weight FROM (
				SELECT ROUND(start_lat::numeric, 4) AS lat,
				       ROUND(start_lon::numeric, 4) AS lon,
				       COUNT(*) AS weight
				FROM trips
				WHERE start_time > NOW() - make_interval(days => $1)
				  AND start_lat IS NOT NULL AND start_lat != 0
				  AND NOT (
				    start_station_id IS NOT DISTINCT FROM end_station_id
				    AND end_time - start_time < INTERVAL '10 minutes'
				    AND COALESCE(distance_meters, 0) < 200
				  )
				GROUP BY 1, 2

				UNION ALL

				SELECT ROUND(end_lat::numeric, 4),
				       ROUND(end_lon::numeric, 4),
				       COUNT(*)
				FROM trips
				WHERE start_time > NOW() - make_interval(days => $1)
				  AND end_lat IS NOT NULL AND end_lat != 0
				  AND end_station_id IS NOT NULL
				  AND NOT (
				    start_station_id IS NOT DISTINCT FROM end_station_id
				    AND end_time - start_time < INTERVAL '10 minutes'
				    AND COALESCE(distance_meters, 0) < 200
				  )
				GROUP BY 1, 2
			) combined
			WHERE weight > 0
			ORDER BY weight DESC
			LIMIT 5000
		`, days)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.HeatPoint, 0)
		for rows.Next() {
			var h models.HeatPoint
			if err := rows.Scan(&h.Lat, &h.Lon, &h.Weight); err != nil {
				continue
			}
			result = append(result, h)
		}
		c.JSON(http.StatusOK, result)
	}
}

func GetBusiestStations(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
		if limit > 50 {
			limit = 50
		}

		rows, err := pool.Query(c.Request.Context(), `
			SELECT s.station_id, s.name, COUNT(t.id) AS trip_count
			FROM stations s
			LEFT JOIN trips t ON (t.start_station_id = s.station_id OR t.end_station_id = s.station_id)
				AND t.start_time >= NOW() - INTERVAL '7 days'
				AND NOT (
				    t.start_station_id IS NOT DISTINCT FROM t.end_station_id
				    AND t.end_time - t.start_time < INTERVAL '10 minutes'
				    AND COALESCE(t.distance_meters, 0) < 200
				)
			GROUP BY s.station_id, s.name
			ORDER BY trip_count DESC
			LIMIT $1
		`, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.BusiestStation, 0, limit)
		for rows.Next() {
			var b models.BusiestStation
			if err := rows.Scan(&b.StationID, &b.Name, &b.TripCount); err != nil {
				continue
			}
			result = append(result, b)
		}
		c.JSON(http.StatusOK, result)
	}
}
