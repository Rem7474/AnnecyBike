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
			SELECT s.station_id, s.name, s.lat, s.lon, s.capacity,
				s.vehicle_type_capacity, s.is_virtual_station, s.is_charging_station, s.last_updated,
				ss.num_bikes_available, ss.num_docks_available
			FROM stations s
			LEFT JOIN LATERAL (
				SELECT num_bikes_available, num_docks_available
				FROM station_snapshots
				WHERE station_id = s.station_id
				ORDER BY time DESC LIMIT 1
			) ss ON true
			WHERE s.station_id = $1
		`, stationID).Scan(
			&st.StationID, &st.Name, &st.Lat, &st.Lon, &st.Capacity,
			&vtCapJSON, &st.IsVirtualStation, &st.IsChargingStation, &st.LastUpdated,
			&st.NumBikesAvailable, &st.NumDocksAvailable,
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

// GetStationBikes returns the bikes currently docked at a station with their
// battery level and a health score computed from the last 30 days of data.
func GetStationBikes(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		stationID := c.Param("id")
		ctx := c.Request.Context()

		rows, err := pool.Query(ctx, `
			WITH current AS (
				SELECT DISTINCT ON (bs.bike_id)
					bs.bike_id, b.vehicle_type_id, bs.current_range_meters
				FROM bike_snapshots bs
				JOIN bikes b ON b.bike_id = bs.bike_id
				WHERE bs.station_id = $1
				  AND bs.time > NOW() - INTERVAL '2 minutes'
				ORDER BY bs.bike_id, bs.time DESC
			),
			bat AS (
				SELECT bs.bike_id, AVG(bs.current_range_meters)::INT AS avg_range
				FROM bike_snapshots bs
				JOIN current c ON c.bike_id = bs.bike_id
				WHERE bs.time > NOW() - INTERVAL '30 days'
				GROUP BY bs.bike_id
			),
			dis AS (
				SELECT bs.bike_id, COUNT(DISTINCT DATE(bs.time)) AS cnt
				FROM bike_snapshots bs
				JOIN current c ON c.bike_id = bs.bike_id
				WHERE bs.is_disabled = TRUE
				  AND bs.time > NOW() - INTERVAL '30 days'
				GROUP BY bs.bike_id
			),
			trp AS (
				SELECT t.bike_id, COUNT(*) AS cnt
				FROM trips t
				JOIN current c ON c.bike_id = t.bike_id
				WHERE t.start_time > NOW() - INTERVAL '30 days'
				  AND NOT (
				    t.start_station_id IS NOT DISTINCT FROM t.end_station_id
				    AND t.end_time - t.start_time < INTERVAL '10 minutes'
				    AND COALESCE(t.distance_meters, 0) < 200
				  )
				GROUP BY t.bike_id
			)
			SELECT
				c.bike_id, c.vehicle_type_id, c.current_range_meters,
				COALESCE(bat.avg_range, 0),
				COALESCE(dis.cnt, 0),
				LEAST(COALESCE(trp.cnt, 0), 30)
			FROM current c
			LEFT JOIN bat ON bat.bike_id = c.bike_id
			LEFT JOIN dis ON dis.bike_id = c.bike_id
			LEFT JOIN trp ON trp.bike_id = c.bike_id
			ORDER BY c.bike_id
		`, stationID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.StationBike, 0)
		for rows.Next() {
			var bikeID, vehicleTypeID string
			var rangeMeters, avgRange, disabledDays, trips30d int
			if err := rows.Scan(&bikeID, &vehicleTypeID, &rangeMeters, &avgRange, &disabledDays, &trips30d); err != nil {
				continue
			}
			avgBatteryPct := batteryPct(avgRange)
			batteryScore := avgBatteryPct * 40 / 100
			reliability := 30 - disabledDays*3
			if reliability < 0 {
				reliability = 0
			}
			healthScore := batteryScore + reliability + trips30d
			var label string
			switch {
			case healthScore >= 70:
				label = "Bon"
			case healthScore >= 40:
				label = "Moyen"
			default:
				label = "À réviser"
			}
			result = append(result, models.StationBike{
				BikeID:        bikeID,
				VehicleTypeID: vehicleTypeID,
				BatteryPct:    batteryPct(rangeMeters),
				HealthScore:   healthScore,
				HealthLabel:   label,
			})
		}
		c.JSON(http.StatusOK, result)
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
