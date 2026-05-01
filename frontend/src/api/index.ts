import type { Bike, BikeLive, BikeSnapshot, BikeStats, BusiestStation, BatteryBucket, DailyCount, FleetStats, Station, Trip } from '../types'

const BASE = '/api/v1'

async function get<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${url}`)
  return res.json()
}

export const api = {
  bikes: {
    live: () => get<BikeLive[]>(`${BASE}/bikes/live`),
    list: (limit = 100, offset = 0) => get<Bike[]>(`${BASE}/bikes?limit=${limit}&offset=${offset}`),
    get: (id: string) => get<Bike>(`${BASE}/bikes/${id}`),
    history: (id: string, from?: string, to?: string, resolution = 'raw') => {
      const params = new URLSearchParams({ resolution })
      if (from) params.set('from', from)
      if (to) params.set('to', to)
      return get<BikeSnapshot[]>(`${BASE}/bikes/${id}/history?${params}`)
    },
    trips: (id: string, limit = 50, offset = 0) =>
      get<Trip[]>(`${BASE}/bikes/${id}/trips?limit=${limit}&offset=${offset}`),
    stats: (id: string) => get<BikeStats>(`${BASE}/bikes/${id}/stats`),
  },
  stations: {
    live: () => get<Station[]>(`${BASE}/stations/live`),
    get: (id: string) => get<Station>(`${BASE}/stations/${id}`),
    history: (id: string, from?: string, to?: string) => {
      const params = new URLSearchParams()
      if (from) params.set('from', from)
      if (to) params.set('to', to)
      return get<{ time: string; num_bikes_available: number; num_docks_available: number }[]>(
        `${BASE}/stations/${id}/history?${params}`
      )
    },
  },
  trips: {
    list: (params: { bike_id?: string; station_id?: string; from?: string; to?: string; limit?: number }) => {
      const p = new URLSearchParams()
      Object.entries(params).forEach(([k, v]) => v !== undefined && p.set(k, String(v)))
      return get<Trip[]>(`${BASE}/trips?${p}`)
    },
  },
  stats: {
    fleet: () => get<FleetStats>(`${BASE}/stats/fleet`),
    tripsPerDay: (days = 30) => get<DailyCount[]>(`${BASE}/stats/trips-per-day?days=${days}`),
    batteryDistribution: () => get<BatteryBucket[]>(`${BASE}/stats/battery-distribution`),
    busiestStations: (limit = 10) => get<BusiestStation[]>(`${BASE}/stats/busiest-stations?limit=${limit}`),
  },
}
