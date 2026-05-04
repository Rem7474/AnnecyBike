package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/annecybike/backend/internal/db"
	"github.com/annecybike/backend/internal/models"
	"github.com/gin-gonic/gin"
)

func GetPhysicalBikes(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := pool.Query(c.Request.Context(), `
			WITH current_state AS (
				SELECT DISTINCT ON (b.physical_bike_id)
					b.physical_bike_id,
					b.bike_id AS current_bike_id,
					st.name   AS current_station_name
				FROM bikes b
				LEFT JOIN LATERAL (
					SELECT station_id FROM bike_snapshots
					WHERE bike_id = b.bike_id
					ORDER BY time DESC LIMIT 1
				) bs ON true
				LEFT JOIN stations st ON st.station_id = bs.station_id
				WHERE b.physical_bike_id IS NOT NULL
				ORDER BY b.physical_bike_id, b.last_seen DESC
			)
			SELECT
				pb.id,
				pb.vehicle_type_id,
				pb.fleet_number,
				pb.custom_name,
				pb.first_seen,
				pb.last_seen,
				COUNT(DISTINCT t.id)                          AS total_trips,
				COALESCE(SUM(t.distance_meters), 0) / 1000.0 AS total_distance_km,
				COUNT(DISTINCT bk.bike_id)                    AS bike_id_count,
				cs.current_bike_id,
				cs.current_station_name
			FROM physical_bikes pb
			LEFT JOIN trips t         ON t.physical_bike_id  = pb.id
			LEFT JOIN bikes bk        ON bk.physical_bike_id = pb.id
			LEFT JOIN current_state cs ON cs.physical_bike_id = pb.id
			GROUP BY pb.id, pb.vehicle_type_id, pb.fleet_number, pb.custom_name,
			         pb.first_seen, pb.last_seen, cs.current_bike_id, cs.current_station_name
			ORDER BY pb.last_seen DESC
			LIMIT 500
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		bikes := make([]models.PhysicalBike, 0, 200)
		for rows.Next() {
			var b models.PhysicalBike
			var idCount int
			if err := rows.Scan(&b.ID, &b.VehicleTypeID, &b.FleetNumber, &b.CustomName,
				&b.FirstSeen, &b.LastSeen, &b.TotalTrips, &b.TotalDistanceKm, &idCount,
				&b.CurrentBikeID, &b.CurrentStationName); err != nil {
				continue
			}
			b.IDCount = &idCount
			bikes = append(bikes, b)
		}
		c.JSON(http.StatusOK, bikes)
	}
}

func GetPhysicalBike(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		ctx := c.Request.Context()

		var b models.PhysicalBikeDetail
		var totalDistM int64
		if err := pool.QueryRow(ctx, `
			SELECT pb.id, pb.vehicle_type_id, pb.fleet_number, pb.custom_name,
				pb.first_seen, pb.last_seen,
				COUNT(t.id), COALESCE(SUM(t.distance_meters), 0)
			FROM physical_bikes pb
			LEFT JOIN trips t ON t.physical_bike_id = pb.id
			WHERE pb.id = $1
			GROUP BY pb.id
		`, pid).Scan(&b.ID, &b.VehicleTypeID, &b.FleetNumber, &b.CustomName,
			&b.FirstSeen, &b.LastSeen, &b.TotalTrips, &totalDistM); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		b.TotalDistanceKm = float64(totalDistM) / 1000.0

		// Current state from most recently active bike_id
		var curBikeID *string
		var curRange *int
		var curDisabled *bool
		var curStationName *string
		_ = pool.QueryRow(ctx, `
			SELECT b.bike_id, s.current_range_meters, s.is_disabled, st.name
			FROM bikes b
			LEFT JOIN LATERAL (
				SELECT current_range_meters, is_disabled, station_id
				FROM bike_snapshots
				WHERE bike_id = b.bike_id
				ORDER BY time DESC LIMIT 1
			) s ON true
			LEFT JOIN stations st ON st.station_id = s.station_id
			WHERE b.physical_bike_id = $1
			ORDER BY b.last_seen DESC
			LIMIT 1
		`, pid).Scan(&curBikeID, &curRange, &curDisabled, &curStationName)

		b.CurrentBikeID = curBikeID
		if curRange != nil {
			pct := batteryPct(*curRange)
			b.CurrentBatteryPct = &pct
		}
		b.IsCurrentlyDisabled = curDisabled
		b.CurrentStationName = curStationName

		// All known bike_ids
		idRows, err := pool.Query(ctx, `
			SELECT bike_id FROM bikes WHERE physical_bike_id = $1 ORDER BY first_seen ASC
		`, pid)
		if err == nil {
			defer idRows.Close()
			for idRows.Next() {
				var id string
				if err := idRows.Scan(&id); err == nil {
					b.KnownBikeIDs = append(b.KnownBikeIDs, id)
				}
			}
		}

		c.JSON(http.StatusOK, b)
	}
}

// UpdatePhysicalBike handles PATCH /physical-bikes/:pid
// Accepts fleet_number and/or custom_name; pass null to clear.
func UpdatePhysicalBike(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var req models.UpdatePhysicalBikeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		_, err = pool.Exec(c.Request.Context(), `
			UPDATE physical_bikes
			SET fleet_number = $2, custom_name = $3
			WHERE id = $1
		`, pid, req.FleetNumber, req.CustomName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// ReassignBike handles PATCH /bikes/:id/reassign
// Moves the bike_id (and all its trips) to a different physical_bike_id.
func ReassignBike(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		bikeID := c.Param("id")
		var req models.ReassignBikeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx := c.Request.Context()

		// Verify target physical bike exists
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM physical_bikes WHERE id = $1)`, req.PhysicalBikeID,
		).Scan(&exists); err != nil || !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "target physical bike not found"})
			return
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer tx.Rollback(ctx)

		// Reassign the bike_id session
		if _, err := tx.Exec(ctx,
			`UPDATE bikes SET physical_bike_id = $2 WHERE bike_id = $1`,
			bikeID, req.PhysicalBikeID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Reassign all trips recorded under this bike_id
		if _, err := tx.Exec(ctx,
			`UPDATE trips SET physical_bike_id = $2 WHERE bike_id = $1`,
			bikeID, req.PhysicalBikeID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Bump last_seen on target physical bike
		if _, err := tx.Exec(ctx,
			`UPDATE physical_bikes SET last_seen = NOW() WHERE id = $1`, req.PhysicalBikeID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := tx.Commit(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// AssignBike handles POST /bikes/:id/assign
// Finds or creates a physical bike by fleet_number and links the bike_id to it.
func AssignBike(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		bikeID := c.Param("id")
		var req struct {
			FleetNumber string `json:"fleet_number"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.FleetNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "fleet_number required"})
			return
		}
		ctx := c.Request.Context()

		// Find or create physical bike by fleet_number
		var physicalID int64
		err := pool.QueryRow(ctx,
			`SELECT id FROM physical_bikes WHERE fleet_number = $1`, req.FleetNumber,
		).Scan(&physicalID)
		if err != nil {
			// Create from the bike's own metadata
			if err2 := pool.QueryRow(ctx, `
				INSERT INTO physical_bikes (vehicle_type_id, fleet_number, first_seen, last_seen)
				SELECT vehicle_type_id, $2, first_seen, last_seen FROM bikes WHERE bike_id = $1
				RETURNING id
			`, bikeID, req.FleetNumber).Scan(&physicalID); err2 != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err2.Error()})
				return
			}
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer tx.Rollback(ctx)

		if _, err := tx.Exec(ctx,
			`UPDATE bikes SET physical_bike_id = $2 WHERE bike_id = $1`, bikeID, physicalID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if _, err := tx.Exec(ctx,
			`UPDATE trips SET physical_bike_id = $2 WHERE bike_id = $1`, bikeID, physicalID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if _, err := tx.Exec(ctx,
			`UPDATE physical_bikes SET last_seen = NOW() WHERE id = $1`, physicalID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := tx.Commit(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "physical_bike_id": physicalID})
	}
}

func GetPhysicalBikeTrips(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
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
			WHERE t.physical_bike_id = $1
			ORDER BY t.start_time DESC
			LIMIT $2 OFFSET $3
		`, pid, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		trips := make([]models.Trip, 0, limit)
		for rows.Next() {
			t, err := scanTrip(rows)
			if err != nil {
				continue
			}
			trips = append(trips, t)
		}
		c.JSON(http.StatusOK, trips)
	}
}

func GetPhysicalBikeHistory(pool *db.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		from, to := parseTimeRange(c, 7*24*time.Hour)

		rows, err := pool.Query(c.Request.Context(), `
			SELECT bs.time, bs.bike_id, bs.lat, bs.lon, bs.station_id, bs.is_disabled, bs.current_range_meters
			FROM bike_snapshots bs
			WHERE bs.bike_id IN (SELECT bike_id FROM bikes WHERE physical_bike_id = $1)
			  AND bs.time BETWEEN $2 AND $3
			ORDER BY bs.time DESC
			LIMIT 5000
		`, pid, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		result := make([]models.BikeSnapshot, 0)
		for rows.Next() {
			var s models.BikeSnapshot
			if err := rows.Scan(&s.Time, &s.BikeID, &s.Lat, &s.Lon, &s.StationID, &s.IsDisabled, &s.CurrentRangeMeters); err != nil {
				continue
			}
			result = append(result, s)
		}
		c.JSON(http.StatusOK, result)
	}
}
