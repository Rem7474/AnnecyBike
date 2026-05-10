import type {
  Anomaly, Bike, BikeLive, BikeHealth, BikeSnapshot, BikeStats,
  BusiestStation, BatteryBucket, DailyCount, FleetStats,
  GeoJsonFeatureCollection, HeatPoint, HourlyBikeStats, NearestBike, NearestStation, PhysicalBike, ReplayBucket,
  Station, StationBike, StationBikeVisit, Trip,
} from '../types'

const BASE = '/api/v1'

async function get<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${url}`)
  return res.json()
}

async function patch<T>(url: string, body: unknown): Promise<T> {
  const res = await fetch(url, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${url}`)
  return res.json()
}

async function post<T>(url: string, body: unknown): Promise<T> {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${url}`)
  return res.json()
}

export const api = {
  bikes: {
    live: () => get<BikeLive[]>(`${BASE}/bikes/live`),
    list: (limit = 100, offset = 0) => get<Bike[]>(`${BASE}/bikes?limit=${limit}&offset=${offset}`),
    get: (id: string) => get<Bike>(`${BASE}/bikes/${id}`),
    history: (id: string, from?: string, to?: string, resolution = 'raw') => {
      const p = new URLSearchParams({ resolution })
      if (from) p.set('from', from)
      if (to) p.set('to', to)
      return get<BikeSnapshot[]>(`${BASE}/bikes/${id}/history?${p}`)
    },
    trips: (id: string, limit = 50, offset = 0) =>
      get<Trip[]>(`${BASE}/bikes/${id}/trips?limit=${limit}&offset=${offset}`),
    stats: (id: string) => get<BikeStats>(`${BASE}/bikes/${id}/stats`),
    health: (id: string) => get<BikeHealth>(`${BASE}/bikes/${id}/health`),
    nearest: (lat: number, lon: number, limit = 5) =>
      get<NearestBike[]>(`${BASE}/bikes/nearest?lat=${lat}&lon=${lon}&limit=${limit}`),
    assign: (bikeId: string, fleetNumber: string) =>
      post<{ ok: boolean; physical_bike_id: number }>(`${BASE}/bikes/${bikeId}/assign`, { fleet_number: fleetNumber }),
  },
  stations: {
    live: () => get<Station[]>(`${BASE}/stations/live`),
    get: (id: string) => get<Station>(`${BASE}/stations/${id}`),
    bikes: (id: string) => get<StationBike[]>(`${BASE}/stations/${id}/bikes`),
    history: (id: string, from?: string, to?: string) => {
      const p = new URLSearchParams()
      if (from) p.set('from', from)
      if (to) p.set('to', to)
      return get<{ time: string; num_bikes_available: number; num_docks_available: number }[]>(
        `${BASE}/stations/${id}/history?${p}`
      )
    },
    nearest: (lat: number, lon: number, limit = 5) =>
      get<NearestStation[]>(`${BASE}/stations/nearest?lat=${lat}&lon=${lon}&limit=${limit}`),
    bikeHistory: (id: string, hours = 48) =>
      get<StationBikeVisit[]>(`${BASE}/stations/${id}/bike-history?hours=${hours}`),
    hourlyStats: (id: string, days = 90) =>
      get<HourlyBikeStats[]>(`${BASE}/stations/${id}/hourly-stats?days=${days}`),
  },
  physicalBikes: {
    list: () => get<PhysicalBike[]>(`${BASE}/physical-bikes`),
    get: (id: number) => get<PhysicalBike>(`${BASE}/physical-bikes/${id}`),
    update: (id: number, body: { fleet_number: string | null; custom_name: string | null }) =>
      patch<{ ok: boolean }>(`${BASE}/physical-bikes/${id}`, body),
    reassign: (bikeId: string, physicalBikeId: number) =>
      patch<{ ok: boolean }>(`${BASE}/bikes/${bikeId}/reassign`, { physical_bike_id: physicalBikeId }),
    delete: (id: number) => fetch(`${BASE}/physical-bikes/${id}`, { method: 'DELETE' }).then(r => { if (!r.ok) throw new Error(`HTTP ${r.status}`) }),
    trips: (id: number, limit = 50, offset = 0) =>
      get<Trip[]>(`${BASE}/physical-bikes/${id}/trips?limit=${limit}&offset=${offset}`),
    history: (id: number, from?: string, to?: string) => {
      const p = new URLSearchParams()
      if (from) p.set('from', from)
      if (to) p.set('to', to)
      return get<BikeSnapshot[]>(`${BASE}/physical-bikes/${id}/history?${p}`)
    },
  },
  trips: {
    list: (params: { bike_id?: string; station_id?: string; from?: string; to?: string; limit?: number }) => {
      const p = new URLSearchParams()
      Object.entries(params).forEach(([k, v]) => v !== undefined && p.set(k, String(v)))
      return get<Trip[]>(`${BASE}/trips?${p}`)
    },
  },
  anomalies: {
    list: () => get<Anomaly[]>(`${BASE}/anomalies`),
  },
  geofencing: {
    zones: () => get<GeoJsonFeatureCollection>(`${BASE}/geofencing`),
  },
  replay: {
    get: (date: string, resolution = 10) =>
      get<ReplayBucket[]>(`${BASE}/replay?date=${date}&resolution=${resolution}`),
  },
  stats: {
    fleet: () => get<FleetStats>(`${BASE}/stats/fleet`),
    heatmap: (days = 30) => get<HeatPoint[]>(`${BASE}/stats/heatmap?days=${days}`),
    tripsPerDay: (days = 30) => get<DailyCount[]>(`${BASE}/stats/trips-per-day?days=${days}`),
    batteryDistribution: () => get<BatteryBucket[]>(`${BASE}/stats/battery-distribution`),
    busiestStations: (limit = 10) => get<BusiestStation[]>(`${BASE}/stats/busiest-stations?limit=${limit}`),
  },
}
