import { BrowserRouter, Routes, Route, NavLink } from 'react-router-dom'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { MapPage } from './pages/MapPage'
import { BikeDetailPage } from './pages/BikeDetailPage'
import { StationDetailPage } from './pages/StationDetailPage'
import { StatsPage } from './pages/StatsPage'
import { AnomaliesPage } from './pages/AnomaliesPage'
import { PhysicalBikeDetailPage } from './pages/PhysicalBikeDetailPage'
import { PhysicalBikeListPage } from './pages/PhysicalBikeListPage'
import { api } from './api'

const qc = new QueryClient({ defaultOptions: { queries: { retry: 2, staleTime: 30_000 } } })

const navStyle = ({ isActive }: { isActive: boolean }): React.CSSProperties => ({
  color: isActive ? '#38bdf8' : '#94a3b8',
  textDecoration: 'none',
  fontWeight: isActive ? 600 : 400,
  padding: '0 4px',
})

function AnomalyBadge() {
  const { data } = useQuery({
    queryKey: ['anomalies'],
    queryFn: api.anomalies.list,
    refetchInterval: 60_000,
  })
  const count = data?.length ?? 0
  if (count === 0) return null
  return (
    <span style={{
      background: '#ef4444', color: 'white',
      borderRadius: '50%', width: 18, height: 18,
      display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
      fontSize: 10, fontWeight: 700, marginLeft: 4, verticalAlign: 'middle',
    }}>
      {count > 9 ? '9+' : count}
    </span>
  )
}

export function App() {
  return (
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <nav style={{
          height: 48, background: '#0f172a', borderBottom: '1px solid #1e293b',
          display: 'flex', alignItems: 'center', gap: 24, padding: '0 20px',
          flexShrink: 0,
        }}>
          <span style={{ fontWeight: 700, color: '#f1f5f9', fontSize: 16 }}>🚲 AnnecyBike</span>
          <NavLink to="/" end style={navStyle}>Carte</NavLink>
          <NavLink to="/stats" style={navStyle}>Statistiques</NavLink>
          <NavLink to="/physical-bikes" style={navStyle}>Vélos</NavLink>
          <NavLink to="/anomalies" style={navStyle}>
            Anomalies<AnomalyBadge />
          </NavLink>
        </nav>
        <Routes>
          <Route path="/" element={<MapPage />} />
          <Route path="/bikes/:id" element={<BikeDetailPage />} />
          <Route path="/stations/:id" element={<StationDetailPage />} />
          <Route path="/stats" element={<StatsPage />} />
          <Route path="/anomalies" element={<AnomaliesPage />} />
          <Route path="/physical-bikes" element={<PhysicalBikeListPage />} />
          <Route path="/physical-bikes/:id" element={<PhysicalBikeDetailPage />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
