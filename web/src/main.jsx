import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { ToastProvider } from './hooks/useToast'
import './index.css'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import MemoryBrowser from './pages/MemoryBrowser'
import Search from './pages/Search'
import Consolidation from './pages/Consolidation'
import EmptyState from './components/EmptyState'

function NotFound() {
  return <EmptyState icon="🔮" title="Page not found" description="The page you're looking for doesn't exist." />
}

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <ToastProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<Dashboard />} />
            <Route path="/memories" element={<MemoryBrowser />} />
            <Route path="/search" element={<Search />} />
            <Route path="/consolidation" element={<Consolidation />} />
            <Route path="*" element={<NotFound />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </ToastProvider>
  </React.StrictMode>
)
