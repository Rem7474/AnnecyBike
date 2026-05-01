import { BrowserRouter, Routes, Route, NavLink } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MapPage } from './pages/MapPage'
import { BikeDetailPage } from './pages/BikeDetailPage'
import { StationDetailPage } from './pages/StationDetailPage'
import { StatsPage } from './pages/StatsPage'

const qc = new QueryClient({ defaultOptions: { queries: { retry: 2, staleTime: 30_000 } } })

const navStyle = ({ isActive }: { isActive: boolean }): React.CSSProperties => ({
  color: isActive ? '#38bdf8' : '#94a3b8',
  textDecoration: 'none',
  fontWeight: isActive ? 600 : 400,
  padding: '0 4px',
})

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
        </nav>
        <Routes>
          <Route path="/" element={<MapPage />} />
          <Route path="/bikes/:id" element={<BikeDetailPage />} />
          <Route path="/stations/:id" element={<StationDetailPage />} />
          <Route path="/stats" element={<StatsPage />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
