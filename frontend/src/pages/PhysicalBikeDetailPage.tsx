import { CSSProperties, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid,
} from 'recharts'
import { api } from '../api'
import type { BikeSnapshot, Trip } from '../types'

const S: Record<string, CSSProperties> = {
  page: { padding: 24, maxWidth: 960, margin: '0 auto' },
  back: { color: '#94a3b8', textDecoration: 'none', fontSize: 13 },
  title: { fontSize: 22, fontWeight: 700, margin: '12px 0 4px' },
  subtitle: { fontSize: 13, color: '#64748b', marginBottom: 16 },
  cards: { display: 'flex', gap: 12, flexWrap: 'wrap', margin: '16px 0' },
  card: { background: '#1e293b', borderRadius: 8, padding: '12px 20px', minWidth: 140 },
  cardLabel: { fontSize: 11, color: '#64748b', textTransform: 'uppercase', letterSpacing: 1 },
  cardValue: { fontSize: 24, fontWeight: 700, marginTop: 4 },
  section: { marginTop: 28 },
  sectionTitle: { fontSize: 16, fontWeight: 600, marginBottom: 12, color: '#94a3b8' },
  table: { width: '100%', borderCollapse: 'collapse', fontSize: 13 },
  th: { textAlign: 'left', padding: '6px 10px', color: '#64748b', borderBottom: '1px solid #1e293b' },
  td: { padding: '8px 10px', borderBottom: '1px solid #1e293b' },
  editBox: {
    background: '#1e293b', borderRadius: 8, padding: '16px 20px', marginBottom: 20,
    display: 'flex', flexWrap: 'wrap', gap: 16, alignItems: 'flex-end',
  },
  fieldGroup: { display: 'flex', flexDirection: 'column', gap: 4 },
  label: { fontSize: 11, color: '#64748b', textTransform: 'uppercase', letterSpacing: 1 },
  input: {
    background: '#0f172a', border: '1px solid #334155', borderRadius: 6,
    color: '#f1f5f9', fontSize: 14, padding: '6px 10px', outline: 'none',
  },
  saveBtn: {
    background: '#38bdf8', color: '#0f172a', border: 'none', borderRadius: 6,
    padding: '7px 16px', fontSize: 13, fontWeight: 600, cursor: 'pointer',
  },
  cancelBtn: {
    background: 'none', border: '1px solid #334155', borderRadius: 6,
    color: '#94a3b8', padding: '7px 16px', fontSize: 13, cursor: 'pointer',
  },
  editBtn: {
    background: 'none', border: '1px solid #334155', borderRadius: 6,
    color: '#94a3b8', fontSize: 12, padding: '4px 10px', cursor: 'pointer', marginLeft: 8,
  },
  idsBox: {
    background: '#1e293b', borderRadius: 8, padding: '12px 16px',
    fontFamily: 'monospace', fontSize: 11, color: '#94a3b8',
    lineHeight: 1.8,
  },
  reassignRow: { display: 'flex', alignItems: 'center', gap: 8, marginTop: 12 },
  smallInput: {
    background: '#0f172a', border: '1px solid #334155', borderRadius: 6,
    color: '#f1f5f9', fontSize: 12, padding: '5px 8px', outline: 'none',
    fontFamily: 'monospace', width: 300,
  },
  smallBtn: {
    background: 'none', border: '1px solid #334155', borderRadius: 6,
    color: '#38bdf8', fontSize: 12, padding: '5px 10px', cursor: 'pointer',
  },
}

function batteryColor(pct: number) {
  if (pct >= 50) return '#22c55e'
  if (pct >= 20) return '#f97316'
  return '#ef4444'
}

function displayName(bike: { id: number; fleet_number?: string; custom_name?: string } | undefined, fallbackId: number) {
  if (!bike) return `#${String(fallbackId).padStart(6, '0')}`
  if (bike.custom_name) return bike.custom_name
  if (bike.fleet_number) return `Vélo ${bike.fleet_number}`
  return `#${String(bike.id).padStart(6, '0')}`
}

