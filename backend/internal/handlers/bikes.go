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
		var (
			rangeMeters  *int
			lat, lon     *float64
			stationID    *string
			stationName  *string
			isDisabled   *bool
			snapshotTime *time.Time
		)
		err := pool.QueryRow(c.Request.Context(), `
			SELECT b.bike_id, b.vehicle_type_id, b.first_seen, b.last_seen, b.physical_bike_id,
				s.current_range_meters, s.lat, s.lon, s.station_id, s.is_disabled, s.time,
				st.name
			FROM bikes b
			LEFT JOIN LATERAL (
				SELECT time, lat, lon, station_id, is_disabled, current_range_meters
				FROM bike_snapshots
				WHERE bike_id = b.bike_id
				ORDER BY time DESC LIMIT 1
			) s ON true
			LEFT JOIN stations st ON st.station_id = s.station_id
			WHERE b.bike_id = $1
		`, bikeID).Scan(
			&b.BikeID, &b.VehicleTypeID, &b.FirstSeen, &b.LastSeen, &b.PhysicalBikeID,
			&rangeMeters, &lat, &lon, &stationID, &isDisabled, &snapshotTime, &stationName,
		)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "bike not found"})
			return
		}
		if rangeMeters != nil {
			pct := batteryPct(*rangeMeters)
			b.CurrentBatteryPct = &pct
			b.CurrentLat = lat
			b.CurrentLon = lon
			b.CurrentStationID = stationID
			b.CurrentStationName = stationName
			b.IsCurrentlyDisabled = isDisabled
			b.LastSnapshotTime = snapshotTime
		}
		c.JSON(http.StatusOK, b)
	}
}

