import { CSSProperties } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { MapContainer, TileLayer, Polyline, CircleMarker, Popup } from 'react-leaflet'
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

export function BikeDetailPage() {
  const { id } = useParams<{ id: string }>()

  const { data: bike } = useQuery({ queryKey: ['bike', id], queryFn: () => api.bikes.get(id!) })
  const { data: trips } = useQuery({ queryKey: ['bike-trips', id], queryFn: () => api.bikes.trips(id!, 5) })

  // Current session GPS track (raw, 24h) — also used for live state
  const { data: trajectory } = useQuery({
    queryKey: ['bike-trajectory', id],
    queryFn: () => {
      const to = new Date().toISOString()
      const from = new Date(Date.now() - 24 * 3600_000).toISOString()
      return api.bikes.history(id!, from, to, 'raw')
    },
  })

  const trajectoryPoints = (trajectory ?? [])
    .filter((s: BikeSnapshot) => s.station_id === null && s.lat !== 0 && s.lon !== 0)
    .map((s: BikeSnapshot) => [s.lat, s.lon] as [number, number])
    .reverse()

  // Latest snapshot for current battery/status
  const latestSnapshot: BikeSnapshot | null =
    trajectory && trajectory.length > 0 ? trajectory[trajectory.length - 1] : null

  const batteryPct = latestSnapshot
    ? Math.round((latestSnapshot.current_range_meters / 45000) * 100)
    : (bike?.current_battery_pct ?? null)

  const isDisabled = latestSnapshot?.is_disabled ?? bike?.is_currently_disabled ?? false
  const autonomyKm = latestSnapshot
    ? Math.round(latestSnapshot.current_range_meters / 1000)
    : batteryPct !== null ? Math.round((batteryPct / 100) * 45) : null

  const mapCenter: [number, number] = trajectoryPoints[0] ?? [45.899, 6.129]

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
        Cette page affiche uniquement la <strong>session courante</strong> — l'historique complet
        est consultable par station.
      </div>

      {bike?.physical_bike_id && (
        <div style={{ marginBottom: 16 }}>
          <Link to={`/physical-bikes/${bike.physical_bike_id}`} style={{ color: '#38bdf8', fontSize: 13 }}>
            → Voir l'historique complet du vélo physique #{bike.physical_bike_id}
          </Link>
        </div>
      )}

      {/* Current state */}
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

      {/* Trajectory map */}
      <div style={S.section}>
        <div style={S.sectionTitle}>Trajectoire (session courante)</div>
        <div style={{ borderRadius: 8, overflow: 'hidden', height: 300 }}>
          <MapContainer center={mapCenter} zoom={14} style={{ height: '100%' }} zoomControl={false}>
            <TileLayer url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png" attribution="" />
            {trajectoryPoints.length > 1 && (
              <Polyline positions={trajectoryPoints} pathOptions={{ color: '#38bdf8', weight: 2 }} />
            )}
            {trajectoryPoints.length > 0 && (
              <CircleMarker center={trajectoryPoints[trajectoryPoints.length - 1]} radius={5}
                pathOptions={{ color: '#38bdf8', fillColor: '#38bdf8', fillOpacity: 1 }}>
                <Popup>Dernière position connue</Popup>
              </CircleMarker>
            )}
          </MapContainer>
        </div>
      </div>

      {/* Trips this session */}
      <div style={S.section}>
        <div style={S.sectionTitle}>Trajets (session courante)</div>
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
                <td style={S.td}>{t.duration_minutes != null ? `${Math.round(t.duration_minutes)} min` : '—'}</td>
                <td style={S.td}>{t.distance_meters != null ? `${(t.distance_meters / 1000).toFixed(1)} km` : '—'}</td>
                <td style={S.td}>
                  {t.battery_delta != null
                    ? <span style={{ color: '#f97316' }}>−{Math.round((Math.abs(t.battery_delta) / 45000) * 100)}%</span>
                    : '—'}
                </td>
                <td style={{ ...S.td, fontSize: 12, color: '#64748b' }}>
                  {t.start_station_name ?? '?'} → {t.end_station_name ?? '?'}
                </td>
              </tr>
            ))}
            {(trips ?? []).length === 0 && (
              <tr><td colSpan={5} style={{ ...S.td, color: '#64748b' }}>Aucun trajet pour cette session</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
