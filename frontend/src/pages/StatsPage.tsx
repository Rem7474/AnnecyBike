import { useQuery } from '@tanstack/react-query'
import {
  BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid,
  PieChart, Pie, Cell,
} from 'recharts'
import { useNavigate } from 'react-router-dom'
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

function fmtDate(iso: unknown) {
  if (typeof iso !== 'string' || iso.length < 10) return String(iso ?? '')
  const parts = iso.slice(0, 10).split('-')
  return `${parts[2]}/${parts[1]}`
}

const tooltipStyle = {
  contentStyle: { background: '#0f172a', border: '1px solid #334155', borderRadius: 6, fontSize: 12 },
  itemStyle: { color: '#f1f5f9' },
  labelStyle: { color: '#94a3b8', marginBottom: 4 },
  cursor: { fill: 'rgba(255,255,255,0.05)' },
}

export function StatsPage() {
  const navigate = useNavigate()
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
            <BarChart data={tripsPerDay ?? []} margin={{ left: 0, right: 8, top: 4, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
              <XAxis
                dataKey="date"
                tick={{ fill: '#64748b', fontSize: 10 }}
                tickFormatter={fmtDate}
                interval={4}
              />
              <YAxis tick={{ fill: '#64748b', fontSize: 11 }} width={32} allowDecimals={false} />
              <Tooltip
                {...tooltipStyle}
                labelFormatter={fmtDate}
                formatter={(v: number) => [v, 'Trajets']}
              />
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
                cy="45%"
                outerRadius={72}
                label={({ range, percent }) => `${range} (${(percent * 100).toFixed(0)}%)`}
                labelLine={{ stroke: '#475569' }}
              >
                {(battery ?? []).map((_, i) => (
                  <Cell key={i} fill={BATTERY_COLORS[i % BATTERY_COLORS.length]} />
                ))}
              </Pie>
              <Tooltip
                {...tooltipStyle}
                formatter={(v: number, name: string) => [`${v} vélos`, name]}
              />
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
              <tr
                key={s.station_id}
                onClick={() => navigate(`/stations/${s.station_id}`)}
                style={{ cursor: 'pointer' }}
                onMouseEnter={(e) => (e.currentTarget.style.background = '#263348')}
                onMouseLeave={(e) => (e.currentTarget.style.background = '')}
              >
                <td style={S.td}>{i + 1}</td>
                <td style={{ ...S.td, color: '#38bdf8' }}>{s.name}</td>
                <td style={S.td}>{s.trip_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
