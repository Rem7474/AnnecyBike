import { useState } from 'react'
import { MapContainer, TileLayer, Marker, Popup, useMap } from 'react-leaflet'
import { Icon, DivIcon } from 'leaflet'
import type { BikeLive, Station, MapFilters, HeatPoint } from '../../types'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { HeatmapLayer } from './HeatmapLayer'
import { MapFiltersPanel } from './MapFilters'
import { NearestBikeControl } from './NearestBike'
import { ReplayPlayerInner } from './ReplayPlayer'

delete (Icon.Default.prototype as any)._getIconUrl
Icon.Default.mergeOptions({
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
  iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
})

function batteryColor(pct: number, isDisabled: boolean, isReserved: boolean) {
  if (isDisabled) return '#6b7280'
  if (isReserved) return '#8b5cf6'
  if (pct < 20) return '#ef4444'
  if (pct < 50) return '#f97316'
  return '#22c55e'
}

function bikeIcon(bike: BikeLive) {
  const color = batteryColor(bike.battery_pct, bike.is_disabled, bike.is_reserved)
  const html = `<div style="
    width:28px;height:28px;border-radius:50%;
    background:${color};border:2px solid white;
    display:flex;align-items:center;justify-content:center;
    font-size:10px;font-weight:bold;color:white;
    box-shadow:0 2px 4px rgba(0,0,0,0.4)">
    ${bike.battery_pct}%
  </div>`
  return new DivIcon({ html, className: '', iconSize: [28, 28], iconAnchor: [14, 14] })
}

function stationIcon(avail: number, cap: number) {
  const ratio = cap > 0 ? avail / cap : 0
  const bg = ratio > 0.5 ? '#22c55e' : ratio > 0.2 ? '#f97316' : avail === 0 ? '#6b7280' : '#ef4444'
  const html = `<div style="
    width:36px;height:36px;border-radius:8px;
    background:${bg};border:2px solid white;
    display:flex;flex-direction:column;align-items:center;justify-content:center;
    font-weight:bold;color:white;line-height:1.1;
    box-shadow:0 2px 4px rgba(0,0,0,0.4)">
    <span style="font-size:14px">${avail}</span>
    <span style="font-size:8px;opacity:0.85">vélos</span>
  </div>`
  return new DivIcon({ html, className: '', iconSize: [36, 36], iconAnchor: [18, 18] })
}

function GeolocateControl() {
  const map = useMap()
  const locate = () => {
    map.locate({ setView: true, maxZoom: 16 })
    map.once('locationerror', () => alert('Géolocalisation refusée ou indisponible.'))
  }
  return (
    <div onClick={locate} title="Ma position" style={{
      position: 'absolute', bottom: 24, right: 12, zIndex: 1000,
      width: 36, height: 36, borderRadius: 8,
      background: 'white', border: '2px solid #cbd5e1',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      cursor: 'pointer', boxShadow: '0 2px 6px rgba(0,0,0,0.25)', fontSize: 18,
    }}>⊕</div>
  )
}

function ReplayToggle({ active, onToggle }: { active: boolean; onToggle: () => void }) {
  return (
    <div onClick={onToggle} title="Replay temporel" style={{
      position: 'absolute', bottom: 68, left: 12, zIndex: 1000,
      background: active ? '#3b82f6' : 'rgba(15,23,42,0.92)',
      border: '1px solid #334155', borderRadius: 8,
      padding: '7px 12px', color: '#f1f5f9',
      fontSize: 12, cursor: 'pointer', backdropFilter: 'blur(6px)',
    }}>
      ⏱ Replay
    </div>
  )
}

// Electric vehicle type IDs (from GBFS data: 1,2,4,5,6,7,15 = electric, 10,14 = human)
const ELECTRIC_IDS = new Set(['1', '2', '4', '5', '6', '7', '15'])

interface MapContentProps {
  bikes: BikeLive[]
  stations: Station[]
  filters: MapFilters
  heatPoints: HeatPoint[]
  showReplay: boolean
  onCloseReplay: () => void
}

function MapContent({ bikes, stations, filters, heatPoints, showReplay, onCloseReplay }: MapContentProps) {
  const navigate = useNavigate()

  const filteredBikes = bikes.filter((b) => {
    if (filters.hideDisabled && b.is_disabled) return false
    if (b.battery_pct < filters.minBattery) return false
    const isElectric = ELECTRIC_IDS.has(b.vehicle_type_id)
    if (!filters.showElectric && isElectric) return false
    if (!filters.showManual && !isElectric) return false
    return true
  })

  return (
    <>
      {filters.showHeatmap && <HeatmapLayer points={heatPoints} />}

      {stations.map((st) => {
        const avail = st.num_bikes_available ?? 0
        const cap = st.capacity ?? 1
        return (
          <Marker
            key={st.station_id}
            position={[st.lat, st.lon]}
            icon={stationIcon(avail, cap)}
            eventHandlers={{ click: () => navigate(`/stations/${st.station_id}`) }}
          >
            <Popup>
              <strong>{st.name}</strong><br />
              <span style={{ color: avail > 0 ? 'green' : 'gray' }}>
                {avail} vélo{avail !== 1 ? 's' : ''} disponible{avail !== 1 ? 's' : ''}
              </span><br />
              {st.num_docks_available ?? '?'} dock{(st.num_docks_available ?? 0) !== 1 ? 's' : ''} libre{(st.num_docks_available ?? 0) !== 1 ? 's' : ''}<br />
              <small style={{ color: '#888' }}>Capacité : {cap}</small>
            </Popup>
          </Marker>
        )
      })}

      {!showReplay && filteredBikes.map((bike) => (
        <Marker
          key={bike.bike_id}
          position={[bike.lat, bike.lon]}
          icon={bikeIcon(bike)}
          eventHandlers={{ click: () => navigate(`/bikes/${bike.bike_id}`) }}
        >
          <Popup>
            <strong>{bike.bike_id.slice(0, 8)}…</strong><br />
            Batterie : {bike.battery_pct}%<br />
            {bike.is_disabled ? '🔴 Hors service' : bike.is_reserved ? '🟣 Réservé' : '🟢 Disponible'}
          </Popup>
        </Marker>
      ))}

      <GeolocateControl />
      <NearestBikeControl />
      <ReplayToggle active={showReplay} onToggle={onCloseReplay} />
      {showReplay && <ReplayPlayerInner onClose={onCloseReplay} />}
    </>
  )
}

interface Props {
  bikes: BikeLive[]
  stations: Station[]
}

const DEFAULT_FILTERS: MapFilters = {
  minBattery: 0,
  showElectric: true,
  showManual: true,
  hideDisabled: false,
  showHeatmap: false,
}

export function LiveMap({ bikes, stations }: Props) {
  const [filters, setFilters] = useState<MapFilters>(DEFAULT_FILTERS)
  const [showReplay, setShowReplay] = useState(false)

  const { data: heatPoints = [] } = useQuery({
    queryKey: ['heatmap'],
    queryFn: () => api.stats.heatmap(30),
    enabled: filters.showHeatmap,
    staleTime: 5 * 60_000,
  })

  return (
    <div style={{ flex: 1, position: 'relative' }}>
      <MapFiltersPanel filters={filters} onChange={setFilters} />
      <MapContainer
        center={[45.899, 6.129]}
        zoom={14}
        style={{ height: '100%', width: '100%' }}
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <MapContent
          bikes={bikes}
          stations={stations}
          filters={filters}
          heatPoints={heatPoints}
          showReplay={showReplay}
          onCloseReplay={() => setShowReplay((v) => !v)}
        />
      </MapContainer>
    </div>
  )
}
