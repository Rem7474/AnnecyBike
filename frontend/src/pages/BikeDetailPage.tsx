import { CSSProperties, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import type { BikeSnapshot } from '../types'

const S: Record<string, CSSProperties> = {
  page: { padding: 24, maxWidth: 900, margin: '0 auto' },
  back: { color: '#94a3b8', textDecoration: 'none', fontSize: 13 },
  title: { fontSize: 22, fontWeight: 700, margin: '12px 0 4px' },
  notice: {
    background: '#1e293b', border: '1px solid #334155', borderRadius: 8,
    padding: '10px 16px', fontSize: 12, color: '#64748b', marginBottom: 20,
  },
  cards: { display: 'flex', gap: 12, flexWrap: 'wrap', margin: '16px 0' },
  card: { background: '#1e293b', borderRadius: 8, padding: '12px 20px', minWidth: 140 },
  cardLabel: { fontSize: 11, color: '#64748b', textTransform: 'uppercase', letterSpacing: 1 },
  cardValue: { fontSize: 24, fontWeight: 700, marginTop: 4 },
  infoBlock: {
    background: '#1e293b', borderRadius: 8, padding: '16px 20px',
    marginBottom: 20, display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 16,
  },
  infoLabel: { fontSize: 11, color: '#64748b', textTransform: 'uppercase', letterSpacing: 1, marginBottom: 4 },
  infoValue: { fontSize: 13, color: '#f1f5f9' },
  section: { marginTop: 24 },
  sectionTitle: { fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#94a3b8' },
  table: { width: '100%', borderCollapse: 'collapse', fontSize: 13 },
  th: { textAlign: 'left', padding: '6px 10px', color: '#64748b', borderBottom: '1px solid #1e293b' },
  td: { padding: '8px 10px', borderBottom: '1px solid #1e293b' },
}

function batteryColor(pct: number) {
  if (pct >= 50) return '#22c55e'
  if (pct >= 20) return '#f97316'
  return '#ef4444'
}

function fmt(iso: string) {
  return new Date(iso).toLocaleString('fr-FR', {
    day: '2-digit', month: '2-digit', year: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

function elapsed(iso: string) {
  const ms = Date.now() - new Date(iso).getTime()
  const m = Math.floor(ms / 60_000)
  if (m < 1) return "à l'instant"
  if (m < 60) return `${m} min`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} h ${m % 60 > 0 ? `${m % 60} min` : ''}`
  return `${Math.floor(h / 24)} j ${h % 24} h`
}

export function BikeDetailPage() {
  const { id } = useParams<{ id: string }>()

  const { data: bike } = useQuery({ queryKey: ['bike', id], queryFn: () => api.bikes.get(id!) })
  const { data: trips } = useQuery({ queryKey: ['bike-trips', id], queryFn: () => api.bikes.trips(id!, 20) })

  const { data: trajectory } = useQuery({
    queryKey: ['bike-trajectory', id],
    queryFn: () => {
      const to = new Date().toISOString()
      const from = new Date(Date.now() - 24 * 3600_000).toISOString()
      return api.bikes.history(id!, from, to, 'raw')
    },
  })

  // trajectory is DESC (most recent first); last element = oldest
  const latestSnapshot: BikeSnapshot | null =
    trajectory && trajectory.length > 0 ? trajectory[0] : null

  const batteryPct = latestSnapshot
    ? Math.round((latestSnapshot.current_range_meters / 45000) * 100)
    : (bike?.current_battery_pct ?? null)

  const isDisabled = latestSnapshot?.is_disabled ?? bike?.is_currently_disabled ?? false
  const autonomyKm = latestSnapshot
    ? Math.round(latestSnapshot.current_range_meters / 1000)
    : batteryPct !== null ? Math.round((batteryPct / 100) * 45) : null

  // Arrival time at current station: walk back through DESC snapshots while station_id matches
  const currentStationId = latestSnapshot?.station_id ?? bike?.current_station_id ?? null
  const arrivalTime = useMemo(() => {
    if (!currentStationId || !trajectory || trajectory.length === 0) return null
    // trajectory[0] is most recent; find the oldest contiguous snapshot at this station
    let earliest: string | null = null
    for (const s of trajectory) {
      if (s.station_id === currentStationId) earliest = s.time
      else break
    }
    return earliest
  }, [trajectory, currentStationId])

  const stationName = latestSnapshot?.station_id
    ? (bike?.current_station_name ?? latestSnapshot.station_id)
    : bike?.current_station_name ?? null

  return (
    <div style={S.page}>
      <Link to="/" style={S.back}>← Retour à la carte</Link>
      <div style={S.title}>
        Vélo <code style={{ fontSize: 16, color: '#38bdf8' }}>{id?.slice(0, 8)}…</code>
      </div>
      <div style={{ fontSize: 13, color: '#64748b', marginBottom: 12 }}>
        Type : {bike?.vehicle_type_id ?? '—'}
      </div>

      <div style={S.notice}>
        ⚠ Les identifiants vélo sont temporaires (rotation après chaque trajet, spec GBFS v2.0).
        Cette page affiche uniquement la <strong>session courante</strong>.{' '}
        {bike?.physical_bike_id
          ? <Link to={`/physical-bikes/${bike.physical_bike_id}`} style={{ color: '#38bdf8' }}>
              Voir l'historique complet du vélo physique →
            </Link>
          : "Assignez ce vélo à sa fiche physique depuis la page station."}
      </div>

      {/* Current state cards */}
      <div style={S.cards}>
        <div style={{ ...S.card, borderLeft: `4px solid ${batteryColor(batteryPct ?? 0)}` }}>
          <div style={S.cardLabel}>Batterie</div>
          <div style={{ ...S.cardValue, color: batteryColor(batteryPct ?? 0) }}>
            {batteryPct !== null ? `${batteryPct}%` : '—'}
          </div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Statut</div>
          <div style={{ ...S.cardValue, fontSize: 16, marginTop: 8 }}>
            {isDisabled
              ? <span style={{ color: '#ef4444' }}>Hors service</span>
              : <span style={{ color: '#22c55e' }}>Disponible</span>}
          </div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Autonomie</div>
          <div style={S.cardValue}>
            {autonomyKm !== null ? `${autonomyKm} km` : '—'}
          </div>
        </div>
      </div>

      {/* Info block: station + timing */}
      <div style={S.infoBlock}>
        <div>
          <div style={S.infoLabel}>Station actuelle</div>
          <div style={S.infoValue}>
            {currentStationId
              ? <Link to={`/stations/${currentStationId}`} style={{ color: '#38bdf8', textDecoration: 'none' }}>
                  {stationName ?? currentStationId}
                </Link>
              : <span style={{ color: '#64748b' }}>En transit / hors station</span>}
          </div>
        </div>
        <div>
          <div style={S.infoLabel}>Présent depuis</div>
          <div style={S.infoValue}>
            {arrivalTime
              ? <><span style={{ color: '#34d399' }}>{elapsed(arrivalTime)}</span>{' '}
                  <span style={{ color: '#64748b', fontSize: 11 }}>({fmt(arrivalTime)})</span></>
              : <span style={{ color: '#64748b' }}>—</span>}
          </div>
        </div>
        <div>
          <div style={S.infoLabel}>Première apparition (session)</div>
          <div style={S.infoValue}>
            {bike?.first_seen
              ? <>{fmt(bike.first_seen)}<span style={{ color: '#64748b', fontSize: 11, marginLeft: 6 }}>
                  (il y a {elapsed(bike.first_seen)})
                </span></>
              : '—'}
          </div>
        </div>
        <div>
          <div style={S.infoLabel}>Dernière mise à jour</div>
          <div style={S.infoValue}>
            {bike?.last_seen
              ? <>{fmt(bike.last_seen)}<span style={{ color: '#64748b', fontSize: 11, marginLeft: 6 }}>
                  (il y a {elapsed(bike.last_seen)})
                </span></>
              : '—'}
          </div>
        </div>
      </div>

      {/* Trips */}
      <div style={S.section}>
        <div style={S.sectionTitle}>Trajets (session courante)</div>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>Départ</th>
              <th style={S.th}>Durée</th>
              <th style={S.th}>Distance</th>
              <th style={S.th}>Batterie</th>
              <th style={S.th}>Itinéraire</th>
            </tr>
          </thead>
          <tbody>
            {(trips ?? []).map((t, i, arr) => {
              // Trips are DESC: arr[i+1] is the trip just before this one.
              // Arrival of previous trip should match departure of this trip.
              const prevTrip = arr[i + 1]
              const teleport = prevTrip &&
                prevTrip.end_station_name &&
                t.start_station_name &&
                prevTrip.end_station_name !== t.start_station_name
              return (
                <tr key={t.id} style={teleport ? { background: '#2d1f0f' } : undefined}>
                  <td style={S.td}>{fmt(t.start_time)}</td>
                  <td style={S.td}>{t.duration_minutes != null ? `${Math.round(t.duration_minutes)} min` : '—'}</td>
                  <td style={S.td}>{t.distance_meters != null ? `${(t.distance_meters / 1000).toFixed(1)} km` : '—'}</td>
                  <td style={S.td}>
                    {t.battery_delta != null
                      ? <span style={{ color: '#f97316' }}>−{Math.round((Math.abs(t.battery_delta) / 45000) * 100)}%</span>
                      : '—'}
                  </td>
                  <td style={{ ...S.td, fontSize: 12 }}>
                    {teleport && (
                      <span title={`Téléportation détectée : arrivée précédente "${prevTrip.end_station_name}" ≠ départ actuel "${t.start_station_name}"`}
                        style={{ color: '#f97316', marginRight: 6, cursor: 'help' }}>⚠</span>
                    )}
                    <span style={{ color: '#64748b' }}>
                      {t.start_station_name ?? '?'} → {t.end_station_name ?? '?'}
                    </span>
                  </td>
                </tr>
              )
            })}
            {(trips ?? []).length === 0 && (
              <tr><td colSpan={5} style={{ ...S.td, color: '#64748b' }}>Aucun trajet pour cette session</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
