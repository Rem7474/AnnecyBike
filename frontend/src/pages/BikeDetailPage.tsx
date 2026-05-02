import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  LineChart, Line, XAxis, YAxis, Tooltip,
  ResponsiveContainer, CartesianGrid, ReferenceLine,
} from 'recharts'
import { MapContainer, TileLayer, Polyline, CircleMarker, Popup } from 'react-leaflet'
import { api } from '../api'

const S: Record<string, React.CSSProperties> = {
  page: { padding: 24, maxWidth: 1100, margin: '0 auto' },
  back: { color: '#94a3b8', textDecoration: 'none', fontSize: 13 },
  title: { fontSize: 22, fontWeight: 700, margin: '12px 0 4px' },
  cards: { display: 'flex', gap: 12, flexWrap: 'wrap', margin: '16px 0' },
  card: { background: '#1e293b', borderRadius: 8, padding: '12px 20px', minWidth: 140 },
  cardLabel: { fontSize: 11, color: '#64748b', textTransform: 'uppercase', letterSpacing: 1 },
  cardValue: { fontSize: 24, fontWeight: 700, marginTop: 4 },
  section: { marginTop: 24 },
  sectionTitle: { fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#94a3b8' },
  table: { width: '100%', borderCollapse: 'collapse', fontSize: 13 },
  th: { textAlign: 'left', padding: '6px 10px', color: '#64748b', borderBottom: '1px solid #1e293b' },
  td: { padding: '8px 10px', borderBottom: '1px solid #1e293b' },
}

function healthColor(score: number) {
  if (score >= 70) return '#22c55e'
  if (score >= 40) return '#f97316'
  return '#ef4444'
}

function fmt(n?: number, unit = '') {
  if (n === undefined || n === null) return '—'
  return `${Math.round(n)}${unit}`
}

export function BikeDetailPage() {
  const { id } = useParams<{ id: string }>()

  const { data: bike } = useQuery({ queryKey: ['bike', id], queryFn: () => api.bikes.get(id!) })
  const { data: stats } = useQuery({ queryKey: ['bike-stats', id], queryFn: () => api.bikes.stats(id!) })
  const { data: health } = useQuery({ queryKey: ['bike-health', id], queryFn: () => api.bikes.health(id!) })
  const { data: trips } = useQuery({ queryKey: ['bike-trips', id], queryFn: () => api.bikes.trips(id!, 20) })
  const { data: history } = useQuery({
    queryKey: ['bike-history', id, '7d'],
    queryFn: () => {
      const to = new Date().toISOString()
      const from = new Date(Date.now() - 7 * 86400_000).toISOString()
      return api.bikes.history(id!, from, to, '1h')
    },
  })

  // Last 24h raw history for trajectory map
  const { data: trajectory } = useQuery({
    queryKey: ['bike-trajectory', id],
    queryFn: () => {
      const to = new Date().toISOString()
      const from = new Date(Date.now() - 24 * 3600_000).toISOString()
      return api.bikes.history(id!, from, to, 'raw')
    },
  })

  const chartData = (history ?? []).map((s) => ({
    time: new Date(s.time).toLocaleString('fr-FR', { day: '2-digit', month: '2-digit', hour: '2-digit' }),
    battery: Math.round((s.current_range_meters / 45000) * 100),
  })).reverse()

  // Build trajectory polyline segments: each continuous free-floating period
  // becomes its own segment, so we don't draw phantom lines between separate trips.
  const trajectorySegments: [number, number][][] = []
  {
    let seg: [number, number][] = []
    // trajectory arrives newest-first; reverse to walk chronologically
    for (const s of (trajectory ?? []).slice().reverse()) {
      if (s.station_id === null) {
        seg.push([s.lat, s.lon])
      } else {
        if (seg.length > 1) trajectorySegments.push(seg)
        seg = []
      }
    }
    if (seg.length > 1) trajectorySegments.push(seg)
  }

  const tripLines = (trips ?? [])
    .filter((t) => t.start_lat !== 0 && t.start_lon !== 0 && t.end_lat !== 0 && t.end_lon !== 0)
    .map((t) => ({
      id: t.id,
      path: [[t.start_lat, t.start_lon], [t.end_lat, t.end_lon]] as [number, number][],
      battery: t.battery_delta,
    }))

  const mapCenter: [number, number] = trajectorySegments[0]?.[0]
    ?? tripLines[0]?.path[0]
    ?? [45.899, 6.129]

  return (
    <div style={S.page}>
      <Link to="/" style={S.back}>← Retour à la carte</Link>
      <div style={S.title}>
        Vélo <code style={{ fontSize: 16, color: '#38bdf8' }}>{id?.slice(0, 8)}…</code>
        {health && (
          <span style={{
            marginLeft: 12, fontSize: 14, fontWeight: 700,
            color: healthColor(health.health_score),
            background: '#1e293b', borderRadius: 6,
            padding: '3px 10px', verticalAlign: 'middle',
          }}>
            ❤ {health.health_score}/100 — {health.label}
          </span>
        )}
      </div>
      <div style={{ fontSize: 13, color: '#64748b' }}>Type : {bike?.vehicle_type_id ?? '—'}</div>

      {/* Health score breakdown */}
      {health && (
        <div style={{ ...S.cards, marginTop: 12 }}>
          <div style={{ ...S.card, borderLeft: `4px solid ${healthColor(health.health_score)}` }}>
            <div style={S.cardLabel}>Score global</div>
            <div style={{ ...S.cardValue, color: healthColor(health.health_score) }}>{health.health_score}/100</div>
          </div>
          <div style={S.card}>
            <div style={S.cardLabel}>Batterie (×0.4)</div>
            <div style={S.cardValue}>{health.battery_score}/40</div>
          </div>
          <div style={S.card}>
            <div style={S.cardLabel}>Fiabilité (30j)</div>
            <div style={S.cardValue}>{health.reliability_score}/30</div>
          </div>
          <div style={S.card}>
            <div style={S.cardLabel}>Activité (30j)</div>
            <div style={S.cardValue}>{health.activity_score}/30</div>
          </div>
          <div style={S.card}>
            <div style={S.cardLabel}>Incidents hors-service</div>
            <div style={{ ...S.cardValue, color: health.disabled_count_30d > 3 ? '#ef4444' : '#f1f5f9' }}>
              {health.disabled_count_30d}j
            </div>
          </div>
        </div>
      )}

      {/* Usage stats */}
      <div style={S.cards}>
        <div style={S.card}>
          <div style={S.cardLabel}>Trajets totaux</div>
          <div style={S.cardValue}>{stats?.total_trips ?? '—'}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Distance totale</div>
          <div style={S.cardValue}>{fmt(stats?.total_distance_km)} km</div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Batterie moy. (7j)</div>
          <div style={S.cardValue}>{fmt(stats?.avg_battery_pct)}%</div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Disponibilité</div>
          <div style={S.cardValue}>{fmt(stats?.availability_pct)}%</div>
        </div>
      </div>

      {/* Battery chart + trajectory map side by side */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginTop: 24 }}>
        <div>
          <div style={S.sectionTitle}>Batterie — 7 derniers jours</div>
          <div style={{ background: '#1e293b', borderRadius: 8, padding: 16 }}>
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                <XAxis dataKey="time" tick={{ fill: '#64748b', fontSize: 9 }} interval="preserveStartEnd" />
                <YAxis domain={[0, 100]} tick={{ fill: '#64748b', fontSize: 11 }} unit="%" />
                <ReferenceLine y={20} stroke="#ef4444" strokeDasharray="4 2" label={{ value: '20%', fill: '#ef4444', fontSize: 10 }} />
                <Tooltip
                  contentStyle={{ background: '#0f172a', border: '1px solid #334155' }}
                  formatter={(v: number) => [`${v}%`, 'Batterie']}
                />
                <Line type="monotone" dataKey="battery" stroke="#38bdf8" dot={false} strokeWidth={2} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div>
          <div style={S.sectionTitle}>Trajectoires — 24 dernières heures</div>
          <div style={{ borderRadius: 8, overflow: 'hidden', height: 232 }}>
            <MapContainer center={mapCenter} zoom={13} style={{ height: '100%' }} zoomControl={false}>
              <TileLayer
                url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
                attribution=""
              />
              {/* Free-floating path — one Polyline per continuous segment */}
              {trajectorySegments.map((seg, i) => (
                <Polyline key={i} positions={seg} pathOptions={{ color: '#38bdf8', weight: 2 }} />
              ))}
              {/* Trip start/end lines */}
              {tripLines.map((t) => (
                <Polyline
                  key={t.id}
                  positions={t.path}
                  pathOptions={{
                    color: (t.battery ?? 0) < -5000 ? '#f97316' : '#22c55e',
                    weight: 2, dashArray: '5 3',
                  }}
                />
              ))}
              {/* Start markers */}
              {tripLines.map((t) => (
                <CircleMarker key={`s-${t.id}`} center={t.path[0]} radius={4}
                  pathOptions={{ color: '#22c55e', fillColor: '#22c55e', fillOpacity: 1 }}>
                  <Popup>Départ trajet #{t.id}</Popup>
                </CircleMarker>
              ))}
            </MapContainer>
          </div>
        </div>
      </div>

      {/* Trips table */}
      <div style={S.section}>
        <div style={S.sectionTitle}>Derniers trajets</div>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>Départ</th>
              <th style={S.th}>Durée</th>
              <th style={S.th}>Distance</th>
              <th style={S.th}>Batterie consommée</th>
              <th style={S.th}>Station départ → arrivée</th>
            </tr>
          </thead>
          <tbody>
            {(trips ?? []).map((t) => (
              <tr key={t.id}>
                <td style={S.td}>{new Date(t.start_time).toLocaleString('fr-FR')}</td>
                <td style={S.td}>{fmt(t.duration_minutes)} min</td>
                <td style={S.td}>{t.distance_meters != null ? `${(t.distance_meters / 1000).toFixed(1)} km` : '—'}</td>
                <td style={S.td}>
                  {t.battery_delta !== undefined
                    ? `${t.battery_delta <= 0 ? '−' : '+'}${Math.round((Math.abs(t.battery_delta) / 45000) * 100)}%`
                    : '—'}
                </td>
                <td style={{ ...S.td, fontSize: 11, color: '#64748b' }}>
                  {t.start_station_name ?? t.start_station_id?.slice(0, 8) ?? '?'} → {t.end_station_name ?? t.end_station_id?.slice(0, 8) ?? '?'}
                </td>
              </tr>
            ))}
            {(trips ?? []).length === 0 && (
              <tr><td style={S.td} colSpan={5}>Aucun trajet enregistré</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
