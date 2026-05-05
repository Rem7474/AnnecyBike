package trip

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/annecybike/poller/internal/db"
	"github.com/annecybike/poller/internal/routing"
)

type BikeState struct {
	VehicleTypeID string  // needed when creating a new physical_bike
	StationID     *string
	Lat           float64
	Lon           float64
	Battery       int
	IsDisabled    bool
	SeenAt        time.Time
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
	lastState   map[string]BikeState
	openTrips   map[string]openTrip
	physicalIDs map[string]int64 // bike_id → physical_bike_id (in-memory, persisted via DB)
	db          *db.Pool
	matrix      *routing.Matrix // nil until SetMatrix is called; haversine fallback used
}

func NewDetector(pool *db.Pool) *Detector {
	return &Detector{
		lastState:   make(map[string]BikeState),
		openTrips:   make(map[string]openTrip),
		physicalIDs: make(map[string]int64),
		db:          pool,
	}
}

// SetMatrix updates the routing matrix used for travel-time estimation.
// Safe to call at any time between poll cycles.
func (d *Detector) SetMatrix(m *routing.Matrix) { d.matrix = m }

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
	physIDs, err2 := d.db.FetchBikePhysicalIDs(ctx)
	if err2 == nil {
		for bikeID, pid := range physIDs {
			d.physicalIDs[bikeID] = pid
		}
	}
	slog.Info("detector state hydrated", "bikes", len(rows), "physical_ids", len(d.physicalIDs))
	return nil
}

// Process compares the current poll snapshot against the previous one
// and detects trip starts/ends.
//
// GBFS §free_bike_status: vehicles in active rental are REMOVED from the feed,
// and bike_id MUST be rotated after each trip (v2.0+). This means we can reliably
// detect departures (docked bike disappears) but cannot correlate the return to
// the same bike_id. Returns get a best-effort match via tryMatchArrival; unmatched
// trips close after 2 hours with end_station = NULL.
func (d *Detector) Process(ctx context.Context, now time.Time, current map[string]BikeState) {
	for bikeID, cur := range current {
		prev, seen := d.lastState[bikeID]
		if !seen {
			// Unknown bike_id: either a new deployment or a post-trip ID rotation.
			// If it appears directly at a station it is likely a rotated return.
			if cur.StationID != nil && !cur.IsDisabled {
				d.tryMatchArrival(ctx, now, bikeID, cur)
			}
			// Assign physical bike ID if not yet matched
			if _, hasID := d.physicalIDs[bikeID]; !hasID {
				pid, err := d.db.CreatePhysicalBike(ctx, cur.VehicleTypeID)
				if err == nil {
					d.physicalIDs[bikeID] = pid
					_ = d.db.LinkBikeToPhysical(ctx, bikeID, pid)
				} else {
					slog.Warn("create physical bike failed", "bike", bikeID, "err", err)
				}
			}
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
			//
			// Guard: if the elapsed time is physically incompatible with cycling
			// the straight-line distance between the two stations, this is a GBFS
			// GPS artefact (transient wrong station_id) rather than a real trip.
			sinceLastSeen := now.Sub(prev.SeenAt)
			stationDist := haversineRaw(prev.Lat, prev.Lon, cur.Lat, cur.Lon)
			minCycling, _, _ := cyclingWindow(stationDist)
			if sinceLastSeen < minCycling {
				slog.Warn("station teleport skipped (speed violation)",
					"bike", bikeID,
					"from", stationStr(prev.StationID),
					"to", stationStr(cur.StationID),
					"elapsed", sinceLastSeen.Round(time.Second),
					"min_expected", minCycling.Round(time.Second),
				)
				break
			}
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
			}
			// Within 2-hour window: keep state and open trip alive waiting for return.
		} else if prev.StationID != nil && !prev.IsDisabled {
			// Docked, non-disabled bike vanished from feed — per GBFS spec the vehicle
			// is now in active rental. Open the trip immediately; the arrival will either
			// be matched via tryMatchArrival (ID rotation) or closed after 2 h (timeout).
			d.openTrips[bikeID] = openTrip{
				startTime:      prev.SeenAt,
				startStationID: prev.StationID,
				startLat:       prev.Lat,
				startLon:       prev.Lon,
				batteryStart:   prev.Battery,
			}
			slog.Info("trip started (bike left feed)", "bike", bikeID, "from_station", stationStr(prev.StationID))
		}

		if now.Sub(prev.SeenAt) > 2*time.Hour {
			delete(d.lastState, bikeID)
		}
	}
}

