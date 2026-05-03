package handlers

import (
	"net/http"

	"github.com/annecybike/backend/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// GetGeofencingZones returns the latest geofencing GeoJSON as-is.
// The frontend renders it as a Leaflet GeoJSON layer.
func GetGeofencingZones(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := pool.FetchGeofencingZones(c.Request.Context())
		if err != nil {
			if err == pgx.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "geofencing zones not yet available"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
	}
}
