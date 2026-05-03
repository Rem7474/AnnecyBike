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

// pendingClose holds an arrival observation that is buffered for one poll.
// GBFS feeds often return the old (departure) station for one cycle before
// updating to the real destination, producing false A→A trips. We wait one
// more observation before committing the close.
type pendingClose struct {
	endTime  time.Time
	endState BikeState
}

type openTrip struct {
	startTime      time.Time
	startStationID *string
	startLat       float64
	startLon       float64
	batteryStart   int
	// Non-nil when the bike arrived at its departure station last poll.
	// Resolved on the next observation.
	pendingClose *pendingClose
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

		// ── Resolve a buffered same-station arrival from the previous poll ──
		// We deferred the close because GBFS sometimes briefly echoes the departure
		// station before updating to the real destination (A→""→A→B pattern).
		if ot, ok := d.openTrips[bikeID]; ok && ot.pendingClose != nil {
			pc := ot.pendingClose
			ot.pendingClose = nil
			switch {
			case cur.StationID != nil && sameStation(prev.StationID, cur.StationID):
				// Confirmed: bike is still at the same station — genuine A→A arrival.
				d.closeTrip(ctx, bikeID, ot, pc.endTime, pc.endState)
				delete(d.openTrips, bikeID)
			case cur.StationID != nil:
				// Bike moved on to a different station: real destination was B, not A.
				slog.Info("pending arrival redirected",
					"bike", bikeID,
					"from", stationStr(ot.startStationID),
					"to", stationStr(cur.StationID),
				)
				d.closeTrip(ctx, bikeID, ot, now, cur)
				delete(d.openTrips, bikeID)
			default:
				// Bike went free-floating again — cancel the tentative close.
				d.openTrips[bikeID] = ot
			}
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
			// Arrival: was free-floating, now at a station.
			if ot, ok := d.openTrips[bikeID]; ok {
				if sameStation(ot.startStationID, cur.StationID) {
					// Arrived at the departure station — buffer for one more poll.
					// GBFS update lag can make the real destination look like A briefly.
					ot.pendingClose = &pendingClose{endTime: now, endState: cur}
					d.openTrips[bikeID] = ot
				} else {
					d.closeTrip(ctx, bikeID, ot, now, cur)
					delete(d.openTrips, bikeID)
				}
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
	// Keep state alive for up to 2 hours so the teleport case can fire when the
	// bike returns (covers rentals where the provider removes the bike from the
	// GBFS feed while it is in use).
	for bikeID, prev := range d.lastState {
		if _, stillPresent := current[bikeID]; stillPresent {
			continue
		}

		if ot, ok := d.openTrips[bikeID]; ok {
			if ot.pendingClose != nil {
				// Bike was at its departure station and has now left the feed.
				// Treat as confirmed arrival (it parked, then the GBFS dropped it).
				d.closeTrip(ctx, bikeID, ot, ot.pendingClose.endTime, ot.pendingClose.endState)
				delete(d.openTrips, bikeID)
				continue
			}
			if now.Sub(prev.SeenAt) > 2*time.Hour {
				d.closeTrip(ctx, bikeID, ot, prev.SeenAt, prev)
				delete(d.openTrips, bikeID)
			} else {
				continue // within 2-hour window — keep state and open trip alive
			}
		}

		if now.Sub(prev.SeenAt) > 2*time.Hour {
			delete(d.lastState, bikeID)
		}
	}
}

// gpsPathDistance computes the total estimated cycling distance by walking the
// recorded GPS path: departure station → free-floating snapshots → arrival station.
// A 1.3 detour factor is applied uniformly to account for road curvature.
//
// Many GBFS providers (including Velonecy) only update bike GPS inside geofenced
// station areas. Free-floating snapshots therefore often carry lat=0,lon=0 or the
// stale departure-station coordinates. Both cases are handled:
//   - 0,0 intermediate points are skipped (null-island avoidance).
//   - 0,0 endpoints fall back to station coordinates fetched from the DB.
func (d *Detector) gpsPathDistance(ctx context.Context, bikeID string, startTime, endTime time.Time, startLat, startLon float64, startSID *string, endLat, endLon float64, endSID *string) int {
	// Resolve 0,0 endpoints using reliable station coordinates.
	if startLat == 0 && startLon == 0 && startSID != nil {
		if lat, lon, err := d.db.FetchStationCoords(ctx, *startSID); err == nil {
			startLat, startLon = lat, lon
		}
	}
	if endLat == 0 && endLon == 0 && endSID != nil {
		if lat, lon, err := d.db.FetchStationCoords(ctx, *endSID); err == nil {
			endLat, endLon = lat, lon
		}
	}

	pts, err := d.db.FetchBikePathPoints(ctx, bikeID, startTime, endTime)
	if err != nil {
		pts = nil
	}

	// Build full path: departure coord → valid free-floating GPS points → arrival coord.
	// Skip any point at (0,0): providers that don't track GPS in motion emit null-island.
	path := make([][2]float64, 0, len(pts)+2)
	if startLat != 0 || startLon != 0 {
		path = append(path, [2]float64{startLat, startLon})
	}
	for _, pt := range pts {
		if pt[0] != 0 || pt[1] != 0 {
			path = append(path, pt)
		}
	}
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
//  2. Very short movement with no clear station-to-station transition → GBFS noise.
//     This condition is intentionally NOT applied when start and end are two
//     distinct known stations: the teleport case (bike skipped the free-floating
//     phase) always produces a short computed duration (one poll boundary) and
//     possibly zero distance when the provider omits coordinates for docked bikes.
//     Filtering those would silently discard real trips.
func isGhostTrip(startSID, endSID *string, battStart, battEnd, dist int, duration time.Duration) bool {
	if sameStation(startSID, endSID) && battEnd >= battStart && duration < 10*time.Minute {
		return true
	}
	if dist < 150 && duration < 3*time.Minute && !differentStations(startSID, endSID) {
		return true
	}
	return false
}

// differentStations returns true only when both IDs are non-nil and unequal —
// i.e., the bike provably moved between two distinct known stations.
func differentStations(a, b *string) bool {
	return a != nil && b != nil && *a != *b
}

func (d *Detector) closeTrip(ctx context.Context, bikeID string, ot openTrip, endTime time.Time, endState BikeState) {
	duration := endTime.Sub(ot.startTime)
	dist := d.gpsPathDistance(ctx, bikeID, ot.startTime, endTime, ot.startLat, ot.startLon, ot.startStationID, endState.Lat, endState.Lon, endState.StationID)

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