// tryMatchArrival attempts to close the best open trip when a new bike_id appears
// at a station — the signature of a post-rental ID rotation (GBFS §free_bike_status v2.0+).
//
// A candidate trip must satisfy the cycling time window: the elapsed time since
// departure must be consistent with traveling the straight-line distance between
// stations at 8–25 km/h (with a ×1.3 route detour factor and a ×1.5 upper buffer
// for stops and slow riders). Among valid candidates the one whose elapsed time is
// closest to 15 km/h average (the most likely speed) is chosen.
// If no candidate falls within the window, the arrival is ignored — it may be a
// freshly-deployed bike or a GBFS echo; the open trips will close via 2-hour timeout.
func (d *Detector) tryMatchArrival(ctx context.Context, now time.Time, newBikeID string, arrived BikeState) {
	var bestKey string
	var bestScore time.Duration

	for key, ot := range d.openTrips {
		if ot.pendingClose != nil {
			continue
		}
		if sameStation(ot.startStationID, arrived.StationID) {
			continue // same-station returns handled by pendingClose path
		}

		// Hard constraint: battery cannot increase during a ride (no in-trip charging).
		// Allow a small tolerance (500 m-range) for sensor noise.
		if arrived.Battery > ot.batteryStart+500 {
			continue
		}

		elapsed := now.Sub(ot.startTime)

		// Use OSRM road distance when both station IDs are known; otherwise fall
		// back to haversine straight-line (×1.3 detour factor) via cyclingWindow.
		var minT, maxT, expectedT time.Duration
		if d.matrix != nil && ot.startStationID != nil && arrived.StationID != nil {
			var ok bool
			minT, maxT, expectedT, ok = d.matrix.TravelWindow(*ot.startStationID, *arrived.StationID)
			if !ok {
				dist := haversineRaw(ot.startLat, ot.startLon, arrived.Lat, arrived.Lon)
				minT, maxT, expectedT = cyclingWindow(dist)
			}
		} else {
			dist := haversineRaw(ot.startLat, ot.startLon, arrived.Lat, arrived.Lon)
			minT, maxT, expectedT = cyclingWindow(dist)
		}

		if elapsed < minT || elapsed > maxT {
			continue // elapsed time incompatible with cycling distance
		}

		// Score = |elapsed − expected|; lower is better.
		score := elapsed - expectedT
		if score < 0 {
			score = -score
		}
		if bestKey == "" || score < bestScore {
			bestKey = key
			bestScore = score
		}
	}

	if bestKey == "" {
		return
	}

	ot := d.openTrips[bestKey]
	slog.Info("trip matched via ID rotation",
		"old_bike", bestKey,
		"new_bike", newBikeID,
		"new_bike_station", stationStr(arrived.StationID),
		"elapsed", now.Sub(ot.startTime).Round(time.Second),
		"score", bestScore.Round(time.Second),
	)

	// Inherit physical bike ID: new bike_id is the same physical bike
	if pid, ok := d.physicalIDs[bestKey]; ok {
		d.physicalIDs[newBikeID] = pid
		_ = d.db.LinkBikeToPhysical(ctx, newBikeID, pid)
	}

	d.closeTrip(ctx, bestKey, ot, now, arrived)
	delete(d.openTrips, bestKey)
}

// cyclingWindow returns the (min, max, expected) travel durations for a given
// straight-line distance, assuming cyclists travel at 8–25 km/h on routes that
// are on average 30 % longer than the straight-line distance.
//
//	min  = distance × 1.3 / 25 km/h  (fastest realistic cyclist, floored at 60 s)
//	max  = distance × 1.3 / 8 km/h × 1.5  (slowest + 50 % buffer, floored at 5 min)
//	expected = distance × 1.3 / 15 km/h  (average urban cycling speed)
func cyclingWindow(distanceM float64) (min, max, expected time.Duration) {
	const (
		routeFactor = 1.3
		fastKmh     = 25.0
		slowKmh     = 8.0
		avgKmh      = 15.0
		bufferFactor = 1.5
	)
	actual := distanceM * routeFactor
	toSec := func(speedKmh float64) time.Duration {
		return time.Duration(actual/(speedKmh/3.6)*float64(time.Second))
	}
	min = toSec(fastKmh)
	if min < 60*time.Second {
		min = 60 * time.Second
	}
	max = time.Duration(float64(toSec(slowKmh)) * bufferFactor)
	if max < 5*time.Minute {
		max = 5 * time.Minute
	}
	expected = toSec(avgKmh)
	return
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

	var physicalID *int64
	if pid, ok := d.physicalIDs[bikeID]; ok {
		physicalID = &pid
	}

	err := d.db.InsertTrip(ctx,
		bikeID,
		physicalID,
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
