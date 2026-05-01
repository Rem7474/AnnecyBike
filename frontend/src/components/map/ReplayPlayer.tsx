import { useState, useEffect, useRef, useCallback } from 'react'
import { useMap } from 'react-leaflet'
import { DivIcon } from 'leaflet'
import type { Map as LMap, Marker as LMarker } from 'leaflet'
import L from 'leaflet'
import { api } from '../../api'
import type { ReplayBucket } from '../../types'

const panel: React.CSSProperties = {
  position: 'absolute', bottom: 12, left: '50%', transform: 'translateX(-50%)',
  zIndex: 1000, background: 'rgba(15,23,42,0.95)', backdropFilter: 'blur(8px)',
  border: '1px solid #334155', borderRadius: 12, padding: '10px 16px',
  display: 'flex', flexDirection: 'column', gap: 8,
  color: '#f1f5f9', fontSize: 13, minWidth: 340,
}

function bikeReplayIcon(hasStation: boolean) {
  const color = hasStation ? '#64748b' : '#38bdf8'
  return new DivIcon({
    html: `<div style="width:10px;height:10px;border-radius:50%;background:${color};border:1px solid white;opacity:0.85"></div>`,
    className: '', iconSize: [10, 10], iconAnchor: [5, 5],
  })
}

// Manages replay markers imperatively (avoids re-rendering hundreds of React nodes)
function useReplayMarkers(map: LMap) {
  const markers = useRef<Map<string, LMarker>>(new Map())

  const update = useCallback((buckets: ReplayBucket[], idx: number) => {
    if (!buckets[idx]) return
    const snap = buckets[idx].snapshots

    const seen = new Set<string>()
    for (const b of snap) {
      seen.add(b.b)
      const existing = markers.current.get(b.b)
      const icon = bikeReplayIcon(!!b.s)
      if (existing) {
        existing.setLatLng([b.la, b.lo])
        existing.setIcon(icon)
      } else {
        const m = L.marker([b.la, b.lo], { icon }).addTo(map)
        markers.current.set(b.b, m)
      }
    }
    // Remove bikes not in this bucket
    for (const [id, m] of markers.current) {
      if (!seen.has(id)) {
        map.removeLayer(m)
        markers.current.delete(id)
      }
    }
  }, [map])

  const clear = useCallback(() => {
    for (const m of markers.current.values()) map.removeLayer(m)
    markers.current.clear()
  }, [map])

  return { update, clear }
}

interface Props {
  onClose: () => void
}

export function ReplayPlayerInner({ onClose }: Props) {
  const map = useMap()
  const { update, clear } = useReplayMarkers(map)

  const today = new Date().toISOString().slice(0, 10)
  const [date, setDate] = useState(today)
  const [buckets, setBuckets] = useState<ReplayBucket[]>([])
  const [idx, setIdx] = useState(0)
  const [playing, setPlaying] = useState(false)
  const [speed, setSpeed] = useState(1)
  const [loading, setLoading] = useState(false)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const load = async () => {
    setLoading(true)
    clear()
    try {
      const data = await api.replay.get(date, 10)
      setBuckets(data)
      setIdx(0)
      setPlaying(false)
    } catch (e) {
      alert('Erreur lors du chargement du replay.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (buckets.length > 0) update(buckets, idx)
  }, [buckets, idx, update])

  useEffect(() => {
    if (intervalRef.current) clearInterval(intervalRef.current)
    if (playing && buckets.length > 0) {
      intervalRef.current = setInterval(() => {
        setIdx((i) => {
          if (i >= buckets.length - 1) { setPlaying(false); return i }
          return i + 1
        })
      }, Math.round(800 / speed))
    }
    return () => { if (intervalRef.current) clearInterval(intervalRef.current) }
  }, [playing, speed, buckets])

  useEffect(() => () => clear(), [clear])

  const currentTime = buckets[idx]
    ? new Date(buckets[idx].time).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
    : '--:--'

  return (
    <div style={panel}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span style={{ fontWeight: 600 }}>⏱ Replay</span>
        <input type="date" value={date} max={today}
          onChange={(e) => setDate(e.target.value)}
          style={{ flex: 1, background: '#1e293b', border: '1px solid #475569', borderRadius: 6, color: '#f1f5f9', padding: '2px 6px' }} />
        <button onClick={load} disabled={loading}
          style={{ background: '#3b82f6', border: 'none', borderRadius: 6, color: 'white', padding: '4px 10px', cursor: 'pointer' }}>
          {loading ? '…' : 'Charger'}
        </button>
        <button onClick={onClose}
          style={{ background: '#334155', border: 'none', borderRadius: 6, color: '#94a3b8', padding: '4px 8px', cursor: 'pointer' }}>
          ✕
        </button>
      </div>

      {buckets.length > 0 && (
        <>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <button onClick={() => setPlaying(!playing)}
              style={{ background: playing ? '#f59e0b' : '#22c55e', border: 'none', borderRadius: 6, color: 'white', padding: '4px 12px', cursor: 'pointer', fontWeight: 600 }}>
              {playing ? '⏸' : '▶'}
            </button>
            <span style={{ minWidth: 38, textAlign: 'center', color: '#38bdf8', fontWeight: 600 }}>{currentTime}</span>
            <input type="range" min={0} max={buckets.length - 1} value={idx}
              onChange={(e) => { setPlaying(false); setIdx(+e.target.value) }}
              style={{ flex: 1, accentColor: '#38bdf8' }} />
            <select value={speed} onChange={(e) => setSpeed(+e.target.value)}
              style={{ background: '#1e293b', border: '1px solid #475569', borderRadius: 6, color: '#f1f5f9', padding: '2px 4px' }}>
              <option value={0.5}>×0.5</option>
              <option value={1}>×1</option>
              <option value={2}>×2</option>
              <option value={4}>×4</option>
            </select>
          </div>
          <div style={{ color: '#64748b', fontSize: 11 }}>
            {buckets[idx]?.snapshots.length ?? 0} vélos · step {idx + 1}/{buckets.length}
          </div>
        </>
      )}
    </div>
  )
}
