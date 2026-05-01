package handlers

import (
	"net/http"

	"github.com/annecybike/backend/internal/db"
	"github.com/annecybike/backend/internal/models"
	"github.com/gin-gonic/gin"
)

// GetAnomalies returns bikes that are outside a station, not disabled,
// and haven't moved in the last 24 hours (likely abandoned).
func GetAnomalies(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := pool.Query(c.Request.Context(), `
			WITH latest AS (
				SELECT DISTINCT ON (bs.bike_id)
					bs.bike_id, b.vehicle_type_id,
					bs.lat, bs.lon, bs.time AS last_seen,
					bs.station_id, bs.is_disabled
				FROM bike_snapshots bs
				JOIN bikes b ON b.bike_id = bs.bike_id
				WHERE bs.time > NOW() - INTERVAL '2 minutes'
				ORDER BY bs.bike_id, bs.time DESC
			),
			old_pos AS (
				SELECT DISTINCT ON (bike_id)
					bike_id, lat, lon
				FROM bike_snapshots
				WHERE time BETWEEN NOW() - INTERVAL '25 hours' AND NOW() - INTERVAL '23 hours'
				ORDER BY bike_id, time DESC
			)
			SELECT
				l.bike_id,
				l.vehicle_type_id,
				l.lat, l.lon,
				l.last_seen,
				EXTRACT(EPOCH FROM (NOW() - l.last_seen)) / 3600 AS hours_outside
			FROM latest l
			JOIN old_pos o ON o.bike_id = l.bike_id
			WHERE l.station_id IS NULL
			  AND l.is_disabled = FALSE
			  AND ABS(l.lat - o.lat) < 0.0002
			  AND ABS(l.lon - o.lon) < 0.0002
			ORDER BY hours_outside DESC
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.Anomaly, 0)
		for rows.Next() {
			var a models.Anomaly
			if err := rows.Scan(&a.BikeID, &a.VehicleTypeID, &a.Lat, &a.Lon, &a.LastSeen, &a.HoursOutside); err != nil {
				continue
			}
			result = append(result, a)
		}
		c.JSON(http.StatusOK, result)
	}
}
