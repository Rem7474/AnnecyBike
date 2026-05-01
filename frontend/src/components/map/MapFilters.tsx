import type { MapFilters } from '../../types'

interface Props {
  filters: MapFilters
  onChange: (f: MapFilters) => void
}

const S: React.CSSProperties = {
  position: 'absolute', top: 12, right: 12, zIndex: 1000,
  background: 'rgba(15,23,42,0.92)', backdropFilter: 'blur(6px)',
  border: '1px solid #334155', borderRadius: 10, padding: '12px 14px',
  width: 210, display: 'flex', flexDirection: 'column', gap: 10,
  color: '#f1f5f9', fontSize: 13,
}

const row: React.CSSProperties = { display: 'flex', alignItems: 'center', gap: 8 }
const label: React.CSSProperties = { flex: 1, userSelect: 'none' }

export function MapFiltersPanel({ filters, onChange }: Props) {
  const set = (patch: Partial<MapFilters>) => onChange({ ...filters, ...patch })

  return (
    <div style={S}>
      <div style={{ fontWeight: 600, marginBottom: 2, color: '#94a3b8', fontSize: 11, textTransform: 'uppercase', letterSpacing: 1 }}>
        Filtres
      </div>

      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
          <span>Batterie min.</span>
          <span style={{ color: '#38bdf8', fontWeight: 600 }}>{filters.minBattery}%</span>
        </div>
        <input
          type="range" min={0} max={100} step={5}
          value={filters.minBattery}
          onChange={(e) => set({ minBattery: +e.target.value })}
          style={{ width: '100%', accentColor: '#38bdf8' }}
        />
      </div>

      <label style={row}>
        <input type="checkbox" checked={filters.showElectric}
          onChange={(e) => set({ showElectric: e.target.checked })} />
        <span style={label}>⚡ Vélos électriques</span>
      </label>

      <label style={row}>
        <input type="checkbox" checked={filters.showManual}
          onChange={(e) => set({ showManual: e.target.checked })} />
        <span style={label}>🚲 Vélos mécaniques</span>
      </label>

      <label style={row}>
        <input type="checkbox" checked={!filters.hideDisabled}
          onChange={(e) => set({ hideDisabled: !e.target.checked })} />
        <span style={label}>🔴 Hors service</span>
      </label>

      <div style={{ borderTop: '1px solid #334155', paddingTop: 8 }}>
        <label style={row}>
          <input type="checkbox" checked={filters.showHeatmap}
            onChange={(e) => set({ showHeatmap: e.target.checked })} />
          <span style={label}>🔥 Heatmap trajets</span>
        </label>
      </div>
    </div>
  )
}
