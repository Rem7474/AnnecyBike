package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// StationPoint is the minimal station data needed to build the routing matrix.
type StationPoint struct {
	ID  string
	Lat float64
	Lon float64
}

// Matrix holds inter-station cycling road distances (metres) fetched from OSRM.
// We store distances rather than OSRM durations so we can apply electric-bike
// speed ourselves (OSRM's bike profile assumes ~15 km/h; Velonecy bikes are
// speed-limited to 25 km/h electric assist).
//
// A nil Matrix means OSRM was unavailable; callers must fall back to haversine.
type Matrix struct {
	idx  map[string]int // station_id → row/col index
	dist [][]float64    // dist[i][j] = road distance in metres (OSRM)
}

// BuildMatrix calls the OSRM /table endpoint once and returns the full N×N
// distance matrix for all provided stations (N=63 for Annecy → 1 HTTP request).
// osrmBase should be a URL like "http://router.project-osrm.org" or a local instance.
func BuildMatrix(ctx context.Context, osrmBase string, stations []StationPoint) (*Matrix, error) {
	if len(stations) == 0 {
		return &Matrix{}, nil
	}

	// OSRM coordinate order: lon,lat
	coords := make([]string, len(stations))
	ids := make([]string, len(stations))
	for i, s := range stations {
		coords[i] = fmt.Sprintf("%.6f,%.6f", s.Lon, s.Lat)
		ids[i] = s.ID
	}

	url := fmt.Sprintf(
		"%s/table/v1/bike/%s?annotations=distance",
		strings.TrimRight(osrmBase, "/"),
		strings.Join(coords, ";"),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build OSRM request: %w", err)
	}
	req.Header.Set("User-Agent", "AnnecyBike-Poller/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OSRM table request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code      string      `json:"code"`
		Distances [][]float64 `json:"distances"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode OSRM response: %w", err)
	}
	if result.Code != "Ok" {
		return nil, fmt.Errorf("OSRM returned error code: %s", result.Code)
	}

	idx := make(map[string]int, len(ids))
	for i, id := range ids {
		idx[id] = i
	}
	return &Matrix{idx: idx, dist: result.Distances}, nil
}

// TravelWindow returns the (min, max, expected) travel durations between two
// stations for a Velonecy electric bike (25 km/h assist, speed-limited).
//
// Speed assumptions for urban Annecy:
//   - 22 km/h  fastest realistic (few stops, direct route)
//   - 18 km/h  expected average  (traffic lights, brief stops)
//   - 10 km/h  slowest + ×1.5 buffer (very casual rider, long waits)
//
// Returns ok=false when either station is absent from the matrix, signalling
// the caller should fall back to haversine-based estimation.
func (m *Matrix) TravelWindow(fromID, toID string) (min, max, expected time.Duration, ok bool) {
	if m == nil || len(m.dist) == 0 {
		return 0, 0, 0, false
	}
	i, ok1 := m.idx[fromID]
	j, ok2 := m.idx[toID]
	if !ok1 || !ok2 || m.dist[i][j] <= 0 {
		return 0, 0, 0, false
	}

	dist := m.dist[i][j]
	sec := func(kmh float64) time.Duration {
		return time.Duration(dist/(kmh/3.6)) * time.Second
	}

	min = sec(22)
	if min < 60*time.Second {
		min = 60 * time.Second
	}
	expected = sec(18)
	max = time.Duration(float64(sec(10)) * 1.5)
	if max < 5*time.Minute {
		max = 5 * time.Minute
	}
	return min, max, expected, true
}

// RoadDistance returns the OSRM road distance in metres between two stations,
// or -1 if either station is not in the matrix.
func (m *Matrix) RoadDistance(fromID, toID string) float64 {
	if m == nil || len(m.dist) == 0 {
		return -1
	}
	i, ok1 := m.idx[fromID]
	j, ok2 := m.idx[toID]
	if !ok1 || !ok2 {
		return -1
	}
	return m.dist[i][j]
}
