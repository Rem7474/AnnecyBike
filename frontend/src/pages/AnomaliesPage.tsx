import { useQuery } from '@tanstack/react-query'
import { MapContainer, TileLayer, CircleMarker, Popup } from 'react-leaflet'
import { Link } from 'react-router-dom'
import { api } from '../api'

const S: Record<string, React.CSSProperties> = {
  page: { padding: 24, maxWidth: 1100, margin: '0 auto' },
  title: { fontSize: 22, fontWeight: 700, marginBottom: 4 },
  subtitle: { fontSize: 13, color: '#64748b', marginBottom: 20 },
  grid: { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20 },
  table: { width: '100%', borderCollapse: 'collapse', fontSize: 13 },
  th: { textAlign: 'left', padding: '8px 10px', color: '#64748b', borderBottom: '1px solid #334155' },
  td: { padding: '10px 10px', borderBottom: '1px solid #1e293b', verticalAlign: 'middle' },
  badge: (h: number): React.CSSProperties => ({
    display: 'inline-block', padding: '2px 8px', borderRadius: 12, fontSize: 11, fontWeight: 600,
    background: h > 48 ? '#7f1d1d' : h > 24 ? '#7c2d12' : '#1c1917',
    color: h > 48 ? '#fca5a5' : h > 24 ? '#fdba74' : '#a8a29e',
  }),
}

export function AnomaliesPage() {
  const { data: anomalies = [], isLoading } = useQuery({
    queryKey: ['anomalies'],
    queryFn: api.anomalies.list,
    refetchInterval: 60_000,
  })

  const center: [number, number] = anomalies.length > 0
    ? [anomalies[0].lat, anomalies[0].lon]
    : [45.899, 6.129]

  return (
    <div style={S.page}>
      <div style={S.title}>
        🚨 Anomalies détectées
        {anomalies.length > 0 && (
          <span style={{ marginLeft: 10, fontSize: 16, color: '#ef4444' }}>
            {anomalies.length} vélo{anomalies.length > 1 ? 's' : ''}
          </span>
        )}
      </div>
      <div style={S.subtitle}>
        Vélos hors station, non désactivés, et immobiles depuis plus de 24 heures.
        Mise à jour toutes les 60 secondes.
      </div>

      {isLoading && <div style={{ color: '#64748b' }}>Chargement…</div>}

      {!isLoading && anomalies.length === 0 && (
        <div style={{
          background: '#1e293b', borderRadius: 10, padding: 24,
          color: '#22c55e', textAlign: 'center', fontSize: 15,
        }}>
          ✅ Aucune anomalie détectée — tous les vélos semblent en ordre.
        </div>
      )}

      {anomalies.length > 0 && (
        <div style={S.grid}>
          {/* Table */}
          <div>
            <table style={S.table}>
              <thead>
                <tr>
                  <th style={S.th}>Vélo</th>
                  <th style={S.th}>Type</th>
                  <th style={S.th}>Immobile depuis</th>
                  <th style={S.th}>Position</th>
                </tr>
              </thead>
              <tbody>
                {anomalies.map((a) => (
                  <tr key={a.bike_id}>
                    <td style={S.td}>
                      <Link to={`/bikes/${a.bike_id}`} style={{ color: '#38bdf8', textDecoration: 'none', fontFamily: 'monospace' }}>
                        {a.bike_id.slice(0, 8)}…
                      </Link>
                    </td>
                    <td style={S.td}>{a.vehicle_type_id}</td>
                    <td style={S.td}>
                      <span style={S.badge(a.hours_outside)}>
                        {a.hours_outside.toFixed(0)}h
                      </span>
                    </td>
                    <td style={{ ...S.td, fontSize: 11, color: '#64748b', fontFamily: 'monospace' }}>
                      {a.lat.toFixed(5)}, {a.lon.toFixed(5)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Map */}
          <div style={{ borderRadius: 10, overflow: 'hidden', height: 400 }}>
            <MapContainer center={center} zoom={13} style={{ height: '100%' }}>
              <TileLayer
                url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
                attribution='&copy; OpenStreetMap'
              />
              {anomalies.map((a) => (
                <CircleMarker
                  key={a.bike_id}
                  center={[a.lat, a.lon]}
                  radius={10}
                  pathOptions={{
                    color: '#ef4444', fillColor: '#ef4444', fillOpacity: 0.7, weight: 2,
                  }}
                >
                  <Popup>
                    <strong>{a.bike_id.slice(0, 8)}…</strong><br />
                    Immobile depuis {a.hours_outside.toFixed(0)}h<br />
                    <Link to={`/bikes/${a.bike_id}`}>Voir le vélo →</Link>
                  </Popup>
                </CircleMarker>
              ))}
            </MapContainer>
          </div>
        </div>
      )}
    </div>
  )
}
