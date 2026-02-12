import { useState, useEffect } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'

const navItems = [
  { to: '/', label: 'Dashboard', icon: '🧠' },
  { to: '/memories', label: 'Memories', icon: '📁' },
  { to: '/search', label: 'Search', icon: '🔍' },
  { to: '/consolidation', label: 'Consolidation', icon: '🔗' },
  { to: '/analytics', label: 'Analytics', icon: '📊' },
]

export default function Layout() {
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem('sidebar') === 'collapsed')
  const [mobileOpen, setMobileOpen] = useState(false)
  const location = useLocation()

  useEffect(() => { setMobileOpen(false) }, [location.pathname])
  useEffect(() => { localStorage.setItem('sidebar', collapsed ? 'collapsed' : 'expanded') }, [collapsed])

  return (
    <div className="min-h-screen flex bg-surface-0 text-gray-100">
      {/* Mobile overlay */}
      {mobileOpen && (
        <div className="fixed inset-0 bg-black/50 z-30 md:hidden" onClick={() => setMobileOpen(false)} />
      )}

      {/* Sidebar */}
      <aside
        className={`
          fixed md:sticky top-0 h-screen z-40 flex flex-col shrink-0
          bg-surface-1 border-r border-border transition-all duration-200
          ${mobileOpen ? 'translate-x-0' : '-translate-x-full'}
          md:translate-x-0
        `}
        style={{ width: mobileOpen ? '14rem' : collapsed ? '4rem' : '14rem' }}
      >
        {/* Logo */}
        <div className="px-4 py-5 border-b border-border flex items-center gap-3 shrink-0 overflow-hidden">
          <span className="text-2xl shrink-0">🐙</span>
          <span className={`text-brand-400 font-bold text-lg tracking-tight whitespace-nowrap transition-opacity duration-200 ${collapsed && !mobileOpen ? 'opacity-0 w-0' : 'opacity-100'}`}>
            Contextify
          </span>
        </div>

        {/* Nav */}
        <nav className="flex-1 py-3 px-2 space-y-1 overflow-hidden">
          {navItems.map(item => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) => `
                flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium
                transition-colors relative overflow-hidden
                ${isActive
                  ? 'bg-brand-600/15 text-brand-300'
                  : 'text-gray-400 hover:text-white hover:bg-surface-2'
                }
              `}
            >
              {({ isActive }) => (
                <>
                  {isActive && <div className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-5 bg-brand-400 rounded-r" />}
                  <span className="text-lg shrink-0">{item.icon}</span>
                  <span className={`whitespace-nowrap transition-opacity duration-200 ${collapsed && !mobileOpen ? 'opacity-0' : 'opacity-100'}`}>
                    {item.label}
                  </span>
                </>
              )}
            </NavLink>
          ))}
        </nav>

        {/* Footer */}
        <div className="px-3 py-3 border-t border-border shrink-0 flex items-center justify-between overflow-hidden">
          <span className={`text-xs text-gray-600 whitespace-nowrap transition-opacity duration-200 ${collapsed && !mobileOpen ? 'opacity-0 w-0' : 'opacity-100'}`}>
            v0.1.0
          </span>
          <button
            onClick={() => setCollapsed(c => !c)}
            className="text-gray-600 hover:text-gray-400 transition-colors text-sm hidden md:block shrink-0"
            title={collapsed ? 'Expand' : 'Collapse'}
          >
            {collapsed ? '▸' : '◂'}
          </button>
        </div>
      </aside>

      {/* Main */}
      <div className="flex-1 flex flex-col min-h-screen min-w-0">
        {/* Header */}
        <header className="sticky top-0 z-20 bg-surface-0/80 backdrop-blur-md border-b border-border px-4 md:px-6 h-14 flex items-center shrink-0">
          <button onClick={() => setMobileOpen(true)} className="md:hidden btn-ghost p-2 -ml-2 mr-2 text-lg">
            ☰
          </button>
        </header>

        {/* Content */}
        <main className="flex-1 p-4 md:p-6 max-w-7xl mx-auto w-full">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
