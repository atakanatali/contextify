import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import { TypeBadge, ImportanceDot, TtlBadge } from '../components/Badge'
import { StatCard as SkeletonStatCard, MemoryCard as SkeletonMemoryCard } from '../components/Skeleton'
import EmptyState from '../components/EmptyState'

const STAT_ICONS = [
  { key: 'total', icon: '🧠', label: 'Total Memories', color: 'from-brand-600 to-brand-700' },
  { key: 'longterm', icon: '💎', label: 'Long-term', color: 'from-emerald-600 to-emerald-700' },
  { key: 'shortterm', icon: '⏳', label: 'Short-term', color: 'from-amber-600 to-amber-700' },
  { key: 'expiring', icon: '🔥', label: 'Expiring Soon', color: 'from-red-600 to-red-700' },
  { key: 'pending', icon: '🔗', label: 'Pending Merges', color: 'from-violet-600 to-violet-700', link: '/consolidation' },
]

const TYPE_COLORS = {
  solution: 'bg-emerald-500',
  problem: 'bg-red-500',
  code_pattern: 'bg-blue-500',
  fix: 'bg-amber-500',
  error: 'bg-rose-500',
  workflow: 'bg-purple-500',
  decision: 'bg-orange-500',
  general: 'bg-gray-500',
}

