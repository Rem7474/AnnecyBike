package models

import "time"

type VehicleType struct {
	ID             string `json:"vehicle_type_id"`
	Name           string `json:"name"`
	FormFactor     string `json:"form_factor"`
	PropulsionType string `json:"propulsion_type"`
	MaxRangeMeters int    `json:"max_range_meters"`
}

type Station struct {
	StationID           string         `json:"station_id"`
	Name                string         `json:"name"`
	Lat                 float64        `json:"lat"`
	Lon                 float64        `json:"lon"`
	Capacity            int            `json:"capacity"`
	VehicleTypeCapacity map[string]int `json:"vehicle_type_capacity,omitempty"`
	IsVirtualStation    bool           `json:"is_virtual_station"`
	IsChargingStation   bool           `json:"is_charging_station"`
	LastUpdated         time.Time      `json:"last_updated"`
	NumBikesAvailable   *int           `json:"num_bikes_available,omitempty"`
	NumDocksAvailable   *int           `json:"num_docks_available,omitempty"`
}

type StationHistory struct {
	Time              time.Time `json:"time"`
	NumBikesAvailable int       `json:"num_bikes_available"`
	NumDocksAvailable int       `json:"num_docks_available"`
}

type Bike struct {
	BikeID        string    `json:"bike_id"`
	VehicleTypeID string    `json:"vehicle_type_id"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
}

type BikeSnapshot struct {
	Time               time.Time `json:"time"`
	BikeID             string    `json:"bike_id"`
	Lat                float64   `json:"lat"`
	Lon                float64   `json:"lon"`
	StationID          *string   `json:"station_id"`
	IsReserved         bool      `json:"is_reserved"`
	IsDisabled         bool      `json:"is_disabled"`
	CurrentRangeMeters int       `json:"current_range_meters"`
}

type BikeLive struct {
	BikeID             string  `json:"bike_id"`
	VehicleTypeID      string  `json:"vehicle_type_id"`
	Lat                float64 `json:"lat"`
	Lon                float64 `json:"lon"`
	StationID          *string `json:"station_id"`
	IsReserved         bool    `json:"is_reserved"`
	IsDisabled         bool    `json:"is_disabled"`
	CurrentRangeMeters int     `json:"current_range_meters"`
	BatteryPct         int     `json:"battery_pct"`
}

type BikeStats struct {
	BikeID          string  `json:"bike_id"`
	TotalTrips      int     `json:"total_trips"`
	TotalDistanceKm float64 `json:"total_distance_km"`
	AvgBatteryPct   int     `json:"avg_battery_pct"`
	AvailabilityPct float64 `json:"availability_pct"`
	DisabledPct     float64 `json:"disabled_pct"`
}

type BikeHealth struct {
	BikeID           string `json:"bike_id"`
	HealthScore      int    `json:"health_score"`      // 0-100
	BatteryScore     int    `json:"battery_score"`     // 0-40
	ReliabilityScore int    `json:"reliability_score"` // 0-30
	ActivityScore    int    `json:"activity_score"`    // 0-30
	AvgBatteryPct    int    `json:"avg_battery_pct"`
	DisabledCount30d int    `json:"disabled_count_30d"`
	Trips30d         int    `json:"trips_30d"`
	Label            string `json:"label"` // "Bon", "Moyen", "À réviser"
}

type Trip struct {
	ID               int64    `json:"id"`
	BikeID           string   `json:"bike_id"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	StartStationID   *string  `json:"start_station_id,omitempty"`
	EndStationID     *string  `json:"end_station_id,omitempty"`
	StartStationName *string  `json:"start_station_name,omitempty"`
	EndStationName   *string  `json:"end_station_name,omitempty"`
	StartLat         float64  `json:"start_lat"`
	StartLon         float64  `json:"start_lon"`
	EndLat           float64  `json:"end_lat"`
	EndLon           float64  `json:"end_lon"`
	DistanceMeters   *int     `json:"distance_meters,omitempty"`
	BatteryStart     *int     `json:"battery_start,omitempty"`
	BatteryEnd       *int     `json:"battery_end,omitempty"`
	BatteryDelta     *int     `json:"battery_delta,omitempty"`
	DurationMinutes  *float64 `json:"duration_minutes,omitempty"`
}

type FleetStats struct {
	TotalBikes    int `json:"total_bikes"`
	AvailableNow  int `json:"available_now"`
	DisabledNow   int `json:"disabled_now"`
	ReservedNow   int `json:"reserved_now"`
	TripsToday    int `json:"trips_today"`
	TripsWeek     int `json:"trips_week"`
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type BatteryBucket struct {
	Range string `json:"range"`
	Count int    `json:"count"`
}

type BusiestStation struct {
	StationID string `json:"station_id"`
	Name      string `json:"name"`
	TripCount int    `json:"trip_count"`
}

type StationBike struct {
	BikeID        string `json:"bike_id"`
	VehicleTypeID string `json:"vehicle_type_id"`
	BatteryPct    int    `json:"battery_pct"`
	HealthScore   int    `json:"health_score"`
	HealthLabel   string `json:"health_label"`
}

type HeatPoint struct {
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Weight float64 `json:"weight"`
}

type Anomaly struct {
	BikeID        string    `json:"bike_id"`
	VehicleTypeID string    `json:"vehicle_type_id"`
	Lat           float64   `json:"lat"`
	Lon           float64   `json:"lon"`
	LastSeen      time.Time `json:"last_seen"`
	HoursOutside  float64   `json:"hours_outside"`
}

type NearestBike struct {
	BikeID             string  `json:"bike_id"`
	VehicleTypeID      string  `json:"vehicle_type_id"`
	Lat                float64 `json:"lat"`
	Lon                float64 `json:"lon"`
	CurrentRangeMeters int     `json:"current_range_meters"`
	BatteryPct         int     `json:"battery_pct"`
	DistanceMeters     int     `json:"distance_m"`
}

type NearestStation struct {
	StationID         string  `json:"station_id"`
	Name              string  `json:"name"`
	Lat               float64 `json:"lat"`
	Lon               float64 `json:"lon"`
	NumDocksAvailable int     `json:"num_docks_available"`
	DistanceMeters    int     `json:"distance_m"`
}

type ReplayBike struct {
	BikeID    string  `json:"b"`
	Lat       float64 `json:"la"`
	Lon       float64 `json:"lo"`
	StationID *string `json:"s,omitempty"`
}

type ReplayBucket struct {
	Time      time.Time    `json:"time"`
	Snapshots []ReplayBike `json:"snapshots"`
}
