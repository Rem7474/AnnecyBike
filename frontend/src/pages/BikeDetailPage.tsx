import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts'
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

function fmt(n?: number, unit = '') {
  if (n === undefined || n === null) return '—'
  return `${Math.round(n)}${unit}`
}

export function BikeDetailPage() {
  const { id } = useParams<{ id: string }>()

  const { data: bike } = useQuery({ queryKey: ['bike', id], queryFn: () => api.bikes.get(id!) })
  const { data: stats } = useQuery({ queryKey: ['bike-stats', id], queryFn: () => api.bikes.stats(id!) })
  const { data: history } = useQuery({
    queryKey: ['bike-history', id, '7d'],
    queryFn: () => {
      const to = new Date().toISOString()
      const from = new Date(Date.now() - 7 * 86400_000).toISOString()
      return api.bikes.history(id!, from, to, '1h')
    },
  })
  const { data: trips } = useQuery({
    queryKey: ['bike-trips', id],
    queryFn: () => api.bikes.trips(id!, 20),
  })

  const chartData = (history ?? []).map((s) => ({
    time: new Date(s.time).toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }),
    battery: Math.round((s.current_range_meters / 45000) * 100),
  })).reverse()

  return (
    <div style={S.page}>
      <Link to="/" style={S.back}>← Retour à la carte</Link>
      <div style={S.title}>
        Vélo <code style={{ fontSize: 16, color: '#38bdf8' }}>{id?.slice(0, 8)}…</code>
      </div>
      <div style={{ fontSize: 13, color: '#64748b' }}>Type : {bike?.vehicle_type_id ?? '—'}</div>

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
        <div style={S.card}>
          <div style={S.cardLabel}>Hors service</div>
          <div style={S.cardValue}>{fmt(stats?.disabled_pct)}%</div>
        </div>
      </div>

      <div style={S.section}>
        <div style={S.sectionTitle}>Niveau de batterie — 7 derniers jours</div>
        <div style={{ background: '#1e293b', borderRadius: 8, padding: 16 }}>
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
              <XAxis dataKey="time" tick={{ fill: '#64748b', fontSize: 10 }} interval="preserveStartEnd" />
              <YAxis domain={[0, 100]} tick={{ fill: '#64748b', fontSize: 11 }} unit="%" />
              <Tooltip
                contentStyle={{ background: '#0f172a', border: '1px solid #334155' }}
                formatter={(v: number) => [`${v}%`, 'Batterie']}
              />
              <Line type="monotone" dataKey="battery" stroke="#38bdf8" dot={false} strokeWidth={2} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div style={S.section}>
        <div style={S.sectionTitle}>Derniers trajets</div>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>Départ</th>
              <th style={S.th}>Durée</th>
              <th style={S.th}>Distance</th>
              <th style={S.th}>Batterie consommée</th>
            </tr>
          </thead>
          <tbody>
            {(trips ?? []).map((t) => (
              <tr key={t.id}>
                <td style={S.td}>{new Date(t.start_time).toLocaleString('fr-FR')}</td>
                <td style={S.td}>{fmt(t.duration_minutes)} min</td>
                <td style={S.td}>{t.distance_meters ? `${(t.distance_meters / 1000).toFixed(1)} km` : '—'}</td>
                <td style={S.td}>{t.battery_delta !== undefined ? `${Math.round((Math.abs(t.battery_delta) / 45000) * 100)}%` : '—'}</td>
              </tr>
            ))}
            {(trips ?? []).length === 0 && (
              <tr><td style={S.td} colSpan={4}>Aucun trajet enregistré</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
