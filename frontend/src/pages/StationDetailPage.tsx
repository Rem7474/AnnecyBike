import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts'
import { api } from '../api'

const S: Record<string, React.CSSProperties> = {
  page: { padding: 24, maxWidth: 1100, margin: '0 auto' },
  back: { color: '#94a3b8', textDecoration: 'none', fontSize: 13 },
  title: { fontSize: 22, fontWeight: 700, margin: '12px 0 4px' },
  meta: { fontSize: 13, color: '#64748b', marginBottom: 16 },
  cards: { display: 'flex', gap: 12, flexWrap: 'wrap', margin: '16px 0' },
  card: { background: '#1e293b', borderRadius: 8, padding: '12px 20px', minWidth: 140 },
  cardLabel: { fontSize: 11, color: '#64748b', textTransform: 'uppercase', letterSpacing: 1 },
  cardValue: { fontSize: 24, fontWeight: 700, marginTop: 4 },
  section: { marginTop: 24 },
  sectionTitle: { fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#94a3b8' },
  table: { width: '100%', borderCollapse: 'collapse' as const, fontSize: 13 },
  th: { textAlign: 'left' as const, padding: '6px 10px', color: '#64748b', borderBottom: '1px solid #334155' },
  td: { padding: '8px 10px', borderBottom: '1px solid #1e293b' },
}

function healthColor(score: number) {
  if (score >= 70) return '#22c55e'
  if (score >= 40) return '#f97316'
  return '#ef4444'
}

function batteryBarColor(pct: number) {
  if (pct >= 60) return '#22c55e'
  if (pct >= 30) return '#f97316'
  return '#ef4444'
}

export function StationDetailPage() {
  const { id } = useParams<{ id: string }>()

  const { data: station } = useQuery({
    queryKey: ['station', id],
    queryFn: () => api.stations.get(id!),
  })
  const { data: history } = useQuery({
    queryKey: ['station-history', id, '24h'],
    queryFn: () => {
      const to = new Date().toISOString()
      const from = new Date(Date.now() - 24 * 3600_000).toISOString()
      return api.stations.history(id!, from, to)
    },
  })
  const { data: stationBikes } = useQuery({
    queryKey: ['station-bikes', id],
    queryFn: () => api.stations.bikes(id!),
    refetchInterval: 60_000,
  })
  const { data: trips } = useQuery({
    queryKey: ['station-trips', id],
    queryFn: () => api.trips.list({ station_id: id, limit: 20 }),
  })

  const chartData = (history ?? []).map((h) => ({
    time: new Date(h.time).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' }),
    bikes: h.num_bikes_available,
    docks: h.num_docks_available,
  })).reverse()

  return (
    <div style={S.page}>
      <Link to="/" style={S.back}>← Retour à la carte</Link>
      <div style={S.title}>{station?.name ?? id}</div>
      <div style={S.meta}>
        Capacité totale : {station?.capacity ?? '—'} emplacements
        {station?.is_virtual_station && ' · Station virtuelle'}
        {station?.is_charging_station && ' · Recharge'}
      </div>

      <div style={S.cards}>
        <div style={S.card}>
          <div style={S.cardLabel}>Vélos disponibles</div>
          <div style={S.cardValue}>{station?.num_bikes_available ?? '—'}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Docks libres</div>
          <div style={S.cardValue}>{station?.num_docks_available ?? '—'}</div>
        </div>
      </div>

      <div style={S.section}>
        <div style={S.sectionTitle}>Occupation — 24 dernières heures</div>
        <div style={{ background: '#1e293b', borderRadius: 8, padding: 16 }}>
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
              <XAxis dataKey="time" tick={{ fill: '#64748b', fontSize: 10 }} interval="preserveStartEnd" />
              <YAxis tick={{ fill: '#64748b', fontSize: 11 }} />
              <Tooltip contentStyle={{ background: '#0f172a', border: '1px solid #334155' }} />
              <Area type="monotone" dataKey="bikes" stroke="#22c55e" fill="#22c55e22" name="Vélos dispo" />
              <Area type="monotone" dataKey="docks" stroke="#38bdf8" fill="#38bdf822" name="Docks libres" />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Bikes currently docked */}
      <div style={S.section}>
        <div style={S.sectionTitle}>
          Vélos présents
          {stationBikes && stationBikes.length > 0 && (
            <span style={{ marginLeft: 8, fontSize: 13, fontWeight: 400, color: '#64748b' }}>
              ({stationBikes.length})
            </span>
          )}
        </div>
        {(stationBikes ?? []).length === 0 ? (
          <div style={{ color: '#64748b', fontSize: 13 }}>Aucun vélo actuellement présent</div>
        ) : (
          <table style={S.table}>
            <thead>
              <tr>
                <th style={S.th}>Vélo</th>
                <th style={S.th}>Type</th>
                <th style={S.th}>Batterie</th>
                <th style={S.th}>Santé (30j)</th>
              </tr>
            </thead>
            <tbody>
              {(stationBikes ?? []).map((b) => (
                <tr key={b.bike_id}>
                  <td style={S.td}>
                    <Link
                      to={`/bikes/${b.bike_id}`}
                      style={{ color: '#38bdf8', textDecoration: 'none', fontFamily: 'monospace' }}
                    >
                      {b.bike_id.slice(0, 8)}…
                    </Link>
                  </td>
                  <td style={{ ...S.td, color: '#64748b', fontSize: 11 }}>
                    {b.vehicle_type_id}
                  </td>
                  <td style={S.td}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <div style={{
                        width: 60, height: 8, borderRadius: 4,
                        background: '#1e293b', border: '1px solid #334155', overflow: 'hidden',
                      }}>
                        <div style={{
                          width: `${b.battery_pct}%`, height: '100%',
                          background: batteryBarColor(b.battery_pct),
                          borderRadius: 4, transition: 'width 0.3s',
                        }} />
                      </div>
                      <span style={{ color: batteryBarColor(b.battery_pct), fontWeight: 600, fontSize: 12 }}>
                        {b.battery_pct}%
                      </span>
                    </div>
                  </td>
                  <td style={S.td}>
                    <span style={{
                      color: healthColor(b.health_score),
                      fontWeight: 600, fontSize: 12,
                    }}>
                      {b.health_score}/100
                    </span>
                    <span style={{ color: '#64748b', fontSize: 11, marginLeft: 6 }}>
                      {b.health_label}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div style={S.section}>
        <div style={S.sectionTitle}>Trajets récents depuis/vers cette station</div>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>Vélo</th>
              <th style={S.th}>Début</th>
              <th style={S.th}>Durée</th>
              <th style={S.th}>Distance</th>
            </tr>
          </thead>
          <tbody>
            {(trips ?? []).map((t) => (
              <tr key={t.id}>
                <td style={S.td}>
                  <Link to={`/bikes/${t.bike_id}`} style={{ color: '#38bdf8', textDecoration: 'none' }}>
                    {t.bike_id.slice(0, 8)}…
                  </Link>
                </td>
                <td style={S.td}>
                  {new Date(t.start_time).toLocaleString('fr-FR')}
                </td>
                <td style={S.td}>
                  {t.duration_minutes != null ? `${Math.round(t.duration_minutes)} min` : '—'}
                </td>
                <td style={S.td}>
                  {t.distance_meters != null ? `${(t.distance_meters / 1000).toFixed(1)} km` : '—'}
                </td>
              </tr>
            ))}
            {(trips ?? []).length === 0 && (
              <tr><td colSpan={4} style={{ padding: '8px 10px', color: '#64748b' }}>Aucun trajet</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