function timeAgo(dateStr) {
  if (!dateStr) return ''
  const seconds = Math.floor((Date.now() - new Date(dateStr)) / 1000)
  if (seconds < 60) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`
  return new Date(dateStr).toLocaleDateString()
}

export default function Dashboard() {
  const [stats, setStats] = useState(null)
  const [recent, setRecent] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    api.getStats()
      .then(setStats)
      .catch((e) => setError(e.message))

    api.listMemories({ limit: 5 })
      .then(results => setRecent(results?.map(r => r.memory) || []))
      .catch(() => {}) // non-critical
  }, [])

  if (error) {
    return (
      <div className="text-red-400 bg-red-900/20 border border-red-800 rounded-lg p-4 animate-fade-in">
        <span className="font-medium">Failed to load stats:</span> {error}
      </div>
    )
  }

  return (
    <div className="space-y-8 animate-fade-in">
      {/* Stat cards */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
        {!stats ? (
          Array.from({ length: 5 }).map((_, i) => <SkeletonStatCard key={i} />)
        ) : (
          STAT_ICONS.map(({ key, icon, label, color, link }) => {
            const value = key === 'total' ? stats.total_memories
              : key === 'longterm' ? stats.long_term_count
              : key === 'shortterm' ? stats.short_term_count
              : key === 'expiring' ? stats.expiring_count
              : stats.pending_suggestions || 0

            const Wrapper = link ? Link : 'div'
            const wrapperProps = link ? { to: link } : {}

            return (
              <Wrapper key={key} {...wrapperProps} className={`card p-5 ${link ? 'hover:border-brand-500/30 transition-colors cursor-pointer' : ''}`}>
                <div className="flex items-center gap-3 mb-3">
                  <div className={`w-10 h-10 rounded-xl bg-gradient-to-br ${color} flex items-center justify-center text-lg shadow-lg`}>
                    {icon}
                  </div>
                  <span className="text-xs text-gray-500 uppercase tracking-wider font-medium">{label}</span>
                </div>
                <div className="text-3xl font-bold text-white tabular-nums">{value}</div>
              </Wrapper>
            )
          })
        )}
      </div>

      {/* Quick actions */}
      <div className="flex flex-wrap gap-3">
        <Link to="/memories" className="btn-primary">
          <span>＋</span> Store Memory
        </Link>
        <Link to="/search" className="btn-ghost border border-border">
          <span>🔍</span> Search Memories
        </Link>
      </div>

      {/* Main content grid */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        {/* Type distribution */}
        <div className="card p-5 xl:col-span-1">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 uppercase tracking-wider">By Type</h3>
          {stats ? (
            <TypeDistribution data={stats.by_type} />
          ) : (
            <div className="space-y-3">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="h-6 bg-surface-2 rounded animate-pulse-soft" />
              ))}
            </div>
          )}
        </div>

        {/* Breakdowns */}
        <div className="card p-5">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 uppercase tracking-wider">By Scope</h3>
          {stats ? (
            <BreakdownBars data={stats.by_scope} />
          ) : (
            <div className="space-y-3">
              {Array.from({ length: 2 }).map((_, i) => (
                <div key={i} className="h-6 bg-surface-2 rounded animate-pulse-soft" />
              ))}
            </div>
          )}
        </div>

        <div className="card p-5">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 uppercase tracking-wider">By Agent</h3>
          {stats ? (
            <BreakdownBars data={stats.by_agent} />
          ) : (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-6 bg-surface-2 rounded animate-pulse-soft" />
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Recent memories */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold text-gray-300 uppercase tracking-wider">Recent Memories</h3>
          <Link to="/memories" className="text-xs text-brand-400 hover:text-brand-300 transition-colors">
            View all →
          </Link>
        </div>

        {recent === null ? (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
            {Array.from({ length: 3 }).map((_, i) => <SkeletonMemoryCard key={i} />)}
          </div>
        ) : recent.length === 0 ? (
          <EmptyState
            icon="🧠"
            title="No memories yet"
            description="Store your first memory to get started."
            action={<Link to="/memories" className="btn-primary text-sm">＋ Store Memory</Link>}
          />
        ) : (
          <div className="space-y-2">
            {recent.map(mem => (
              <Link
                key={mem.id}
                to="/memories"
                className="card-hover p-3 flex items-center gap-4 group"
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <h4 className="text-sm font-medium text-white truncate">{mem.title}</h4>
                    <TypeBadge type={mem.type} />
                  </div>
                  <p className="text-xs text-gray-500 truncate">{mem.content}</p>
                </div>
                <div className="flex items-center gap-3 shrink-0">
                  <ImportanceDot value={mem.importance} />
                  <TtlBadge ttlSeconds={mem.ttl_seconds} />
                  <span className="text-xs text-gray-600">{timeAgo(mem.created_at)}</span>
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function TypeDistribution({ data }) {
  const entries = Object.entries(data || {}).sort((a, b) => b[1] - a[1])
  const total = entries.reduce((sum, [, v]) => sum + v, 0)

  if (entries.length === 0) {
    return <div className="text-gray-600 text-sm">No data</div>
  }

  return (
    <div className="space-y-3">
      {entries.map(([type, count]) => {
        const pct = total ? (count / total) * 100 : 0
        return (
          <div key={type} className="group">
            <div className="flex items-center justify-between text-xs mb-1.5">
              <span className="text-gray-400 capitalize">{type.replace('_', ' ')}</span>
              <span className="text-gray-500 tabular-nums">{count} <span className="text-gray-600">({pct.toFixed(0)}%)</span></span>
            </div>
            <div className="h-2 bg-surface-2 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all duration-700 ${TYPE_COLORS[type] || 'bg-gray-500'}`}
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>
        )
      })}
    </div>
  )
}

function BreakdownBars({ data }) {
  const entries = Object.entries(data || {}).sort((a, b) => b[1] - a[1])
  const total = entries.reduce((sum, [, v]) => sum + v, 0)

  if (entries.length === 0) {
    return <div className="text-gray-600 text-sm">No data</div>
  }

  return (
    <div className="space-y-4">
      {entries.map(([key, count]) => {
        const pct = total ? (count / total) * 100 : 0
        return (
          <div key={key}>
            <div className="flex items-center justify-between text-xs mb-1.5">
              <span className="text-gray-400">{key}</span>
              <span className="text-gray-500 tabular-nums">{count}</span>
            </div>
            <div className="h-2 bg-surface-2 rounded-full overflow-hidden">
              <div
                className="h-full bg-brand-500 rounded-full transition-all duration-700"
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>
        )
      })}
    </div>
  )
}
