import { MapContainer, TileLayer, Marker, Popup, CircleMarker } from 'react-leaflet'
import { Icon, DivIcon } from 'leaflet'
import type { BikeLive, Station } from '../../types'
import { useNavigate } from 'react-router-dom'

// Suppress default icon import issues
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

interface Props {
  bikes: BikeLive[]
  stations: Station[]
}

export function LiveMap({ bikes, stations }: Props) {
  const navigate = useNavigate()

  return (
    <MapContainer
      center={[45.899, 6.129]}
      zoom={13}
      style={{ flex: 1, width: '100%' }}
      preferCanvas
    >
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />

      {stations.map((st) => {
        const avail = st.num_bikes_available ?? 0
        const cap = st.capacity || 1
        const ratio = avail / cap
        const color = ratio > 0.5 ? '#22c55e' : ratio > 0.2 ? '#f97316' : '#ef4444'
        return (
          <CircleMarker
            key={st.station_id}
            center={[st.lat, st.lon]}
            radius={10}
            pathOptions={{ color, fillColor: color, fillOpacity: 0.6, weight: 2 }}
            eventHandlers={{ click: () => navigate(`/stations/${st.station_id}`) }}
          >
            <Popup>
              <strong>{st.name}</strong><br />
              {avail}/{cap} vélos disponibles
            </Popup>
          </CircleMarker>
        )
      })}

      {bikes.map((bike) => (
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
    </MapContainer>
  )
}
