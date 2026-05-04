export interface VehicleType {
  vehicle_type_id: string
  name: string
  form_factor: string
  propulsion_type: string
  max_range_meters: number
}

export interface Station {
  station_id: string
  name: string
  lat: number
  lon: number
  capacity: number
  vehicle_type_capacity?: Record<string, number>
  is_virtual_station: boolean
  is_charging_station: boolean
  last_updated: string
  num_bikes_available?: number
  num_docks_available?: number
}

export interface Bike {
  bike_id: string
  vehicle_type_id: string
  first_seen: string
  last_seen: string
  physical_bike_id?: number
  current_battery_pct?: number
  current_lat?: number
  current_lon?: number
  current_station_id?: string
  current_station_name?: string
  is_currently_disabled?: boolean
  last_snapshot_time?: string
}

export interface PhysicalBike {
  id: number
  vehicle_type_id: string
  fleet_number?: string
  custom_name?: string
  first_seen: string
  last_seen: string
  total_trips: number
  total_distance_km: number
  id_count?: number
  current_battery_pct?: number
  current_station_name?: string
  is_currently_disabled?: boolean
  current_bike_id?: string
  known_bike_ids?: string[]
}

export interface BikeLive {
  bike_id: string
  vehicle_type_id: string
  lat: number
  lon: number
  station_id: string | null
  is_reserved: boolean
  is_disabled: boolean
  current_range_meters: number
  battery_pct: number
}

export interface BikeSnapshot {
  time: string
  bike_id: string
  lat: number
  lon: number
  station_id: string | null
  is_disabled: boolean
  current_range_meters: number
}

export interface BikeStats {
  bike_id: string
  total_trips: number
  total_distance_km: number
  avg_battery_pct: number
  availability_pct: number
  disabled_pct: number
}

export interface BikeHealth {
  bike_id: string
  health_score: number
  battery_score: number
  reliability_score: number
  activity_score: number
  avg_battery_pct: number
  disabled_count_30d: number
  trips_30d: number
  label: string
}

export interface Trip {
  id: number
  bike_id: string
  start_time: string
  end_time: string
  start_station_id?: string
  end_station_id?: string
  start_station_name?: string
  end_station_name?: string
  start_lat: number
  start_lon: number
  end_lat: number
  end_lon: number
  distance_meters?: number
  battery_start?: number
  battery_end?: number
  battery_delta?: number
  duration_minutes?: number
}

export interface FleetStats {
  total_bikes: number
  available_now: number
  disabled_now: number
  reserved_now: number
  trips_today: number
  trips_week: number
}

export interface DailyCount {
  date: string
  count: number
}

export interface BatteryBucket {
  range: string
  count: number
}

export interface BusiestStation {
  station_id: string
  name: string
  trip_count: number
}

export interface StationBikeVisit {
  bike_id: string
  arrived_at: string
  departed_at?: string
  battery_arrival: number
  duration_minutes: number
  still_present: boolean
}

export interface StationBike {
  bike_id: string
  vehicle_type_id: string
  battery_pct: number
  health_score: number
  health_label: string
}

export interface GeoJsonFeatureCollection {
  type: 'FeatureCollection'
  features: {
    type: 'Feature'
    geometry: { type: string; coordinates: unknown }
    properties: Record<string, unknown>
  }[]
}

export interface HeatPoint {
  lat: number
  lon: number
  weight: number
}

export interface Anomaly {
  bike_id: string
  vehicle_type_id: string
  lat: number
  lon: number
  last_seen: string
  hours_outside: number
}

export interface NearestBike {
  bike_id: string
  vehicle_type_id: string
  lat: number
  lon: number
  current_range_meters: number
  battery_pct: number
  distance_m: number
}

export interface NearestStation {
  station_id: string
  name: string
  lat: number
  lon: number
  num_docks_available: number
  distance_m: number
}

export interface ReplayBike {
  b: string   // bike_id
  la: number  // lat
  lo: number  // lon
  s?: string  // station_id
}

export interface ReplayBucket {
  time: string
  snapshots: ReplayBike[]
}

export interface WSMessage {
  type: 'snapshot'
  bikes: BikeLive[]
  stations: Station[]
}

export interface MapFilters {
  showBikes: boolean
  showStations: boolean
  minBattery: number
  showElectric: boolean
  showManual: boolean
  hideDisabled: boolean
  showHeatmap: boolean
  showGeofencing: boolean
}
