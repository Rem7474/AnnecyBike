package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
					bs.bike_id, b.vehicle_type_id, bs.current_range_meters, b.physical_bike_id
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
				LEAST(COALESCE(trp.cnt, 0), 30),
				c.physical_bike_id,
				pb.fleet_number
			FROM current c
			LEFT JOIN bat ON bat.bike_id = c.bike_id
			LEFT JOIN dis ON dis.bike_id = c.bike_id
			LEFT JOIN trp ON trp.bike_id = c.bike_id
			LEFT JOIN physical_bikes pb ON pb.id = c.physical_bike_id
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
			var physicalBikeID *int64
			var fleetNumber *string
			if err := rows.Scan(&bikeID, &vehicleTypeID, &rangeMeters, &avgRange, &disabledDays, &trips30d,
				&physicalBikeID, &fleetNumber); err != nil {
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
				BikeID:         bikeID,
				VehicleTypeID:  vehicleTypeID,
				BatteryPct:     batteryPct(rangeMeters),
				HealthScore:    healthScore,
				HealthLabel:    label,
				PhysicalBikeID: physicalBikeID,
				FleetNumber:    fleetNumber,
			})
		}
		c.JSON(http.StatusOK, result)
	}
}

// GetStationBikeHistory returns each bike's docking visits at a station over a
// configurable window (default 48h, max 7 days). It reconstructs arrival/departure
// times from bike_snapshots using window functions — useful for debugging trip
// detection and understanding station turnover.
func GetStationBikeHistory(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		stationID := c.Param("id")

		hours := 48
		if v, err := strconv.Atoi(c.DefaultQuery("hours", "48")); err == nil && v > 0 && v <= 168 {
			hours = v
		}
		interval := fmt.Sprintf("%d hours", hours)

		rows, err := pool.Query(c.Request.Context(), `
			WITH relevant_bikes AS (
				SELECT DISTINCT bike_id
				FROM bike_snapshots
				WHERE station_id = $1
				  AND time > NOW() - CAST($2 AS INTERVAL)
			),
			-- Extend look-back by 10 minutes to get a valid LAG for bikes already
			-- present at the start of the window (avoids false arrivals).
			snaps AS (
				SELECT bs.bike_id, bs.time, bs.station_id, bs.current_range_meters,
					LAG(bs.station_id) OVER (PARTITION BY bs.bike_id ORDER BY bs.time) AS prev_sid
				FROM bike_snapshots bs
				JOIN relevant_bikes rb ON rb.bike_id = bs.bike_id
				WHERE bs.time > NOW() - CAST($2 AS INTERVAL) - INTERVAL '10 minutes'
			),
			-- Arrival = confirmed transition from a different state into this station.
			-- prev_sid IS NULL means first snapshot in the extended window — still
			-- ambiguous, so we skip those to avoid counting long-parked bikes as arrivals.
			arrivals AS (
				SELECT bike_id, time AS arrived_at, current_range_meters AS battery
				FROM snaps
				WHERE station_id = $1
				  AND time > NOW() - CAST($2 AS INTERVAL)
				  AND prev_sid IS NOT NULL
				  AND prev_sid != $1
			),
			departures AS (
				SELECT bike_id, time AS departed_at
				FROM snaps
				WHERE (station_id IS NULL OR station_id != $1)
				  AND prev_sid = $1
			)
			SELECT
				a.bike_id,
				a.arrived_at,
				d.departed_at,
				a.battery,
				EXTRACT(epoch FROM (COALESCE(d.departed_at, NOW()) - a.arrived_at)) / 60 AS duration_min,
				-- True presence: check the bike's latest snapshot in the whole DB.
				-- If it disappeared from the feed (rented), this returns false.
				COALESCE((
					SELECT bs2.station_id = $1
					FROM bike_snapshots bs2
					WHERE bs2.bike_id = a.bike_id
					ORDER BY bs2.time DESC LIMIT 1
				), false) AS still_present
			FROM arrivals a
			LEFT JOIN LATERAL (
				SELECT departed_at FROM departures d2
				WHERE d2.bike_id = a.bike_id AND d2.departed_at > a.arrived_at
				ORDER BY departed_at ASC LIMIT 1
			) d ON true
			ORDER BY a.arrived_at DESC
			LIMIT 200
		`, stationID, interval)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.StationBikeVisit, 0)
		for rows.Next() {
			var v models.StationBikeVisit
			if err := rows.Scan(
				&v.BikeID, &v.ArrivedAt, &v.DepartedAt,
				&v.BatteryArrival, &v.DurationMinutes, &v.StillPresent,
			); err != nil {
				continue
			}
			result = append(result, v)
		}
		c.JSON(http.StatusOK, result)
	}
}

func GetStationHourlyStats(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		stationID := c.Param("id")
		days, _ := strconv.Atoi(c.DefaultQuery("days", "90"))
		if days <= 0 || days > 365 {
			days = 90
		}

		rows, err := pool.Query(c.Request.Context(), `
			SELECT
				EXTRACT(HOUR FROM time AT TIME ZONE 'Europe/Paris')::int AS hour,
				ROUND(AVG(num_bikes_available)::numeric, 2)::float8 AS avg_all,
				ROUND(COALESCE(AVG(num_bikes_available) FILTER (
					WHERE EXTRACT(DOW FROM time AT TIME ZONE 'Europe/Paris') BETWEEN 1 AND 5
				), 0)::numeric, 2)::float8 AS avg_weekday,
				ROUND(COALESCE(AVG(num_bikes_available) FILTER (
					WHERE EXTRACT(DOW FROM time AT TIME ZONE 'Europe/Paris') IN (0, 6)
				), 0)::numeric, 2)::float8 AS avg_weekend,
				COUNT(*)::int AS sample_count
			FROM station_snapshots
			WHERE station_id = $1
			  AND time > NOW() - INTERVAL '1 day' * $2
			GROUP BY hour
			ORDER BY hour
		`, stationID, days)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.HourlyBikeStats, 0, 24)
		for rows.Next() {
			var h models.HourlyBikeStats
			if err := rows.Scan(&h.Hour, &h.AvgAll, &h.AvgWeekday, &h.AvgWeekend, &h.SampleCount); err != nil {
				continue
			}
			result = append(result, h)
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
