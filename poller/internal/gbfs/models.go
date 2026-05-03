package gbfs

import "encoding/json"

// Root envelope shared by all GBFS feeds
type Feed[T any] struct {
	LastUpdated int64  `json:"last_updated"`
	TTL         int    `json:"ttl"`
	Version     string `json:"version"`
	Data        T      `json:"data"`
}

// gbfs.json (auto-discovery)
// The data object maps language codes ("en", "fr", …) to a list of feed refs.
type GbfsDiscoveryData map[string]struct {
	Feeds []GbfsFeedRef `json:"feeds"`
}

type GbfsFeedRef struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// vehicle_types.json
type VehicleTypesData struct {
	VehicleTypes []VehicleType `json:"vehicle_types"`
}

type VehicleType struct {
	VehicleTypeID  string `json:"vehicle_type_id"`
	FormFactor     string `json:"form_factor"`
	PropulsionType string `json:"propulsion_type"`
	MaxRangeMeters int    `json:"max_range_meters"`
	Name           string `json:"name"`
}

// station_information.json
type StationInfoData struct {
	Stations []StationInfo `json:"stations"`
}

type StationInfo struct {
	StationID           string         `json:"station_id"`
	Name                string         `json:"name"`
	Lat                 float64        `json:"lat"`
	Lon                 float64        `json:"lon"`
	Capacity            int            `json:"capacity"`
	VehicleTypeCapacity map[string]int `json:"vehicle_type_capacity"`
	IsVirtualStation    bool           `json:"is_virtual_station"`
	IsChargingStation   bool           `json:"is_charging_station"`
}

// station_status.json
type StationStatusData struct {
	Stations []StationStatus `json:"stations"`
}

type StationStatus struct {
	StationID             string             `json:"station_id"`
	NumBikesAvailable     int                `json:"num_bikes_available"`
	NumDocksAvailable     int                `json:"num_docks_available"`
	IsInstalled           bool               `json:"is_installed"`
	IsRenting             bool               `json:"is_renting"`
	IsReturning           bool               `json:"is_returning"`
	LastReported          int64              `json:"last_reported"`
	VehicleTypesAvailable []VehicleTypeCount `json:"vehicle_types_available"`
}

type VehicleTypeCount struct {
	VehicleTypeID string `json:"vehicle_type_id"`
	Count         int    `json:"count"`
}

// free_bike_status.json
type FreeBikeData struct {
	Bikes []Bike `json:"bikes"`
}

type Bike struct {
	BikeID             string  `json:"bike_id"`
	SystemID           string  `json:"system_id"`
	Lat                float64 `json:"lat"`
	Lon                float64 `json:"lon"`
	IsReserved         bool    `json:"is_reserved"`
	IsDisabled         bool    `json:"is_disabled"`
	VehicleTypeID      string  `json:"vehicle_type_id"`
	LastReported       int64   `json:"last_reported"`
	CurrentRangeMeters int     `json:"current_range_meters"`
	StationID          string  `json:"station_id"`
	PricingPlanID      string  `json:"pricing_plan_id"`
}

// geofencing_zones.json
// GeofencingZones is kept as raw JSON — it is a GeoJSON FeatureCollection whose
// exact schema varies by provider. We store and forward it verbatim.
type GeofencingZonesData struct {
	GeofencingZones json.RawMessage `json:"geofencing_zones"`
}
