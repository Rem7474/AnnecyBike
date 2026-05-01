import { useEffect, useRef } from 'react'
import { useMap } from 'react-leaflet'
import 'leaflet.heat'
import L from 'leaflet'
import type { HeatPoint } from '../../types'

interface Props {
  points: HeatPoint[]
}

export function HeatmapLayer({ points }: Props) {
  const map = useMap()
  const layerRef = useRef<L.Layer | null>(null)

  useEffect(() => {
    if (layerRef.current) {
      map.removeLayer(layerRef.current)
    }
    if (points.length === 0) return

    const data: [number, number, number][] = points.map((p) => [p.lat, p.lon, p.weight])
    layerRef.current = L.heatLayer(data, {
      radius: 20,
      blur: 15,
      maxZoom: 17,
      gradient: { 0.2: '#3b82f6', 0.5: '#f59e0b', 0.8: '#ef4444', 1.0: '#7c3aed' },
    }).addTo(map)

    return () => {
      if (layerRef.current) map.removeLayer(layerRef.current)
    }
  }, [points, map])

  return null
}
