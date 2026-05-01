import { useState } from 'react'
import { useMap } from 'react-leaflet'
import { Marker, Popup, Polyline } from 'react-leaflet'
import { DivIcon } from 'leaflet'
import { api } from '../../api'
import type { NearestBike, NearestStation } from '../../types'
import { useNavigate } from 'react-router-dom'

const btnStyle: React.CSSProperties = {
  position: 'absolute', bottom: 68, right: 12, zIndex: 1000,
  background: 'rgba(15,23,42,0.92)', border: '1px solid #334155',
  borderRadius: 8, padding: '8px 12px', color: '#f1f5f9',
  fontSize: 12, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6,
  backdropFilter: 'blur(6px)',
}

const destIcon = new DivIcon({
  html: `<div style="width:20px;height:20px;border-radius:50%;background:#f59e0b;border:2px solid white;box-shadow:0 2px 4px rgba(0,0,0,.4)"></div>`,
  className: '', iconSize: [20, 20], iconAnchor: [10, 10],
})

interface Result {
  userLat: number; userLon: number
  destLat: number; destLon: number
  bike: NearestBike
  station: NearestStation
}

// Inner component that has access to the map
export function NearestBikeControl() {
  const map = useMap()
  const navigate = useNavigate()
  const [result, setResult] = useState<Result | null>(null)
  const [picking, setPicking] = useState(false)
  const [loading, setLoading] = useState(false)

  const startPick = () => {
    setPicking(true)
    map.once('click', async (e) => {
      setPicking(false)
      const destLat = e.latlng.lat
      const destLon = e.latlng.lng
      setLoading(true)
      try {
        const pos = await new Promise<GeolocationPosition>((res, rej) =>
          navigator.geolocation.getCurrentPosition(res, rej, { timeout: 8000 })
        )
        const { latitude: userLat, longitude: userLon } = pos.coords
        const [bikes, stations] = await Promise.all([
          api.bikes.nearest(userLat, userLon, 1),
          api.stations.nearest(destLat, destLon, 1),
        ])
        if (bikes.length && stations.length) {
          setResult({ userLat, userLon, destLat, destLon, bike: bikes[0], station: stations[0] })
        }
      } catch {
        alert('Géolocalisation refusée ou aucun vélo/station trouvé.')
      } finally {
        setLoading(false)
      }
    })
  }

  const clear = () => setResult(null)

  const label = loading ? '⏳ Recherche…' : picking ? '🎯 Cliquer destination…' : '🗺 Itinéraire vélo'

  return (
    <>
      <div style={btnStyle} onClick={result ? clear : startPick}>
        {result ? '✕ Effacer' : label}
      </div>

      {result && (
        <>
          {/* Destination marker */}
          <Marker position={[result.destLat, result.destLon]} icon={destIcon}>
            <Popup>Destination de retour</Popup>
          </Marker>

          {/* Suggested bike */}
          <Marker
            position={[result.bike.lat, result.bike.lon]}
            icon={new DivIcon({
              html: `<div style="width:32px;height:32px;border-radius:50%;background:#22c55e;border:3px solid white;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:bold;color:white;box-shadow:0 2px 6px rgba(0,0,0,.5)">${result.bike.battery_pct}%</div>`,
              className: '', iconSize: [32, 32], iconAnchor: [16, 16],
            })}
            eventHandlers={{ click: () => navigate(`/bikes/${result.bike.bike_id}`) }}
          >
            <Popup>
              <strong>Vélo recommandé</strong><br />
              Batterie : {result.bike.battery_pct}%<br />
              À {result.bike.distance_m} m de vous
            </Popup>
          </Marker>

          {/* Suggested return station */}
          <Marker
            position={[result.station.lat, result.station.lon]}
            icon={new DivIcon({
              html: `<div style="width:32px;height:32px;border-radius:8px;background:#3b82f6;border:3px solid white;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:bold;color:white;box-shadow:0 2px 6px rgba(0,0,0,.5)">P</div>`,
              className: '', iconSize: [32, 32], iconAnchor: [16, 16],
            })}
            eventHandlers={{ click: () => navigate(`/stations/${result.station.station_id}`) }}
          >
            <Popup>
              <strong>{result.station.name}</strong><br />
              {result.station.num_docks_available} docks libres<br />
              À {result.station.distance_m} m de la destination
            </Popup>
          </Marker>

          {/* Route line: user → bike */}
          <Polyline
            positions={[[result.userLat, result.userLon], [result.bike.lat, result.bike.lon]]}
            pathOptions={{ color: '#22c55e', weight: 3, dashArray: '6 4' }}
          />
          {/* Route line: destination → station */}
          <Polyline
            positions={[[result.destLat, result.destLon], [result.station.lat, result.station.lon]]}
            pathOptions={{ color: '#3b82f6', weight: 3, dashArray: '6 4' }}
          />
        </>
      )}
    </>
  )
}
