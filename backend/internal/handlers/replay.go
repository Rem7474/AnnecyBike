package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/annecybike/backend/internal/db"
	"github.com/annecybike/backend/internal/models"
	"github.com/gin-gonic/gin"
)

// GetReplay returns bucketed bike positions for a given day at a given resolution.
// Query params:
//   date       = YYYY-MM-DD (default: today)
//   resolution = minutes per bucket (default: 10, min: 5, max: 60)
func GetReplay(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		dateStr := c.DefaultQuery("date", time.Now().UTC().Format("2006-01-02"))
		day, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date, use YYYY-MM-DD"})
			return
		}
		day = day.UTC()
		nextDay := day.Add(24 * time.Hour)

		resolution, _ := strconv.Atoi(c.DefaultQuery("resolution", "10"))
		if resolution < 5 {
			resolution = 5
		}
		if resolution > 60 {
			resolution = 60
		}
		interval := strconv.Itoa(resolution) + " minutes"

		rows, err := pool.Query(c.Request.Context(), `
			SELECT
				time_bucket($1::interval, time) AS bucket,
				bike_id,
				AVG(lat)::float8            AS lat,
				AVG(lon)::float8            AS lon,
				(array_agg(station_id ORDER BY time DESC))[1] AS station_id
			FROM bike_snapshots
			WHERE time >= $2 AND time < $3
			GROUP BY 1, 2
			ORDER BY 1, 2
		`, interval, day, nextDay)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		// Group by bucket
		bucketMap := make(map[time.Time]*models.ReplayBucket)
		var bucketOrder []time.Time

		for rows.Next() {
			var bucket time.Time
			var snap models.ReplayBike
			if err := rows.Scan(&bucket, &snap.BikeID, &snap.Lat, &snap.Lon, &snap.StationID); err != nil {
				continue
			}
			if _, ok := bucketMap[bucket]; !ok {
				bucketMap[bucket] = &models.ReplayBucket{Time: bucket, Snapshots: []models.ReplayBike{}}
				bucketOrder = append(bucketOrder, bucket)
			}
			bucketMap[bucket].Snapshots = append(bucketMap[bucket].Snapshots, snap)
		}

		result := make([]models.ReplayBucket, 0, len(bucketOrder))
		for _, t := range bucketOrder {
			result = append(result, *bucketMap[t])
		}
		c.JSON(http.StatusOK, result)
	}
}
