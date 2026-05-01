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
	startTime       time.Time
	startStationID  *string
	startLat        float64
	startLon        float64
	batteryStart    int
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

// Process compares the current poll snapshot against the previous one
// and detects trip starts/ends.
func (d *Detector) Process(ctx context.Context, now time.Time, current map[string]BikeState) {
	// Detect departures and arrivals
	for bikeID, cur := range current {
		prev, seen := d.lastState[bikeID]
		if !seen {
			// New bike: register state, no trip action
			d.lastState[bikeID] = cur
			continue
		}

		prevDocked := prev.StationID != nil
		curDocked := cur.StationID != nil

		switch {
		case prevDocked && !curDocked && !cur.IsDisabled:
			// Departure: was at station, now free
			sid := prev.StationID
			d.openTrips[bikeID] = openTrip{
				startTime:      now,
				startStationID: sid,
				startLat:       prev.Lat,
				startLon:       prev.Lon,
				batteryStart:   prev.Battery,
			}
			slog.Info("trip started", "bike", bikeID, "from_station", stationStr(sid))

		case !prevDocked && curDocked:
			// Arrival: was free, now at station
			if ot, ok := d.openTrips[bikeID]; ok {
				d.closeTrip(ctx, bikeID, ot, now, cur)
				delete(d.openTrips, bikeID)
			}
		}

		d.lastState[bikeID] = cur
	}

	// Handle bikes that disappeared from the feed
	for bikeID, prev := range d.lastState {
		if _, stillPresent := current[bikeID]; stillPresent {
			continue
		}
		if ot, ok := d.openTrips[bikeID]; ok {
			// Bike gone with an open trip for > 10 min → close it
			if now.Sub(ot.startTime) > 10*time.Minute {
				d.closeTrip(ctx, bikeID, ot, now, prev)
				delete(d.openTrips, bikeID)
			}
		}
		// Remove from last state if gone for a while
		if now.Sub(prev.SeenAt) > 15*time.Minute {
			delete(d.lastState, bikeID)
		}
	}
}

func (d *Detector) closeTrip(ctx context.Context, bikeID string, ot openTrip, endTime time.Time, endState BikeState) {
	dist := haversine(ot.startLat, ot.startLon, endState.Lat, endState.Lon)
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
		"duration", endTime.Sub(ot.startTime).Round(time.Second),
		"distance_m", dist,
	)
}

// haversine returns the straight-line distance in meters × 1.3 (routing correction).
func haversine(lat1, lon1, lat2, lon2 float64) int {
	const R = 6_371_000.0 // Earth radius in meters
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return int(R * c * 1.3)
}

func stationStr(s *string) string {
	if s == nil {
		return fmt.Sprintf("%v", nil)
	}
	return *s
}