export function PhysicalBikeDetailPage() {
  const { id } = useParams<{ id: string }>()
  const numId = Number(id)
  const qc = useQueryClient()

  const [editing, setEditing] = useState(false)
  const [fleetInput, setFleetInput] = useState('')
  const [nameInput, setNameInput] = useState('')

  const [showAllIDs, setShowAllIDs] = useState(false)
  const [pullBikeId, setPullBikeId] = useState('')      // for "rattacher ici"
  const [moveBikeId, setMoveBikeId] = useState('')      // for "déplacer vers autre vélo"
  const [moveTarget, setMoveTarget] = useState('')
  const [reassignMsg, setReassignMsg] = useState<string | null>(null)

  const { data: bike } = useQuery({
    queryKey: ['physical-bike', id],
    queryFn: () => api.physicalBikes.get(numId),
  })

  const { data: trips } = useQuery({
    queryKey: ['physical-bike-trips', id],
    queryFn: () => api.physicalBikes.trips(numId, 50, 0),
  })

  const { data: history } = useQuery({
    queryKey: ['physical-bike-history', id],
    queryFn: () => {
      const to = new Date().toISOString()
      const from = new Date(Date.now() - 7 * 24 * 3600_000).toISOString()
      return api.physicalBikes.history(numId, from, to)
    },
  })

  const updateMutation = useMutation({
    mutationFn: (body: { fleet_number: string | null; custom_name: string | null }) =>
      api.physicalBikes.update(numId, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['physical-bike', id] })
      setEditing(false)
    },
  })

  const reassignMutation = useMutation({
    mutationFn: ({ bikeId, targetPid }: { bikeId: string; targetPid: number }) =>
      api.physicalBikes.reassign(bikeId, targetPid),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['physical-bike', id] })
      setReassignInput('')
      setReassignTarget('')
      setReassignMsg('Réaffectation effectuée.')
      setTimeout(() => setReassignMsg(null), 3000)
    },
    onError: (e: Error) => setReassignMsg(`Erreur : ${e.message}`),
  })

  const batteryData = (history ?? [])
    .filter((s: BikeSnapshot) => s.current_range_meters > 0)
    .map((s: BikeSnapshot) => ({
      time: new Date(s.time).toLocaleDateString('fr-FR', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }),
      pct: Math.min(100, Math.round((s.current_range_meters / 45000) * 100)),
    }))
    .reverse()

  const batteryPct = bike?.current_battery_pct ?? null
  const isDisabled = bike?.is_currently_disabled ?? false
  const knownIDs = bike?.known_bike_ids ?? []
  const visibleIDs = showAllIDs ? knownIDs : knownIDs.slice(0, 5)

  function startEdit() {
    setFleetInput(bike?.fleet_number ?? '')
    setNameInput(bike?.custom_name ?? '')
    setEditing(true)
  }

  function saveEdit() {
    updateMutation.mutate({
      fleet_number: fleetInput.trim() || null,
      custom_name: nameInput.trim() || null,
    })
  }

  function handlePull() {
    const bikeId = pullBikeId.trim()
    if (!bikeId) return
    reassignMutation.mutate({ bikeId, targetPid: numId })
    setPullBikeId('')
  }

  function handleMove() {
    const bikeId = moveBikeId.trim()
    const targetPid = Number(moveTarget.trim())
    if (!bikeId || !targetPid) return
    reassignMutation.mutate({ bikeId, targetPid })
    setMoveBikeId('')
    setMoveTarget('')
  }

  return (
    <div style={S.page}>
      <Link to="/" style={S.back}>← Retour à la carte</Link>

      <div style={{ display: 'flex', alignItems: 'center', gap: 8, margin: '12px 0 4px' }}>
        <div style={S.title}>
          {displayName(bike, numId)}
          {bike?.fleet_number && bike?.custom_name && (
            <span style={{ fontSize: 13, color: '#64748b', fontWeight: 400, marginLeft: 8 }}>
              ({bike.custom_name})
            </span>
          )}
        </div>
        <button style={S.editBtn} onClick={startEdit}>✏ Modifier</button>
      </div>

      <div style={S.subtitle}>
        Type : {bike?.vehicle_type_id ?? '—'}
        {bike?.first_seen && (
          <> · Premier vu le {new Date(bike.first_seen).toLocaleDateString('fr-FR')}</>
        )}
        {bike?.current_station_name && (
          <> · Station actuelle : <strong style={{ color: '#f1f5f9' }}>{bike.current_station_name}</strong></>
        )}
        {' '}&nbsp;·&nbsp;<span style={{ color: '#475569', fontFamily: 'monospace' }}>
          #{String(numId).padStart(6, '0')}
        </span>
      </div>

      {/* Inline edit form */}
      {editing && (
        <div style={S.editBox}>
          <div style={S.fieldGroup}>
            <label style={S.label}>N° de flotte (visible sur le vélo)</label>
            <input
              style={S.input}
              value={fleetInput}
              onChange={e => setFleetInput(e.target.value)}
              placeholder="ex: 042"
            />
          </div>
          <div style={S.fieldGroup}>
            <label style={S.label}>Nom personnalisé</label>
            <input
              style={S.input}
              value={nameInput}
              onChange={e => setNameInput(e.target.value)}
              placeholder="ex: Le rouge"
            />
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button style={S.saveBtn} onClick={saveEdit} disabled={updateMutation.isPending}>
              {updateMutation.isPending ? 'Enregistrement…' : 'Enregistrer'}
            </button>
            <button style={S.cancelBtn} onClick={() => setEditing(false)}>Annuler</button>
          </div>
        </div>
      )}

      {/* Cards */}
      <div style={S.cards}>
        <div style={{ ...S.card, borderLeft: `4px solid ${batteryColor(batteryPct ?? 0)}` }}>
          <div style={S.cardLabel}>Batterie actuelle</div>
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
          <div style={S.cardLabel}>Trajets totaux</div>
          <div style={S.cardValue}>{bike?.total_trips ?? '—'}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>Distance totale</div>
          <div style={S.cardValue}>
            {bike?.total_distance_km != null
              ? `${bike.total_distance_km.toFixed(1)} km`
              : '—'}
          </div>
        </div>
        <div style={S.card}>
          <div style={S.cardLabel}>IDs GBFS connus</div>
          <div style={S.cardValue}>{knownIDs.length}</div>
        </div>
      </div>

      {/* Battery chart */}
      <div style={S.section}>
        <div style={S.sectionTitle}>Batterie — 7 derniers jours</div>
        {batteryData.length > 1 ? (
          <div style={{ background: '#1e293b', borderRadius: 8, padding: '16px 8px' }}>
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={batteryData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                <XAxis
                  dataKey="time"
                  tick={{ fill: '#64748b', fontSize: 10 }}
                  interval="preserveStartEnd"
                />
                <YAxis
                  domain={[0, 100]}
                  tick={{ fill: '#64748b', fontSize: 11 }}
                  tickFormatter={(v) => `${v}%`}
                  width={40}
                />
                <Tooltip
                  contentStyle={{ background: '#0f172a', border: '1px solid #334155', borderRadius: 6 }}
                  labelStyle={{ color: '#94a3b8', fontSize: 11 }}
                  itemStyle={{ color: '#f1f5f9' }}
                  formatter={(v: number) => [`${v}%`, 'Batterie']}
                />
                <Line type="monotone" dataKey="pct" stroke="#38bdf8" dot={false} strokeWidth={2} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        ) : (
          <div style={{ color: '#64748b', fontSize: 13 }}>Pas de données sur cette période.</div>
        )}
      </div>

      {/* Trip table */}
      <div style={S.section}>
        <div style={S.sectionTitle}>Trajets ({trips?.length ?? 0} affichés)</div>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>Départ</th>
              <th style={S.th}>Durée</th>
              <th style={S.th}>Distance</th>
              <th style={S.th}>Batterie</th>
              <th style={S.th}>Itinéraire</th>
              <th style={S.th}>ID GBFS</th>
            </tr>
          </thead>
          <tbody>
            {(trips ?? []).map((t: Trip) => (
              <tr key={t.id}>
                <td style={S.td}>{new Date(t.start_time).toLocaleString('fr-FR')}</td>
                <td style={S.td}>
                  {t.duration_minutes != null ? `${Math.round(t.duration_minutes)} min` : '—'}
                </td>
                <td style={S.td}>
                  {t.distance_meters != null
                    ? `${(t.distance_meters / 1000).toFixed(1)} km`
                    : '—'}
                </td>
                <td style={S.td}>
                  {t.battery_delta != null ? (
                    <span style={{ color: '#f97316' }}>
                      −{Math.round((Math.abs(t.battery_delta) / 45000) * 100)}%
                    </span>
                  ) : '—'}
                </td>
                <td style={{ ...S.td, fontSize: 12, color: '#64748b' }}>
                  {t.start_station_name ?? '?'} → {t.end_station_name ?? '?'}
                </td>
                <td style={{ ...S.td, fontFamily: 'monospace', fontSize: 11, color: '#64748b' }}>
                  <Link to={`/bikes/${t.bike_id}`} style={{ color: '#64748b' }}>
                    {t.bike_id.slice(0, 8)}…
                  </Link>
                </td>
              </tr>
            ))}
            {(trips ?? []).length === 0 && (
              <tr>
                <td colSpan={6} style={{ ...S.td, color: '#64748b' }}>Aucun trajet enregistré.</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Known bike IDs + manual reassignment */}
      <div style={S.section}>
        <div style={S.sectionTitle}>Identifiants GBFS associés</div>
        <p style={{ fontSize: 12, color: '#64748b', marginBottom: 8 }}>
          Ces UUID sont les bike_id successifs attribués à ce vélo physique après chaque rotation (spec GBFS v2.0+).
        </p>
        {knownIDs.length > 0 ? (
          <>
            {knownIDs.length > 5 && (
              <button
                style={{ background: 'none', border: 'none', color: '#38bdf8', cursor: 'pointer', fontSize: 13, padding: 0, marginBottom: 8 }}
                onClick={() => setShowAllIDs(!showAllIDs)}
              >
                {showAllIDs ? '▲ Réduire' : `▼ Afficher tous (${knownIDs.length})`}
              </button>
            )}
            <div style={S.idsBox}>
              {visibleIDs.map((bid) => (
                <div key={bid}>
                  <Link to={`/bikes/${bid}`} style={{ color: '#94a3b8', textDecoration: 'none' }}>
                    {bid}
                  </Link>
                </div>
              ))}
              {!showAllIDs && knownIDs.length > 5 && (
                <div style={{ color: '#475569', marginTop: 4 }}>… et {knownIDs.length - 5} autre(s)</div>
              )}
            </div>
          </>
        ) : (
          <div style={{ color: '#64748b', fontSize: 13, marginBottom: 8 }}>Aucun identifiant associé.</div>
        )}

        {/* Manual reassignment tool */}
        <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 16 }}>
          {/* Pull a bike_id into this physical bike */}
          <div style={{ background: '#1e293b', borderRadius: 8, padding: '12px 16px' }}>
            <div style={{ fontSize: 13, color: '#94a3b8', marginBottom: 4, fontWeight: 600 }}>
              Rattacher un bike_id ici
            </div>
            <p style={{ fontSize: 12, color: '#64748b', margin: '0 0 8px' }}>
              Si un bike_id a été mal affecté automatiquement, entrez-le pour le rattacher à ce vélo physique.
              Tous ses trajets seront également transférés.
            </p>
            <div style={S.reassignRow}>
              <input
                style={S.smallInput}
                value={pullBikeId}
                onChange={e => setPullBikeId(e.target.value)}
                placeholder="UUID du bike_id (ex: a1b2c3d4-…)"
              />
              <button
                style={S.smallBtn}
                onClick={handlePull}
                disabled={reassignMutation.isPending || !pullBikeId.trim()}
              >
                {reassignMutation.isPending ? '…' : 'Rattacher ici'}
              </button>
            </div>
          </div>

          {/* Move a bike_id to a different physical bike */}
          <div style={{ background: '#1e293b', borderRadius: 8, padding: '12px 16px' }}>
            <div style={{ fontSize: 13, color: '#94a3b8', marginBottom: 4, fontWeight: 600 }}>
              Déplacer un bike_id vers un autre vélo physique
            </div>
            <p style={{ fontSize: 12, color: '#64748b', margin: '0 0 8px' }}>
              Pour corriger une mauvaise affectation vers un autre vélo physique (indiquez son numéro interne).
            </p>
            <div style={S.reassignRow}>
              <input
                style={S.smallInput}
                value={moveBikeId}
                onChange={e => setMoveBikeId(e.target.value)}
                placeholder="UUID du bike_id"
              />
              <span style={{ color: '#64748b', fontSize: 13, whiteSpace: 'nowrap' }}>→ Vélo #</span>
              <input
                style={{ ...S.smallInput, width: 80, fontFamily: 'inherit' }}
                value={moveTarget}
                onChange={e => setMoveTarget(e.target.value)}
                placeholder="42"
              />
              <button
                style={S.smallBtn}
                onClick={handleMove}
                disabled={reassignMutation.isPending || !moveBikeId.trim() || !moveTarget.trim()}
              >
                {reassignMutation.isPending ? '…' : 'Déplacer'}
              </button>
            </div>
          </div>

          {reassignMsg && (
            <div style={{ fontSize: 12, color: reassignMsg.startsWith('Erreur') ? '#ef4444' : '#22c55e' }}>
              {reassignMsg}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
