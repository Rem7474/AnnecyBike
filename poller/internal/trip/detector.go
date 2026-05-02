package trip

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/annecybike/poller/internal/db"
)

type BikeState struct {
	StationID  *string
	Lat        float64
	Lon        float64
	Battery    int
	IsDisabled bool
	SeenAt     time.Time
}

type openTrip struct {
	startTime      time.Time
	startStationID *string
	startLat       float64
	startLon       float64
	batteryStart   int
}

// Detector maintains per-bike state across poll cycles and derives trips.
type Detector struct {
	lastState map[string]BikeState
	openTrips map[string]openTrip
	db        *db.Pool
}

func NewDetector(pool *db.Pool) *Detector {
	return &Detector{
		lastState: make(map[string]BikeState),
		openTrips: make(map[string]openTrip),
		db:        pool,
	}
}

// HydrateState loads the most recent known state per bike from the DB so that
// trip detection survives a poller restart without losing in-flight trips.
func (d *Detector) HydrateState(ctx context.Context) error {
	rows, err := d.db.FetchLatestBikeStates(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		d.lastState[row.BikeID] = BikeState{
			StationID:  row.StationID,
			Lat:        row.Lat,
			Lon:        row.Lon,
			Battery:    row.RangeMeters,
			IsDisabled: row.IsDisabled,
			SeenAt:     row.SeenAt,
		}
	}
	slog.Info("detector state hydrated", "bikes", len(rows))
	return nil
}

// Process compares the current poll snapshot against the previous one
// and detects trip starts/ends.
func (d *Detector) Process(ctx context.Context, now time.Time, current map[string]BikeState) {
	for bikeID, cur := range current {
		prev, seen := d.lastState[bikeID]
		if !seen {
			d.lastState[bikeID] = cur
			continue
		}

		prevDocked := prev.StationID != nil
		curDocked := cur.StationID != nil

		switch {
		case prevDocked && !curDocked:
			// Departure: was at station, now free-floating.
			// Note: IsDisabled is intentionally not filtered — a bike can be marked
			// disabled mid-rental (low battery, ops flag) and still needs trip tracking.
			d.openTrips[bikeID] = openTrip{
				startTime:      now,
				startStationID: prev.StationID,
				startLat:       prev.Lat,
				startLon:       prev.Lon,
				batteryStart:   prev.Battery,
			}
			slog.Info("trip started", "bike", bikeID, "from_station", stationStr(prev.StationID))

		case !prevDocked && curDocked:
			// Arrival: was free-floating, now at station.
			if ot, ok := d.openTrips[bikeID]; ok {
				d.closeTrip(ctx, bikeID, ot, now, cur)
				delete(d.openTrips, bikeID)
			}

		case prevDocked && curDocked && !sameStation(prev.StationID, cur.StationID):
			// Bike moved from one station to another within a single poll window
			// (short trip not captured as free-floating). Use poll boundaries as
			// the best available time estimate.
			ot := openTrip{
				startTime:      prev.SeenAt,
				startStationID: prev.StationID,
				startLat:       prev.Lat,
				startLon:       prev.Lon,
				batteryStart:   prev.Battery,
			}
			d.closeTrip(ctx, bikeID, ot, now, cur)
		}

		d.lastState[bikeID] = cur
	}

	// Handle bikes that disappeared from the feed.
	for bikeID, prev := range d.lastState {
		if _, stillPresent := current[bikeID]; stillPresent {
			continue
		}
		if ot, ok := d.openTrips[bikeID]; ok {
			// Close only when the bike has been absent for >10 min, not based on
			// trip duration — a long ride with a brief GBFS blip must not be cut short.
			if now.Sub(prev.SeenAt) > 30*time.Minute {
				d.closeTrip(ctx, bikeID, ot, now, prev)
				delete(d.openTrips, bikeID)
			}
		}
		if now.Sub(prev.SeenAt) > 15*time.Minute {
			// Safety: close any leaked open trip before discarding state.
			if ot, ok := d.openTrips[bikeID]; ok {
				d.closeTrip(ctx, bikeID, ot, prev.SeenAt, prev)
				delete(d.openTrips, bikeID)
			}
			delete(d.lastState, bikeID)
		}
	}
}

// gpsPathDistance computes the total estimated cycling distance by walking the
// recorded GPS path: departure station → free-floating snapshots → arrival station.
// A 1.3 detour factor is applied uniformly to account for road curvature.
func (d *Detector) gpsPathDistance(ctx context.Context, bikeID string, startTime, endTime time.Time, startLat, startLon, endLat, endLon float64) int {
	pts, err := d.db.FetchBikePathPoints(ctx, bikeID, startTime, endTime)
	if err != nil {
		pts = nil
	}

	// Full path: departure coord → free-floating GPS points → arrival coord.
	// Skip 0,0 endpoints: some GBFS providers omit lat/lon for docked bikes,
	// which would produce wildly incorrect distances via null-island (Gulf of Guinea).
	path := make([][2]float64, 0, len(pts)+2)
	if startLat != 0 || startLon != 0 {
		path = append(path, [2]float64{startLat, startLon})
	}
	path = append(path, pts...)
	if endLat != 0 || endLon != 0 {
		path = append(path, [2]float64{endLat, endLon})
	}
	if len(path) < 2 {
		return 0
	}
	var total float64
	for i := 1; i < len(path); i++ {
		total += haversineRaw(path[i-1][0], path[i-1][1], path[i][0], path[i][1])
	}
	return int(total * 1.3)
}

// isGhostTrip returns true when the trip is almost certainly a GBFS connectivity
// blip rather than a real ride. Two patterns observed in the wild:
//  1. Bike blinks off a charging station and reappears at the same station within
//     10 min while the battery went up (or stayed equal) → charging artefact.
//  2. Any "trip" shorter than 3 min with under 150 m displacement → noise.
func isGhostTrip(startSID, endSID *string, battStart, battEnd, dist int, duration time.Duration) bool {
	if sameStation(startSID, endSID) && battEnd >= battStart && duration < 10*time.Minute {
		return true
	}
	if dist < 150 && duration < 3*time.Minute {
		return true
	}
	return false
}

func (d *Detector) closeTrip(ctx context.Context, bikeID string, ot openTrip, endTime time.Time, endState BikeState) {
	duration := endTime.Sub(ot.startTime)
	dist := d.gpsPathDistance(ctx, bikeID, ot.startTime, endTime, ot.startLat, ot.startLon, endState.Lat, endState.Lon)

	if isGhostTrip(ot.startStationID, endState.StationID, ot.batteryStart, endState.Battery, dist, duration) {
		slog.Info("ghost trip discarded",
			"bike", bikeID,
			"duration", duration.Round(time.Second),
			"distance_m", dist,
			"same_station", sameStation(ot.startStationID, endState.StationID),
			"battery_delta", endState.Battery-ot.batteryStart,
		)
		return
	}

	err := d.db.InsertTrip(ctx,
		bikeID,
		ot.startTime, endTime,
		ot.startStationID, endState.StationID,
		ot.startLat, ot.startLon,
		endState.Lat, endState.Lon,
		dist, ot.batteryStart, endState.Battery,
	)
	if err != nil {
		slog.Error("insert trip failed", "bike", bikeID, "err", err)
		return
	}
	slog.Info("trip closed",
		"bike", bikeID,
		"duration", duration.Round(time.Second),
		"distance_m", dist,
	)
}

func haversineRaw(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6_371_000.0
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func sameStation(a, b *string) bool {
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func stationStr(s *string) string {
	if s == nil {
		return fmt.Sprintf("%v", nil)
	}
	return *s
}
