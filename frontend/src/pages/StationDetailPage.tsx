import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AreaChart, Area, BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid, Legend } from 'recharts'
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

function batteryColor(pct: number) {
  if (pct >= 60) return '#22c55e'
  if (pct >= 30) return '#f97316'
  return '#ef4444'
}

export function StationDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [bikeHistoryHours, setBikeHistoryHours] = useState(48)
  const [assigningBike, setAssigningBike] = useState<string | null>(null)
  const [fleetInput, setFleetInput] = useState('')
  const [assignError, setAssignError] = useState<string | null>(null)

  const assignMutation = useMutation({
    mutationFn: ({ bikeId, fleet }: { bikeId: string; fleet: string }) =>
      api.bikes.assign(bikeId, fleet),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ['station-bikes', id] })
      setAssigningBike(null)
      setFleetInput('')
      setAssignError(null)
      navigate(`/physical-bikes/${data.physical_bike_id}`)
    },
    onError: (err: Error) => setAssignError(err.message),
  })

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
  const { data: bikeHistory } = useQuery({
    queryKey: ['station-bike-history', id, bikeHistoryHours],
    queryFn: () => api.stations.bikeHistory(id!, bikeHistoryHours),
  })

  const [hourlyDays, setHourlyDays] = useState(90)
  const { data: hourlyStats } = useQuery({
    queryKey: ['station-hourly', id, hourlyDays],
    queryFn: () => api.stations.hourlyStats(id!, hourlyDays),
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

      {/* Hourly bike availability profile */}
      <div style={S.section}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
          <div style={S.sectionTitle}>Profil horaire moyen</div>
          <div style={{ display: 'flex', gap: 6, marginLeft: 'auto' }}>
            {[30, 60, 90].map(d => (
              <button key={d} onClick={() => setHourlyDays(d)} style={{
                padding: '3px 10px', borderRadius: 6, fontSize: 11, cursor: 'pointer', border: 'none',
                background: hourlyDays === d ? '#38bdf8' : '#1e293b',
                color: hourlyDays === d ? '#0f172a' : '#64748b',
                fontWeight: hourlyDays === d ? 700 : 400,
              }}>{d}j</button>
            ))}
          </div>
        </div>
        {(hourlyStats ?? []).length === 0 ? (
          <div style={{ color: '#64748b', fontSize: 13 }}>Pas encore assez de données</div>
        ) : (
          <div style={{ background: '#1e293b', borderRadius: 8, padding: 16 }}>
            <ResponsiveContainer width="100%" height={220}>
              <BarChart data={(hourlyStats ?? []).map(h => ({
                h: `${String(h.hour).padStart(2, '0')}h`,
                Semaine: h.avg_weekday,
                'Week-end': h.avg_weekend,
              }))} barCategoryGap="20%">
                <CartesianGrid strokeDasharray="3 3" stroke="#334155" vertical={false} />
                <XAxis dataKey="h" tick={{ fill: '#64748b', fontSize: 10 }} interval={1} />
                <YAxis tick={{ fill: '#64748b', fontSize: 11 }} allowDecimals={false} />
                <Tooltip
                  contentStyle={{ background: '#0f172a', border: '1px solid #334155', fontSize: 12 }}
                  formatter={(v: number) => v.toFixed(1)}
                />
                <Legend wrapperStyle={{ fontSize: 12, color: '#94a3b8' }} />
                <Bar dataKey="Semaine" fill="#38bdf8" radius={[2, 2, 0, 0]} />
                <Bar dataKey="Week-end" fill="#f97316" radius={[2, 2, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
            <div style={{ fontSize: 11, color: '#475569', marginTop: 8, textAlign: 'right' }}>
              Moyenne sur {hourlyDays} jours · heure locale (Europe/Paris)
            </div>
          </div>
        )}
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
                <th style={S.th}>Vélo physique</th>
                <th style={S.th}>ID GBFS</th>
                <th style={S.th}>Type</th>
                <th style={S.th}>Batterie</th>
                <th style={S.th}>Santé (30j)</th>
              </tr>
            </thead>
            <tbody>
              {(stationBikes ?? []).map((b) => (
                <tr key={b.bike_id}>
                  {/* Physical bike assignment cell */}
                  <td style={S.td}>
                    {b.physical_bike_id ? (
                      <Link
                        to={`/physical-bikes/${b.physical_bike_id}`}
                        style={{ color: '#38bdf8', textDecoration: 'none', fontWeight: 700, fontFamily: 'monospace' }}
                      >
                        {b.fleet_number ? `#${b.fleet_number}` : `vélo ${b.physical_bike_id}`}
                      </Link>
                    ) : assigningBike === b.bike_id ? (
                      <span style={{ display: 'flex', gap: 4, alignItems: 'center', flexWrap: 'wrap' }}>
                        <input
                          autoFocus
                          style={{
                            background: '#0f172a', border: '1px solid #38bdf8', borderRadius: 4,
                            color: '#f1f5f9', fontSize: 12, padding: '3px 7px', outline: 'none', width: 70,
                          }}
                          placeholder="N° flotte"
                          value={fleetInput}
                          onChange={e => { setFleetInput(e.target.value); setAssignError(null) }}
                          onKeyDown={e => {
                            if (e.key === 'Enter' && fleetInput.trim()) assignMutation.mutate({ bikeId: b.bike_id, fleet: fleetInput.trim() })
                            if (e.key === 'Escape') { setAssigningBike(null); setFleetInput('') }
                          }}
                        />
                        <button
                          onClick={() => assignMutation.mutate({ bikeId: b.bike_id, fleet: fleetInput.trim() })}
                          disabled={!fleetInput.trim() || assignMutation.isPending}
                          style={{ background: '#38bdf8', color: '#0f172a', border: 'none', borderRadius: 4, padding: '3px 8px', fontSize: 11, fontWeight: 700, cursor: 'pointer' }}
                        >✓</button>
                        <button
                          onClick={() => { setAssigningBike(null); setFleetInput(''); setAssignError(null) }}
                          style={{ background: 'none', border: '1px solid #334155', borderRadius: 4, color: '#94a3b8', padding: '3px 6px', fontSize: 11, cursor: 'pointer' }}
                        >✕</button>
                        {assignError && <span style={{ color: '#ef4444', fontSize: 11 }}>Erreur</span>}
                      </span>
                    ) : (
                      <button
                        onClick={() => { setAssigningBike(b.bike_id); setFleetInput('') }}
                        style={{ background: 'none', border: '1px dashed #334155', borderRadius: 4, color: '#64748b', fontSize: 11, padding: '3px 8px', cursor: 'pointer' }}
                      >
                        + Assigner
                      </button>
                    )}
                  </td>
                  {/* GBFS bike_id */}
                  <td style={S.td}>
                    <Link
                      to={`/bikes/${b.bike_id}`}
                      style={{ color: '#475569', textDecoration: 'none', fontFamily: 'monospace', fontSize: 11 }}
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

      {/* Bike visit history */}
      <div style={S.section}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
          <div style={S.sectionTitle}>Historique des vélos</div>
          <div style={{ display: 'flex', gap: 6, marginLeft: 'auto' }}>
            {[24, 48, 168].map((h) => (
              <button key={h} onClick={() => setBikeHistoryHours(h)} style={{
                padding: '3px 10px', borderRadius: 6, fontSize: 11, cursor: 'pointer', border: 'none',
                background: bikeHistoryHours === h ? '#38bdf8' : '#1e293b',
                color: bikeHistoryHours === h ? '#0f172a' : '#64748b',
                fontWeight: bikeHistoryHours === h ? 700 : 400,
              }}>
                {h === 168 ? '7j' : `${h}h`}
              </button>
            ))}
          </div>
        </div>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>Vélo</th>
              <th style={S.th}>Arrivée</th>
              <th style={S.th}>Départ</th>
              <th style={S.th}>Durée</th>
              <th style={S.th}>Batterie à l'arrivée</th>
            </tr>
          </thead>
          <tbody>
            {(bikeHistory ?? []).length === 0 && (
              <tr><td colSpan={5} style={{ padding: '8px 10px', color: '#64748b' }}>Aucune visite enregistrée</td></tr>
            )}
            {(bikeHistory ?? []).map((v, i) => (
              <tr key={i}>
                <td style={S.td}>
                  <Link to={`/bikes/${v.bike_id}`}
                    style={{ color: '#38bdf8', textDecoration: 'none', fontFamily: 'monospace', fontSize: 12 }}>
                    {v.bike_id.slice(0, 8)}…
                  </Link>
                </td>
                <td style={{ ...S.td, fontSize: 12, whiteSpace: 'nowrap' as const }}>
                  {new Date(v.arrived_at).toLocaleString('fr-FR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })}
                </td>
                <td style={{ ...S.td, fontSize: 12, whiteSpace: 'nowrap' as const }}>
                  {v.still_present
                    ? <span style={{ color: '#22c55e' }}>● Présent</span>
                    : v.departed_at
                      ? new Date(v.departed_at).toLocaleString('fr-FR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
                      : '—'
                  }
                </td>
                <td style={{ ...S.td, fontSize: 12 }}>
                  {Math.round(v.duration_minutes)} min
                </td>
                <td style={S.td}>
                  <span style={{ color: batteryColor(Math.round((v.battery_arrival / 45000) * 100)), fontWeight: 600, fontSize: 12 }}>
                    {Math.round((v.battery_arrival / 45000) * 100)}%
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
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