func GetBikeHistory(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		bikeID := c.Param("id")
		from, to := parseTimeRange(c, 24*time.Hour)
		resolution := c.DefaultQuery("resolution", "raw")

		type pgRows interface {
			Next() bool
			Scan(...any) error
			Close()
		}

		var pgr pgRows
		var err error

		switch resolution {
		case "1h":
			pgr, err = pool.Query(c.Request.Context(), `
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
			pgr, err = pool.Query(c.Request.Context(), `
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
			pgr, err = pool.Query(c.Request.Context(), `
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

		_ = pool.QueryRow(ctx, `
			SELECT COUNT(*), COALESCE(SUM(distance_meters), 0) / 1000.0
			FROM trips WHERE bike_id = $1
			  AND NOT (
			    start_station_id IS NOT DISTINCT FROM end_station_id
			    AND end_time - start_time < INTERVAL '10 minutes'
			    AND COALESCE(distance_meters, 0) < 200
			  )
		`, bikeID).Scan(&stats.TotalTrips, &stats.TotalDistanceKm)

		var avgRange *float64
		_ = pool.QueryRow(ctx, `
			SELECT AVG(current_range_meters) FROM bike_snapshots
			WHERE bike_id = $1 AND time > NOW() - INTERVAL '7 days'
		`, bikeID).Scan(&avgRange)
		if avgRange != nil {
			stats.AvgBatteryPct = batteryPct(int(*avgRange))
		}

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

func GetBikeHealth(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		bikeID := c.Param("id")
		ctx := c.Request.Context()

		var h models.BikeHealth
		h.BikeID = bikeID

		// Average battery last 30 days
		var avgRange *float64
		_ = pool.QueryRow(ctx, `
			SELECT AVG(current_range_meters)
			FROM bike_snapshots
			WHERE bike_id = $1 AND time > NOW() - INTERVAL '30 days'
		`, bikeID).Scan(&avgRange)
		if avgRange != nil {
			h.AvgBatteryPct = batteryPct(int(*avgRange))
		}

		// Disabled incidents (distinct days with is_disabled=true) last 30 days
		_ = pool.QueryRow(ctx, `
			SELECT COUNT(DISTINCT DATE(time))
			FROM bike_snapshots
			WHERE bike_id = $1
			  AND time > NOW() - INTERVAL '30 days'
			  AND is_disabled = TRUE
		`, bikeID).Scan(&h.DisabledCount30d)

		// Trips last 30 days
		_ = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM trips
			WHERE bike_id = $1
			  AND start_time > NOW() - INTERVAL '30 days'
			  AND NOT (
			    start_station_id IS NOT DISTINCT FROM end_station_id
			    AND end_time - start_time < INTERVAL '10 minutes'
			    AND COALESCE(distance_meters, 0) < 200
			  )
		`, bikeID).Scan(&h.Trips30d)

		// Score calculation
		h.BatteryScore = h.AvgBatteryPct * 40 / 100 // max 40 pts
		reliability := 30 - h.DisabledCount30d*3
		if reliability < 0 {
			reliability = 0
		}
		h.ReliabilityScore = reliability
		activity := h.Trips30d
		if activity > 30 {
			activity = 30
		}
		h.ActivityScore = activity
		h.HealthScore = h.BatteryScore + h.ReliabilityScore + h.ActivityScore

		switch {
		case h.HealthScore >= 70:
			h.Label = "Bon"
		case h.HealthScore >= 40:
			h.Label = "Moyen"
		default:
			h.Label = "À réviser"
		}

		c.JSON(http.StatusOK, h)
	}
}

// GetNearestBikes returns the N closest available docked bikes to a lat/lon.
func GetNearestBikes(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		lat, errLat := strconv.ParseFloat(c.Query("lat"), 64)
		lon, errLon := strconv.ParseFloat(c.Query("lon"), 64)
		if errLat != nil || errLon != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "lat and lon required"})
			return
		}
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))

		rows, err := pool.Query(c.Request.Context(), `
			WITH latest AS (
				SELECT DISTINCT ON (bs.bike_id)
					bs.bike_id, b.vehicle_type_id,
					bs.lat, bs.lon, bs.current_range_meters
				FROM bike_snapshots bs
				JOIN bikes b ON b.bike_id = bs.bike_id
				WHERE bs.time > NOW() - INTERVAL '2 minutes'
				  AND bs.is_disabled = FALSE
				  AND bs.is_reserved = FALSE
				  AND bs.station_id IS NOT NULL
				ORDER BY bs.bike_id, bs.time DESC
			)
			SELECT bike_id, vehicle_type_id, lat, lon, current_range_meters,
				SQRT(
					POW((lat - $1) * 111320, 2) +
					POW((lon - $2) * 111320 * COS(RADIANS($1)), 2)
				)::INT AS distance_m
			FROM latest
			ORDER BY distance_m
			LIMIT $3
		`, lat, lon, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.NearestBike, 0, limit)
		for rows.Next() {
			var b models.NearestBike
			if err := rows.Scan(&b.BikeID, &b.VehicleTypeID, &b.Lat, &b.Lon, &b.CurrentRangeMeters, &b.DistanceMeters); err != nil {
				continue
			}
			b.BatteryPct = batteryPct(b.CurrentRangeMeters)
			result = append(result, b)
		}
		c.JSON(http.StatusOK, result)
	}
}

// GetNearestStations returns the N closest stations with available docks to a lat/lon.
func GetNearestStations(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		lat, errLat := strconv.ParseFloat(c.Query("lat"), 64)
		lon, errLon := strconv.ParseFloat(c.Query("lon"), 64)
		if errLat != nil || errLon != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "lat and lon required"})
			return
		}
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))

		rows, err := pool.Query(c.Request.Context(), `
			WITH latest_ss AS (
				SELECT DISTINCT ON (station_id)
					station_id, num_docks_available
				FROM station_snapshots
				WHERE time > NOW() - INTERVAL '2 minutes'
				ORDER BY station_id, time DESC
			)
			SELECT s.station_id, s.name, s.lat, s.lon,
				COALESCE(ss.num_docks_available, 0) AS num_docks_available,
				SQRT(
					POW((s.lat - $1) * 111320, 2) +
					POW((s.lon - $2) * 111320 * COS(RADIANS($1)), 2)
				)::INT AS distance_m
			FROM stations s
			LEFT JOIN latest_ss ss ON ss.station_id = s.station_id
			WHERE COALESCE(ss.num_docks_available, 0) > 0
			ORDER BY distance_m
			LIMIT $3
		`, lat, lon, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.NearestStation, 0, limit)
		for rows.Next() {
			var s models.NearestStation
			if err := rows.Scan(&s.StationID, &s.Name, &s.Lat, &s.Lon, &s.NumDocksAvailable, &s.DistanceMeters); err != nil {
				continue
			}
			result = append(result, s)
		}
		c.JSON(http.StatusOK, result)
	}
}

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
