package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/annecybike/backend/internal/db"
	"github.com/annecybike/backend/internal/models"
	"github.com/gin-gonic/gin"
)

func GetStationsLive(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := pool.Query(c.Request.Context(), `
			SELECT
				s.station_id, s.name, s.lat, s.lon, s.capacity,
				s.vehicle_type_capacity, s.is_virtual_station, s.is_charging_station, s.last_updated,
				ss.num_bikes_available, ss.num_docks_available
			FROM stations s
			LEFT JOIN LATERAL (
				SELECT num_bikes_available, num_docks_available
				FROM station_snapshots
				WHERE station_id = s.station_id
				ORDER BY time DESC LIMIT 1
			) ss ON true
			ORDER BY s.name
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		stations := make([]models.Station, 0, 70)
		for rows.Next() {
			var st models.Station
			var vtCapJSON []byte
			if err := rows.Scan(
				&st.StationID, &st.Name, &st.Lat, &st.Lon, &st.Capacity,
				&vtCapJSON, &st.IsVirtualStation, &st.IsChargingStation, &st.LastUpdated,
				&st.NumBikesAvailable, &st.NumDocksAvailable,
			); err != nil {
				continue
			}
			if vtCapJSON != nil {
				_ = json.Unmarshal(vtCapJSON, &st.VehicleTypeCapacity)
			}
			stations = append(stations, st)
		}
		c.JSON(http.StatusOK, stations)
	}
}

func GetStation(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		stationID := c.Param("id")
		var st models.Station
		var vtCapJSON []byte
		err := pool.QueryRow(c.Request.Context(), `
			SELECT station_id, name, lat, lon, capacity,
				vehicle_type_capacity, is_virtual_station, is_charging_station, last_updated
			FROM stations WHERE station_id = $1
		`, stationID).Scan(
			&st.StationID, &st.Name, &st.Lat, &st.Lon, &st.Capacity,
			&vtCapJSON, &st.IsVirtualStation, &st.IsChargingStation, &st.LastUpdated,
		)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "station not found"})
			return
		}
		if vtCapJSON != nil {
			_ = json.Unmarshal(vtCapJSON, &st.VehicleTypeCapacity)
		}
		c.JSON(http.StatusOK, st)
	}
}

func GetStationHistory(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		stationID := c.Param("id")
		from, to := parseTimeRange(c, 24*time.Hour)

		rows, err := pool.Query(c.Request.Context(), `
			SELECT time, num_bikes_available, num_docks_available
			FROM station_snapshots
			WHERE station_id = $1 AND time BETWEEN $2 AND $3
			ORDER BY time DESC
			LIMIT 2000
		`, stationID, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.StationHistory, 0)
		for rows.Next() {
			var h models.StationHistory
			if err := rows.Scan(&h.Time, &h.NumBikesAvailable, &h.NumDocksAvailable); err != nil {
				continue
			}
			result = append(result, h)
		}
		c.JSON(http.StatusOK, result)
	}
}
