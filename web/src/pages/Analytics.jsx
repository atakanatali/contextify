import { useState, useEffect } from 'react'
import { api } from '../api'
import { TypeBadge } from '../components/Badge'
import { StatCard as SkeletonStatCard } from '../components/Skeleton'
import EmptyState from '../components/EmptyState'
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, Legend,
  BarChart, Bar,
} from 'recharts'

const AGENT_COLORS = [
  '#06b6d4', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6',
  '#ec4899', '#14b8a6', '#f97316',
]

function formatNumber(n) {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

function CustomTooltip({ active, payload, label }) {
  if (!active || !payload?.length) return null
  return (
    <div className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-xs shadow-xl">
      <div className="text-gray-400 mb-1">{label}</div>
      {payload.map((p, i) => (
        <div key={i} className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full" style={{ background: p.color }} />
          <span className="text-gray-300">{p.name}:</span>
          <span className="text-white font-medium">{formatNumber(p.value)}</span>
        </div>
      ))}
    </div>
  )
}

export default function Analytics() {
  const [data, setData] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    api.getAnalytics()
      .then(setData)
      .catch(e => setError(e.message))
  }, [])

  if (error) {
    return (
      <div className="text-red-400 bg-red-900/20 border border-red-800 rounded-lg p-4 animate-fade-in">
        <span className="font-medium">Failed to load analytics:</span> {error}
      </div>
    )
  }

  const statCards = [
    { key: 'hits', icon: '🎯', label: 'Total Hits', color: 'from-brand-600 to-brand-700', value: data?.total_hits },
    { key: 'rate', icon: '📈', label: 'Hit Rate', color: 'from-emerald-600 to-emerald-700', value: data ? `${(data.hit_rate * 100).toFixed(1)}%` : null },
    { key: 'stored', icon: '💾', label: 'Tokens Stored', color: 'from-purple-600 to-purple-700', value: data?.total_tokens_stored },
    { key: 'saved', icon: '⚡', label: 'Tokens Saved', color: 'from-amber-600 to-amber-700', value: data?.total_tokens_saved },
  ]

  const agentEntries = data ? Object.entries(data.tokens_by_agent).sort((a, b) => b[1] - a[1]) : []
  const timelineData = data?.timeline?.map(e => ({
    ...e,
    date: e.date.slice(5), // "02-11" format
  })) || []

  return (
    <div className="space-y-8 animate-fade-in">
      {/* Stat cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {!data ? (
          Array.from({ length: 4 }).map((_, i) => <SkeletonStatCard key={i} />)
        ) : (
          statCards.map(({ key, icon, label, color, value }) => (
            <div key={key} className="card p-5">
              <div className="flex items-center gap-3 mb-3">
                <div className={`w-10 h-10 rounded-xl bg-gradient-to-br ${color} flex items-center justify-center text-lg shadow-lg`}>
                  {icon}
                </div>
                <span className="text-xs text-gray-500 uppercase tracking-wider font-medium">{label}</span>
              </div>
              <div className="text-3xl font-bold text-white tabular-nums">
                {typeof value === 'number' ? formatNumber(value) : value}
              </div>
            </div>
          ))
        )}
      </div>

      {/* Timeline chart */}
      <div className="card p-5">
        <h3 className="text-sm font-semibold text-gray-300 mb-4 uppercase tracking-wider">
          Activity — Last 30 Days
        </h3>
        {!data ? (
          <div className="h-64 bg-surface-2 rounded animate-pulse-soft" />
        ) : timelineData.length === 0 ? (
          <EmptyState icon="📊" title="No timeline data" description="Start storing memories to see activity." />
        ) : (
          <ResponsiveContainer width="100%" height={280}>
            <AreaChart data={timelineData} margin={{ top: 5, right: 10, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="colorCreated" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#06b6d4" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#06b6d4" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="colorHits" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
              <XAxis dataKey="date" tick={{ fill: '#94a3b8', fontSize: 11 }} tickLine={false} />
              <YAxis tick={{ fill: '#94a3b8', fontSize: 11 }} tickLine={false} axisLine={false} />
              <Tooltip content={<CustomTooltip />} />
              <Area type="monotone" dataKey="created" name="Created" stroke="#06b6d4" fillOpacity={1} fill="url(#colorCreated)" strokeWidth={2} />
              <Area type="monotone" dataKey="hits" name="Hits" stroke="#10b981" fillOpacity={1} fill="url(#colorHits)" strokeWidth={2} />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Two columns: Top memories + Agent breakdown */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        {/* Top accessed memories */}
        <div className="card p-5 xl:col-span-2">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 uppercase tracking-wider">
            Top Accessed Memories
          </h3>
          {!data ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="h-10 bg-surface-2 rounded animate-pulse-soft" />
              ))}
            </div>
          ) : data.top_accessed_memories.length === 0 ? (
            <EmptyState icon="🎯" title="No hits yet" description="Memory access counts will appear here." />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs text-gray-500 uppercase tracking-wider border-b border-border">
                    <th className="pb-2 pr-4">Memory</th>
                    <th className="pb-2 pr-4">Type</th>
                    <th className="pb-2 pr-4 text-right">Hits</th>
                    <th className="pb-2 pr-4 text-right">Tokens</th>
                    <th className="pb-2 text-right">Agent</th>
                  </tr>
                </thead>
                <tbody>
                  {data.top_accessed_memories.map((mem, i) => (
                    <tr key={mem.id} className="border-b border-border/50 hover:bg-surface-2/50 transition-colors">
                      <td className="py-2.5 pr-4">
                        <div className="flex items-center gap-2">
                          <span className="text-gray-500 text-xs w-5">{i + 1}.</span>
                          <span className="text-white truncate max-w-[280px]">{mem.title}</span>
                        </div>
                      </td>
                      <td className="py-2.5 pr-4">
                        <TypeBadge type={mem.type} />
                      </td>
                      <td className="py-2.5 pr-4 text-right tabular-nums text-brand-400 font-medium">
                        {mem.access_count}
                      </td>
                      <td className="py-2.5 pr-4 text-right tabular-nums text-gray-400">
                        {formatNumber(mem.token_count)}
                      </td>
                      <td className="py-2.5 text-right text-gray-500 text-xs">
                        {mem.agent_source || 'unknown'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Agent token breakdown */}
        <div className="card p-5">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 uppercase tracking-wider">
            Token Savings by Agent
          </h3>
          {!data ? (
            <div className="h-64 bg-surface-2 rounded animate-pulse-soft" />
          ) : agentEntries.length === 0 ? (
            <EmptyState icon="🤖" title="No agent data" description="Token savings per agent will appear here." />
          ) : (
            <>
              <ResponsiveContainer width="100%" height={220}>
                <PieChart>
                  <Pie
                    data={agentEntries.map(([name, value]) => ({ name, value }))}
                    cx="50%"
                    cy="50%"
                    innerRadius={50}
                    outerRadius={80}
                    paddingAngle={3}
                    dataKey="value"
                  >
                    {agentEntries.map((_, i) => (
                      <Cell key={i} fill={AGENT_COLORS[i % AGENT_COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip
                    content={({ active, payload }) => {
                      if (!active || !payload?.length) return null
                      const { name, value } = payload[0].payload
                      return (
                        <div className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-xs shadow-xl">
                          <div className="text-white font-medium">{name}</div>
                          <div className="text-gray-400">{formatNumber(value)} tokens saved</div>
                        </div>
                      )
                    }}
                  />
                </PieChart>
              </ResponsiveContainer>
              <div className="space-y-2 mt-2">
                {agentEntries.map(([agent, tokens], i) => (
                  <div key={agent} className="flex items-center justify-between text-xs">
                    <div className="flex items-center gap-2">
                      <div className="w-2.5 h-2.5 rounded-full" style={{ background: AGENT_COLORS[i % AGENT_COLORS.length] }} />
                      <span className="text-gray-400">{agent}</span>
                    </div>
                    <span className="text-gray-300 tabular-nums font-medium">{formatNumber(tokens)}</span>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
