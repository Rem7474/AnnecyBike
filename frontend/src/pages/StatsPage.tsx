import { useQuery } from '@tanstack/react-query'
import {
  BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid,
  PieChart, Pie, Cell, Legend,
} from 'recharts'
import { api } from '../api'

const S: Record<string, React.CSSProperties> = {
  page: { padding: 24, maxWidth: 1200, margin: '0 auto' },
  title: { fontSize: 22, fontWeight: 700, marginBottom: 24 },
  grid: { display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: 12, marginBottom: 32 },
  card: { background: '#1e293b', borderRadius: 8, padding: '14px 20px' },
  cardLabel: { fontSize: 11, color: '#64748b', textTransform: 'uppercase', letterSpacing: 1 },
  cardValue: { fontSize: 28, fontWeight: 700, marginTop: 4 },
  charts: { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20 },
  chart: { background: '#1e293b', borderRadius: 8, padding: 16 },
  chartTitle: { fontSize: 14, fontWeight: 600, marginBottom: 12, color: '#94a3b8' },
  table: { width: '100%', borderCollapse: 'collapse', fontSize: 13 },
  th: { textAlign: 'left', padding: '6px 10px', color: '#64748b', borderBottom: '1px solid #334155' },
  td: { padding: '8px 10px', borderBottom: '1px solid #1e293b' },
}

const BATTERY_COLORS = [
  '#ef4444', '#f97316', '#f97316', '#eab308',
  '#eab308', '#22c55e', '#22c55e', '#22c55e', '#22c55e', '#22c55e',
]

export function StatsPage() {
  const { data: fleet } = useQuery({ queryKey: ['stats-fleet'], queryFn: api.stats.fleet, refetchInterval: 60_000 })
  const { data: tripsPerDay } = useQuery({ queryKey: ['stats-trips-day'], queryFn: () => api.stats.tripsPerDay(30) })
  const { data: battery } = useQuery({ queryKey: ['stats-battery'], queryFn: api.stats.batteryDistribution, refetchInterval: 60_000 })
  const { data: busiest } = useQuery({ queryKey: ['stats-busiest'], queryFn: () => api.stats.busiestStations(10) })

  return (
    <div style={S.page}>
      <div style={S.title}>Statistiques de la flotte</div>

      <div style={S.grid}>
        <div style={S.card}>
          <div style={S.cardLabel}>Total vélos</div>
          <div style={S.cardValue}>{fleet?.total_bikes ?? '—'}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Disponibles</div>
          <div style={{ ...S.cardValue, color: '#22c55e' }}>{fleet?.active_now ?? '—'}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Hors service</div>
          <div style={{ ...S.cardValue, color: '#ef4444' }}>{fleet?.disabled_now ?? '—'}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Réservés</div>
          <div style={{ ...S.cardValue, color: '#8b5cf6' }}>{fleet?.reserved_now ?? '—'}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Trajets aujourd'hui</div>
          <div style={S.cardValue}>{fleet?.trips_today ?? '—'}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Trajets cette semaine</div>
          <div style={S.cardValue}>{fleet?.trips_week ?? '—'}</div>
        </div>
      </div>

      <div style={S.charts}>
        <div style={S.chart}>
          <div style={S.chartTitle}>Trajets par jour — 30 derniers jours</div>
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={tripsPerDay ?? []}>
              <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
              <XAxis dataKey="date" tick={{ fill: '#64748b', fontSize: 10 }} interval="preserveStartEnd" />
              <YAxis tick={{ fill: '#64748b', fontSize: 11 }} />
              <Tooltip contentStyle={{ background: '#0f172a', border: '1px solid #334155' }} />
              <Bar dataKey="count" fill="#38bdf8" radius={[3, 3, 0, 0]} name="Trajets" />
            </BarChart>
          </ResponsiveContainer>
        </div>

        <div style={S.chart}>
          <div style={S.chartTitle}>Distribution de la batterie (état actuel)</div>
          <ResponsiveContainer width="100%" height={220}>
            <PieChart>
              <Pie
                data={battery ?? []}
                dataKey="count"
                nameKey="range"
                cx="50%"
                cy="50%"
                outerRadius={80}
                label={({ range, percent }) => `${range} (${(percent * 100).toFixed(0)}%)`}
              >
                {(battery ?? []).map((_, i) => (
                  <Cell key={i} fill={BATTERY_COLORS[i % BATTERY_COLORS.length]} />
                ))}
              </Pie>
              <Tooltip contentStyle={{ background: '#0f172a', border: '1px solid #334155' }} />
            </PieChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div style={{ ...S.chart, marginTop: 20 }}>
        <div style={S.chartTitle}>Stations les plus actives (7 derniers jours)</div>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>#</th>
              <th style={S.th}>Station</th>
              <th style={S.th}>Trajets</th>
            </tr>
          </thead>
          <tbody>
            {(busiest ?? []).map((s, i) => (
              <tr key={s.station_id}>
                <td style={S.td}>{i + 1}</td>
                <td style={S.td}>{s.name}</td>
                <td style={S.td}>{s.trip_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
