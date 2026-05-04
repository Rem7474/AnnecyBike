import { CSSProperties, MouseEvent, useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api'
import type { PhysicalBike } from '../types'

const S: Record<string, CSSProperties> = {
  page: { padding: 24, maxWidth: 1100, margin: '0 auto' },
  title: { fontSize: 22, fontWeight: 700, marginBottom: 4 },
  subtitle: { fontSize: 13, color: '#64748b', marginBottom: 20 },
  toolbar: { display: 'flex', gap: 12, marginBottom: 16, alignItems: 'center' },
  search: {
    background: '#1e293b', border: '1px solid #334155', borderRadius: 6,
    color: '#f1f5f9', fontSize: 13, padding: '7px 12px', outline: 'none', width: 280,
  },
  table: { width: '100%', borderCollapse: 'collapse', fontSize: 13 },
  th: {
    textAlign: 'left', padding: '8px 12px', color: '#64748b',
    borderBottom: '2px solid #1e293b', userSelect: 'none', cursor: 'pointer',
    whiteSpace: 'nowrap',
  },
  thActive: { color: '#38bdf8' },
  td: { padding: '9px 12px', borderBottom: '1px solid #1e293b', verticalAlign: 'middle' },
  row: { cursor: 'pointer', transition: 'background 0.1s' },
  serial: { fontFamily: 'monospace', fontSize: 12, color: '#475569' },
  fleetBadge: {
    display: 'inline-block', background: '#0f172a', border: '1px solid #334155',
    borderRadius: 4, padding: '2px 8px', fontWeight: 700, color: '#38bdf8',
    fontSize: 13, fontFamily: 'monospace',
  },
  inlineInput: {
    background: '#0f172a', border: '1px solid #38bdf8', borderRadius: 4,
    color: '#f1f5f9', fontSize: 13, padding: '3px 7px', outline: 'none',
    width: 90,
  },
  nameInput: {
    background: '#0f172a', border: '1px solid #38bdf8', borderRadius: 4,
    color: '#f1f5f9', fontSize: 13, padding: '3px 7px', outline: 'none',
    width: 160,
  },
  iconBtn: {
    background: 'none', border: 'none', cursor: 'pointer',
    color: '#64748b', fontSize: 14, padding: '2px 4px', borderRadius: 4,
  },
  saveBtn: {
    background: '#38bdf8', color: '#0f172a', border: 'none',
    borderRadius: 4, padding: '3px 10px', fontSize: 12, fontWeight: 600, cursor: 'pointer',
  },
  cancelBtn: {
    background: 'none', border: '1px solid #334155', borderRadius: 4,
    color: '#94a3b8', padding: '3px 8px', fontSize: 12, cursor: 'pointer',
  },
}

type SortKey = 'fleet_number' | 'total_trips' | 'total_distance_km' | 'last_seen' | 'id_count'

function relativeTime(iso: string) {
  const diff = Date.now() - new Date(iso).getTime()
  const m = Math.floor(diff / 60_000)
  if (m < 1) return "à l'instant"
  if (m < 60) return `il y a ${m} min`
  const h = Math.floor(m / 60)
  if (h < 24) return `il y a ${h} h`
  return `il y a ${Math.floor(h / 24)} j`
}

function primaryLabel(b: PhysicalBike) {
  if (b.fleet_number) return b.fleet_number
  if (b.custom_name) return b.custom_name
  return null
}

export function PhysicalBikeListPage() {
  const navigate = useNavigate()
  const qc = useQueryClient()

  const [search, setSearch] = useState('')
  const [sortKey, setSortKey] = useState<SortKey>('last_seen')
  const [sortDesc, setSortDesc] = useState(true)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editFleet, setEditFleet] = useState('')
  const [editName, setEditName] = useState('')

  const { data: bikes = [], isLoading } = useQuery({
    queryKey: ['physical-bikes'],
    queryFn: api.physicalBikes.list,
    refetchInterval: 60_000,
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, fleet_number, custom_name }: { id: number; fleet_number: string | null; custom_name: string | null }) =>
      api.physicalBikes.update(id, { fleet_number, custom_name }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['physical-bikes'] })
      setEditingId(null)
    },
  })

  const filtered = useMemo(() => {
    const q = search.toLowerCase()
    return bikes.filter(b =>
      !q ||
      (b.fleet_number ?? '').toLowerCase().includes(q) ||
      (b.custom_name ?? '').toLowerCase().includes(q) ||
      String(b.id).padStart(6, '0').includes(q) ||
      b.vehicle_type_id.toLowerCase().includes(q)
    )
  }, [bikes, search])

  const sorted = useMemo(() => {
    return [...filtered].sort((a, b) => {
      let av: number | string, bv: number | string
      switch (sortKey) {
        case 'fleet_number':
          av = a.fleet_number ?? `￿${a.id}`
          bv = b.fleet_number ?? `￿${b.id}`
          break
        case 'total_trips':      av = a.total_trips; bv = b.total_trips; break
        case 'total_distance_km': av = a.total_distance_km; bv = b.total_distance_km; break
        case 'id_count':         av = a.id_count ?? 0; bv = b.id_count ?? 0; break
        default:                 av = a.last_seen; bv = b.last_seen; break
      }
      if (av < bv) return sortDesc ? 1 : -1
      if (av > bv) return sortDesc ? -1 : 1
      return 0
    })
  }, [filtered, sortKey, sortDesc])

  function toggleSort(key: SortKey) {
    if (sortKey === key) setSortDesc(!sortDesc)
    else { setSortKey(key); setSortDesc(true) }
  }

  function thStyle(key: SortKey): CSSProperties {
    return sortKey === key ? { ...S.th, ...S.thActive } : S.th
  }

  function sortArrow(key: SortKey) {
    if (sortKey !== key) return ' ↕'
    return sortDesc ? ' ↓' : ' ↑'
  }

  function startEdit(e: MouseEvent, b: PhysicalBike) {
    e.stopPropagation()
    setEditingId(b.id)
    setEditFleet(b.fleet_number ?? '')
    setEditName(b.custom_name ?? '')
  }

  function saveEdit(e: MouseEvent, id: number) {
    e.stopPropagation()
    updateMutation.mutate({
      id,
      fleet_number: editFleet.trim() || null,
      custom_name: editName.trim() || null,
    })
  }

  function cancelEdit(e: MouseEvent) {
    e.stopPropagation()
    setEditingId(null)
  }

  return (
    <div style={S.page}>
      <div style={S.title}>Vélos physiques</div>
      <div style={S.subtitle}>
        {bikes.length} vélo{bikes.length !== 1 ? 's' : ''} identifié{bikes.length !== 1 ? 's' : ''}
        {' '}· {bikes.filter(b => b.fleet_number).length} numérotés
        {' '}· Cliquez sur une ligne pour voir le détail, ✏ pour modifier le numéro
      </div>

      <div style={S.toolbar}>
        <input
          style={S.search}
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder="Rechercher par numéro, nom, type…"
        />
        <span style={{ fontSize: 12, color: '#64748b' }}>
          {sorted.length !== bikes.length && `${sorted.length} résultat${sorted.length !== 1 ? 's' : ''}`}
        </span>
      </div>

      {isLoading ? (
        <div style={{ color: '#64748b', padding: 24 }}>Chargement…</div>
      ) : (
        <table style={S.table}>
          <thead>
            <tr>
              <th style={thStyle('fleet_number')} onClick={() => toggleSort('fleet_number')}>
                N° flotte{sortArrow('fleet_number')}
              </th>
              <th style={S.th}>Nom</th>
              <th style={S.th}>Type</th>
              <th style={thStyle('total_trips')} onClick={() => toggleSort('total_trips')}>
                Trajets{sortArrow('total_trips')}
              </th>
              <th style={thStyle('total_distance_km')} onClick={() => toggleSort('total_distance_km')}>
                Distance{sortArrow('total_distance_km')}
              </th>
              <th style={thStyle('id_count')} onClick={() => toggleSort('id_count')}>
                IDs GBFS{sortArrow('id_count')}
              </th>
              <th style={thStyle('last_seen')} onClick={() => toggleSort('last_seen')}>
                Vu{sortArrow('last_seen')}
              </th>
              <th style={S.th}></th>
            </tr>
          </thead>
          <tbody>
            {sorted.map(b => {
              const isEditing = editingId === b.id
              return (
                <tr
                  key={b.id}
                  style={S.row}
                  onClick={() => !isEditing && navigate(`/physical-bikes/${b.id}`)}
                  onMouseEnter={e => (e.currentTarget.style.background = '#1e293b')}
                  onMouseLeave={e => (e.currentTarget.style.background = '')}
                >
                  {/* Fleet number — editable */}
                  <td style={S.td} onClick={e => isEditing && e.stopPropagation()}>
                    {isEditing ? (
                      <input
                        style={S.inlineInput}
                        value={editFleet}
                        onChange={e => setEditFleet(e.target.value)}
                        placeholder="ex: 042"
                        autoFocus
                        onClick={e => e.stopPropagation()}
                      />
                    ) : primaryLabel(b) ? (
                      <span style={S.fleetBadge}>{b.fleet_number ?? primaryLabel(b)}</span>
                    ) : (
                      <span style={S.serial}>#{String(b.id).padStart(6, '0')}</span>
                    )}
                  </td>

                  {/* Custom name — editable */}
                  <td style={S.td} onClick={e => isEditing && e.stopPropagation()}>
                    {isEditing ? (
                      <input
                        style={S.nameInput}
                        value={editName}
                        onChange={e => setEditName(e.target.value)}
                        placeholder="Nom libre"
                        onClick={e => e.stopPropagation()}
                      />
                    ) : (
                      <span style={{ color: b.custom_name ? '#f1f5f9' : '#475569' }}>
                        {b.custom_name ?? (b.fleet_number ? '' : '—')}
                      </span>
                    )}
                  </td>

                  <td style={{ ...S.td, color: '#64748b', fontSize: 12 }}>{b.vehicle_type_id}</td>
                  <td style={S.td}>{b.total_trips}</td>
                  <td style={S.td}>{b.total_distance_km.toFixed(1)} km</td>
                  <td style={{ ...S.td, color: '#64748b' }}>{b.id_count ?? 0}</td>
                  <td style={{ ...S.td, color: '#64748b', fontSize: 12 }}>{relativeTime(b.last_seen)}</td>

                  {/* Actions */}
                  <td style={{ ...S.td, whiteSpace: 'nowrap' }} onClick={e => e.stopPropagation()}>
                    {isEditing ? (
                      <span style={{ display: 'flex', gap: 4 }}>
                        <button
                          style={S.saveBtn}
                          onClick={e => saveEdit(e, b.id)}
                          disabled={updateMutation.isPending}
                        >
                          ✓
                        </button>
                        <button style={S.cancelBtn} onClick={cancelEdit}>✕</button>
                      </span>
                    ) : (
                      <button style={S.iconBtn} onClick={e => startEdit(e, b)} title="Modifier">
                        ✏
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
            {sorted.length === 0 && (
              <tr>
                <td colSpan={8} style={{ ...S.td, color: '#64748b', textAlign: 'center', padding: 32 }}>
                  {search ? 'Aucun résultat pour cette recherche.' : 'Aucun vélo physique enregistré.'}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  )
}
