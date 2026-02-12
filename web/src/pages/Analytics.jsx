import { useState, useEffect, useCallback } from 'react'
import { api } from '../api'
import { TypeBadge } from '../components/Badge'
import { StatCard as SkeletonStatCard } from '../components/Skeleton'
import EmptyState from '../components/EmptyState'
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell,
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
  const [baseError, setBaseError] = useState(null)
  const [funnel, setFunnel] = useState(null)
  const [funnelLoading, setFunnelLoading] = useState(true)
  const [funnelError, setFunnelError] = useState(null)
  const [filters, setFilters] = useState({
    days: 30,
    from: '',
    to: '',
    agentSource: '',
    projectId: '',
  })
  const [appliedFilters, setAppliedFilters] = useState({
    days: 30,
    from: '',
    to: '',
    agentSource: '',
    projectId: '',
  })

  const loadAnalytics = useCallback(() => {
    api.getAnalytics()
      .then(setData)
      .catch(e => setBaseError(e.message))
  }, [])

  const loadFunnel = useCallback(() => {
    setFunnelLoading(true)
    const hasDateRange = appliedFilters.from && appliedFilters.to

    api.getFunnelAnalytics({
      ...(hasDateRange ? { from: appliedFilters.from, to: appliedFilters.to } : { days: appliedFilters.days }),
      ...(appliedFilters.agentSource ? { agentSource: appliedFilters.agentSource } : {}),
      ...(appliedFilters.projectId ? { projectId: appliedFilters.projectId } : {}),
    })
      .then(setFunnel)
      .catch(e => setFunnelError(e.message))
      .finally(() => setFunnelLoading(false))
  }, [appliedFilters])

  useEffect(() => {
    loadAnalytics()
  }, [loadAnalytics])

  useEffect(() => {
    loadFunnel()
  }, [loadFunnel])

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
  const funnelTimeline = funnel?.timeline?.map(e => ({
    ...e,
    date: e.date.slice(5),
  })) || []

  const funnelCards = [
    { key: 'recall_attempts', label: 'Recall Attempts', value: funnel?.recall_attempts ?? 0, color: 'text-cyan-300' },
    { key: 'recall_hits', label: 'Recall Hits', value: funnel?.recall_hits ?? 0, color: 'text-emerald-300' },
    { key: 'hit_rate', label: 'Recall Hit Rate', value: `${((funnel?.recall_hit_rate ?? 0) * 100).toFixed(1)}%`, color: 'text-brand-300' },
    { key: 'store_opp', label: 'Store Opportunities', value: funnel?.store_opportunities ?? 0, color: 'text-amber-300' },
    { key: 'store_actions', label: 'Store Actions', value: funnel?.store_actions ?? 0, color: 'text-violet-300' },
    { key: 'capture_rate', label: 'Store Capture Rate', value: `${((funnel?.store_capture_rate ?? 0) * 100).toFixed(1)}%`, color: 'text-fuchsia-300' },
  ]

  function applyFilters() {
    if ((filters.from && !filters.to) || (!filters.from && filters.to)) {
      setFunnelError('Both from and to dates are required when using explicit date range.')
      return
    }
    setFunnelError(null)
    setAppliedFilters(filters)
  }

  return (
    <div className="space-y-8 animate-fade-in">
      {baseError && (
        <div className="text-red-400 bg-red-900/20 border border-red-800 rounded-lg p-4">
          <span className="font-medium">Failed to load analytics:</span> {baseError}
        </div>
      )}

      {funnelError && (
        <div className="text-red-400 bg-red-900/20 border border-red-800 rounded-lg p-4">
          <span className="font-medium">Failed to load funnel:</span> {funnelError}
        </div>
      )}

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

      {/* Funnel analytics */}
      <div className="card p-5 space-y-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h3 className="text-sm font-semibold text-gray-300 uppercase tracking-wider">Recall & Store Funnel</h3>
          <div className="flex flex-wrap items-center gap-2">
            <select
              className="input w-auto text-xs py-1.5"
              value={filters.days}
              onChange={(e) => setFilters((prev) => ({ ...prev, days: Number(e.target.value) }))}
            >
              <option value={7}>Last 7 days</option>
              <option value={30}>Last 30 days</option>
              <option value={90}>Last 90 days</option>
            </select>
            <input
              type="date"
              value={filters.from}
              onChange={(e) => setFilters((prev) => ({ ...prev, from: e.target.value }))}
              className="input w-auto text-xs py-1.5"
            />
            <input
              type="date"
              value={filters.to}
              onChange={(e) => setFilters((prev) => ({ ...prev, to: e.target.value }))}
              className="input w-auto text-xs py-1.5"
            />
            <input
              type="text"
              placeholder="agent_source"
              value={filters.agentSource}
              onChange={(e) => setFilters((prev) => ({ ...prev, agentSource: e.target.value }))}
              className="input w-[140px] text-xs py-1.5"
            />
            <input
              type="text"
              placeholder="project_id"
              value={filters.projectId}
              onChange={(e) => setFilters((prev) => ({ ...prev, projectId: e.target.value }))}
              className="input w-[200px] text-xs py-1.5"
            />
            <button onClick={applyFilters} className="btn-primary text-xs py-1.5">Apply</button>
          </div>
        </div>

        {funnelLoading ? (
          <div className="space-y-3">
            <div className="grid grid-cols-2 lg:grid-cols-6 gap-3">
              {Array.from({ length: 6 }).map((_, i) => <div key={i} className="h-20 bg-surface-2 rounded animate-pulse-soft" />)}
            </div>
            <div className="h-64 bg-surface-2 rounded animate-pulse-soft" />
          </div>
        ) : (
          <>
            <div className="grid grid-cols-2 lg:grid-cols-6 gap-3">
              {funnelCards.map(card => (
                <div key={card.key} className="bg-surface-2 border border-border rounded-lg p-3">
                  <div className="text-[10px] uppercase tracking-wider text-gray-500 mb-1">{card.label}</div>
                  <div className={`text-lg font-semibold tabular-nums ${card.color}`}>
                    {typeof card.value === 'number' ? formatNumber(card.value) : card.value}
                  </div>
                </div>
              ))}
            </div>

            {funnelTimeline.length === 0 ? (
              <EmptyState icon="📉" title="No funnel events yet" description="Recall/store telemetry events will appear here as agents run." />
            ) : (
              <ResponsiveContainer width="100%" height={260}>
                <AreaChart data={funnelTimeline} margin={{ top: 5, right: 10, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="colorRecallAttempts" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#06b6d4" stopOpacity={0.35} />
                      <stop offset="95%" stopColor="#06b6d4" stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="colorRecallHits" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#10b981" stopOpacity={0.35} />
                      <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                  <XAxis dataKey="date" tick={{ fill: '#94a3b8', fontSize: 11 }} tickLine={false} />
                  <YAxis tick={{ fill: '#94a3b8', fontSize: 11 }} tickLine={false} axisLine={false} />
                  <Tooltip content={<CustomTooltip />} />
                  <Area type="monotone" dataKey="recall_attempts" name="Recall Attempts" stroke="#06b6d4" fillOpacity={1} fill="url(#colorRecallAttempts)" strokeWidth={2} />
                  <Area type="monotone" dataKey="recall_hits" name="Recall Hits" stroke="#10b981" fillOpacity={1} fill="url(#colorRecallHits)" strokeWidth={2} />
                </AreaChart>
              </ResponsiveContainer>
            )}

            <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
              <div className="bg-surface-2 border border-border rounded-lg p-4">
                <h4 className="text-xs uppercase tracking-wider text-gray-400 mb-3">By Agent</h4>
                {funnel?.by_agent?.length ? (
                  <div className="space-y-2">
                    {funnel.by_agent.slice(0, 8).map((row) => (
                      <div key={row.key} className="flex items-center justify-between text-xs">
                        <span className="text-gray-300 truncate pr-2">{row.key}</span>
                        <span className="text-gray-500 tabular-nums">
                          {row.recall_hits}/{row.recall_attempts} recalls
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="text-xs text-gray-500">No agent breakdown data</div>
                )}
              </div>
              <div className="bg-surface-2 border border-border rounded-lg p-4">
                <h4 className="text-xs uppercase tracking-wider text-gray-400 mb-3">By Project</h4>
                {funnel?.by_project?.length ? (
                  <div className="space-y-2">
                    {funnel.by_project.slice(0, 8).map((row) => (
                      <div key={row.key} className="flex items-center justify-between text-xs">
                        <span className="text-gray-300 truncate pr-2">{row.key}</span>
                        <span className="text-gray-500 tabular-nums">
                          {row.store_actions}/{row.store_opportunities} stores
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="text-xs text-gray-500">No project breakdown data</div>
                )}
              </div>
            </div>
          </>
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
