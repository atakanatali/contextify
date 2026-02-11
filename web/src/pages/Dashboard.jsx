import { useState, useEffect } from 'react'
import { api } from '../api'

export default function Dashboard() {
  const [stats, setStats] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    api.getStats().then(setStats).catch((e) => setError(e.message))
  }, [])

  if (error) {
    return (
      <div className="text-red-400 bg-red-900/20 border border-red-800 rounded p-4">
        Failed to load stats: {error}
      </div>
    )
  }

  if (!stats) {
    return <div className="text-gray-500">Loading...</div>
  }

  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Dashboard</h2>

      {/* Stats grid */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        <StatCard label="Total Memories" value={stats.total_memories} />
        <StatCard label="Long-term" value={stats.long_term_count} color="text-indigo-400" />
        <StatCard label="Short-term" value={stats.short_term_count} color="text-yellow-400" />
        <StatCard label="Expiring Soon" value={stats.expiring_count} color="text-red-400" />
      </div>

      {/* Breakdown */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <BreakdownCard title="By Type" data={stats.by_type} />
        <BreakdownCard title="By Scope" data={stats.by_scope} />
        <BreakdownCard title="By Agent" data={stats.by_agent} />
      </div>
    </div>
  )
}

function StatCard({ label, value, color = 'text-white' }) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
      <div className="text-gray-400 text-xs uppercase tracking-wide mb-1">{label}</div>
      <div className={`text-3xl font-bold ${color}`}>{value}</div>
    </div>
  )
}

function BreakdownCard({ title, data }) {
  const entries = Object.entries(data || {}).sort((a, b) => b[1] - a[1])
  const total = entries.reduce((sum, [, v]) => sum + v, 0)

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
      <h3 className="text-sm font-medium text-gray-300 mb-3">{title}</h3>
      {entries.length === 0 ? (
        <div className="text-gray-600 text-sm">No data</div>
      ) : (
        <div className="space-y-2">
          {entries.map(([key, count]) => (
            <div key={key}>
              <div className="flex justify-between text-xs mb-1">
                <span className="text-gray-400">{key}</span>
                <span className="text-gray-500">{count}</span>
              </div>
              <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-indigo-500 rounded-full"
                  style={{ width: `${total ? (count / total) * 100 : 0}%` }}
                />
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
