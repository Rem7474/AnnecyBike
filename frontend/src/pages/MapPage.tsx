import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { useLiveMap } from '../hooks/useLiveMap'
import { LiveMap } from '../components/map/LiveMap'

const dotStyle = (ok: boolean): React.CSSProperties => ({
  width: 8, height: 8, borderRadius: '50%',
  background: ok ? '#22c55e' : '#ef4444', flexShrink: 0,
})

const barStyle: React.CSSProperties = {
  padding: '8px 16px', background: '#1e293b', display: 'flex',
  gap: 16, alignItems: 'center', fontSize: 13, borderBottom: '1px solid #334155',
}

export function MapPage() {
  const { bikes, stations, connected } = useLiveMap()

  const { data: initBikes } = useQuery({
    queryKey: ['bikes-live'],
    queryFn: api.bikes.live,
    staleTime: 60_000,
    enabled: bikes.length === 0,
  })
  const { data: initStations } = useQuery({
    queryKey: ['stations-live'],
    queryFn: api.stations.live,
    staleTime: 60_000,
    enabled: stations.length === 0,
  })

  const displayBikes = bikes.length > 0 ? bikes : (initBikes ?? [])
  const displayStations = stations.length > 0 ? stations : (initStations ?? [])

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <div style={barStyle}>
        <div style={dotStyle(connected)} />
        <span>{connected ? 'Temps réel' : 'Reconnexion…'}</span>
        <span style={{ marginLeft: 'auto' }}>
          {displayBikes.length} vélos · {displayStations.length} stations
        </span>
      </div>
      <LiveMap bikes={displayBikes} stations={displayStations} />
    </div>
  )
}
