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
const divider: React.CSSProperties = { borderTop: '1px solid #334155', paddingTop: 8 }

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ fontWeight: 600, color: '#94a3b8', fontSize: 11, textTransform: 'uppercase', letterSpacing: 1 }}>
      {children}
    </div>
  )
}

export function MapFiltersPanel({ filters, onChange }: Props) {
  const set = (patch: Partial<MapFilters>) => onChange({ ...filters, ...patch })

  return (
    <div style={S}>
      <SectionLabel>Afficher</SectionLabel>

      <div style={{ display: 'flex', gap: 6 }}>
        <button
          onClick={() => set({ showBikes: !filters.showBikes })}
          style={{
            flex: 1, padding: '5px 0', borderRadius: 6, fontSize: 12, cursor: 'pointer', border: 'none',
            background: filters.showBikes ? '#38bdf8' : '#1e293b',
            color: filters.showBikes ? '#0f172a' : '#64748b',
            fontWeight: filters.showBikes ? 700 : 400,
          }}
        >
          Vélos
        </button>
        <button
          onClick={() => set({ showStations: !filters.showStations })}
          style={{
            flex: 1, padding: '5px 0', borderRadius: 6, fontSize: 12, cursor: 'pointer', border: 'none',
            background: filters.showStations ? '#22c55e' : '#1e293b',
            color: filters.showStations ? '#0f172a' : '#64748b',
            fontWeight: filters.showStations ? 700 : 400,
          }}
        >
          Stations
        </button>
      </div>

      <div style={divider}>
        <SectionLabel>Vélos</SectionLabel>
      </div>

      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
          <span>Batterie min.</span>
          <span style={{ color: '#38bdf8', fontWeight: 600 }}>{filters.minBattery}%</span>
        </div>
        <input
          type="range" min={0} max={100} step={5}
          value={filters.minBattery}
          disabled={!filters.showBikes}
          onChange={(e) => set({ minBattery: +e.target.value })}
          style={{ width: '100%', accentColor: '#38bdf8', opacity: filters.showBikes ? 1 : 0.3 }}
        />
      </div>

      <label style={{ ...row, opacity: filters.showBikes ? 1 : 0.3 }}>
        <input type="checkbox" checked={filters.showElectric} disabled={!filters.showBikes}
          onChange={(e) => set({ showElectric: e.target.checked })} />
        <span style={label}>Électriques</span>
      </label>

      <label style={{ ...row, opacity: filters.showBikes ? 1 : 0.3 }}>
        <input type="checkbox" checked={filters.showManual} disabled={!filters.showBikes}
          onChange={(e) => set({ showManual: e.target.checked })} />
        <span style={label}>Mécaniques</span>
      </label>

      <label style={{ ...row, opacity: filters.showBikes ? 1 : 0.3 }}>
        <input type="checkbox" checked={!filters.hideDisabled} disabled={!filters.showBikes}
          onChange={(e) => set({ hideDisabled: !e.target.checked })} />
        <span style={label}>Hors service</span>
      </label>

      <div style={divider}>
        <label style={row}>
          <input type="checkbox" checked={filters.showHeatmap}
            onChange={(e) => set({ showHeatmap: e.target.checked })} />
          <span style={label}>Heatmap trajets</span>
        </label>
      </div>
    </div>
  )
}
