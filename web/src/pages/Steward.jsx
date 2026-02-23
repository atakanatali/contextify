import { useEffect, useMemo, useState } from 'react'
import { api } from '../api'
import EmptyState from '../components/EmptyState'
import ConfirmDialog from '../components/ConfirmDialog'
import { useToast } from '../hooks/useToast'

function get(obj, ...keys) {
  for (const key of keys) {
    if (obj && obj[key] !== undefined) return obj[key]
  }
  return undefined
}

function fmtDate(value) {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return String(value)
  return d.toLocaleString()
}

function fmtNumber(value) {
  if (value == null) return '—'
  return new Intl.NumberFormat().format(Number(value))
}

function fmtMs(value) {
  if (value == null) return '—'
  return `${fmtNumber(value)} ms`
}

function fmtTokens(value) {
  if (value == null) return '—'
  return `${fmtNumber(value)} tok`
}

function statusClass(status) {
  switch (status) {
    case 'succeeded':
      return 'bg-emerald-900/40 text-emerald-300 border-emerald-700/40'
    case 'failed':
    case 'dead_letter':
      return 'bg-red-900/40 text-red-300 border-red-700/40'
    case 'running':
      return 'bg-cyan-900/40 text-cyan-300 border-cyan-700/40'
    case 'queued':
      return 'bg-amber-900/40 text-amber-300 border-amber-700/40'
    case 'cancelled':
      return 'bg-slate-800 text-slate-300 border-slate-600/40'
    default:
      return 'bg-surface-2 text-gray-300 border-border'
  }
}

function hasRedaction(value) {
  if (value == null) return false
  if (typeof value === 'string') return value.toLowerCase().includes('redact')
  if (Array.isArray(value)) return value.some(hasRedaction)
  if (typeof value === 'object') return Object.values(value).some(hasRedaction)
  return false
}

function collectRedactionReasons(value, acc = new Set()) {
  if (value == null) return acc
  if (Array.isArray(value)) {
    value.forEach((v) => collectRedactionReasons(v, acc))
    return acc
  }
  if (typeof value === 'object') {
    const reasons = value._redaction_reasons
    if (Array.isArray(reasons)) reasons.forEach((r) => acc.add(String(r)))
    Object.values(value).forEach((v) => collectRedactionReasons(v, acc))
    return acc
  }
  if (typeof value === 'string' && value.includes('REDACTED')) acc.add('string_marker')
  return acc
}

function jsonText(v) {
  return JSON.stringify(v ?? {}, null, 2)
}

function parseDateInput(value, endOfDay = false) {
  if (!value) return null
  const d = new Date(`${value}T${endOfDay ? '23:59:59' : '00:00:00'}`)
  return Number.isNaN(d.getTime()) ? null : d.getTime()
}

function deriveRunView(run) {
  const id = get(run, 'ID', 'id')
  const jobId = get(run, 'JobID', 'job_id')
  const jobType = get(run, 'JobType', 'job_type') || '—'
  const projectId = get(run, 'ProjectID', 'project_id') || '—'
  const model = get(run, 'Model', 'model') || '—'
  const status = get(run, 'Status', 'status') || 'unknown'
  const totalTokens = get(run, 'TotalTokens', 'total_tokens')
  const latencyMs = get(run, 'LatencyMs', 'latency_ms')
  const createdAt = get(run, 'CreatedAt', 'created_at')
  const output = get(run, 'OutputSnapshot', 'output_snapshot') || {}
  const decision = output.decision || output.action || '—'
  const sideEffects = Array.isArray(output.side_effects) ? output.side_effects : []
  const sideEffectSummary = sideEffects.length
    ? sideEffects.map((s) => s.type || s.kind || s.action || 'effect').slice(0, 3).join(', ')
    : '—'
  return {
    raw: run,
    id,
    jobId,
    jobType,
    projectId,
    model,
    status,
    totalTokens,
    latencyMs,
    createdAt,
    output,
    decision,
    sideEffectSummary,
    input: get(run, 'InputSnapshot', 'input_snapshot') || {},
    errorClass: get(run, 'ErrorClass', 'error_class'),
    errorMessage: get(run, 'ErrorMessage', 'error_message'),
  }
}

function FilterInput({ label, children }) {
  return (
    <label className="space-y-1">
      <div className="text-[10px] uppercase tracking-wider text-gray-500">{label}</div>
      {children}
    </label>
  )
}

function StatCard({ label, value, sub, tone = 'text-white' }) {
  return (
    <div className="card p-4">
      <div className="text-[10px] uppercase tracking-wider text-gray-500 mb-1">{label}</div>
      <div className={`text-2xl font-semibold tabular-nums ${tone}`}>{value}</div>
      {sub ? <div className="text-xs text-gray-500 mt-1">{sub}</div> : null}
    </div>
  )
}

export default function Steward() {
  const toast = useToast()
  const [status, setStatus] = useState(null)
  const [metrics, setMetrics] = useState(null)
  const [runsResp, setRunsResp] = useState({ runs: [], limit: 50, offset: 0 })
  const [eventsResp, setEventsResp] = useState({ events: [] })
  const [selectedJobId, setSelectedJobId] = useState(null)
  const [selectedRunId, setSelectedRunId] = useState(null)
  const [loading, setLoading] = useState(true)
  const [eventsLoading, setEventsLoading] = useState(false)
  const [error, setError] = useState(null)
  const [eventsError, setEventsError] = useState(null)
  const [serverFilters, setServerFilters] = useState({ status: '', jobType: '', projectId: '', model: '' })
  const [clientFilters, setClientFilters] = useState({ from: '', to: '', tokenMin: '', tokenMax: '' })
  const [pagination, setPagination] = useState({ limit: 50, offset: 0 })
  const [refreshSeconds, setRefreshSeconds] = useState(0)
  const [actionLoading, setActionLoading] = useState('')
  const [confirmState, setConfirmState] = useState(null)
  const [lastAction, setLastAction] = useState(null)
  const [jobActionAudit, setJobActionAudit] = useState({})

  async function loadBase({ keepSelection = true } = {}) {
    setLoading(true)
    setError(null)
    try {
      const [s, m, runs] = await Promise.all([
        api.getStewardStatus(),
        api.getStewardMetrics(),
        api.getStewardRuns({
          ...serverFilters,
          limit: pagination.limit,
          offset: pagination.offset,
        }),
      ])
      setStatus(s)
      setMetrics(m)
      setRunsResp(runs)

      const list = Array.isArray(runs.runs) ? runs.runs : []
      if (!keepSelection || !list.some((r) => get(r, 'JobID', 'job_id') === selectedJobId && get(r, 'ID', 'id') === selectedRunId)) {
        const first = list[0]
        setSelectedJobId(get(first, 'JobID', 'job_id') || null)
        setSelectedRunId(get(first, 'ID', 'id') || null)
      }
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadBase({ keepSelection: false })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pagination.offset, pagination.limit])

  useEffect(() => {
    if (!selectedJobId) {
      setEventsResp({ events: [] })
      return
    }
    let cancelled = false
    setEventsLoading(true)
    setEventsError(null)
    api.getStewardJobEvents(selectedJobId)
      .then((resp) => {
        if (!cancelled) setEventsResp(resp)
      })
      .catch((e) => {
        if (!cancelled) setEventsError(e.message)
      })
      .finally(() => {
        if (!cancelled) setEventsLoading(false)
      })
    return () => { cancelled = true }
  }, [selectedJobId])

  useEffect(() => {
    if (!refreshSeconds) return undefined
    const timer = setInterval(() => {
      loadBase()
    }, refreshSeconds * 1000)
    return () => clearInterval(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refreshSeconds, serverFilters, pagination])

  const allRuns = useMemo(() => (Array.isArray(runsResp.runs) ? runsResp.runs.map(deriveRunView) : []), [runsResp])

  const filteredRuns = useMemo(() => {
    const fromTs = parseDateInput(clientFilters.from, false)
    const toTs = parseDateInput(clientFilters.to, true)
    const tokenMin = clientFilters.tokenMin === '' ? null : Number(clientFilters.tokenMin)
    const tokenMax = clientFilters.tokenMax === '' ? null : Number(clientFilters.tokenMax)

    return allRuns.filter((run) => {
      const created = run.createdAt ? new Date(run.createdAt).getTime() : null
      if (fromTs != null && created != null && created < fromTs) return false
      if (toTs != null && created != null && created > toTs) return false
      if (tokenMin != null && (run.totalTokens ?? -Infinity) < tokenMin) return false
      if (tokenMax != null && (run.totalTokens ?? Infinity) > tokenMax) return false
      return true
    })
  }, [allRuns, clientFilters])

  const selectedRun = useMemo(
    () => filteredRuns.find((r) => r.id === selectedRunId) || allRuns.find((r) => r.id === selectedRunId) || filteredRuns[0] || allRuns[0] || null,
    [filteredRuns, allRuns, selectedRunId],
  )

  useEffect(() => {
    if (!selectedRun) return
    if (selectedRun.jobId !== selectedJobId) setSelectedJobId(selectedRun.jobId)
    if (selectedRun.id !== selectedRunId) setSelectedRunId(selectedRun.id)
  }, [selectedRun, selectedJobId, selectedRunId])

  const kpis = {
    successRate: metrics ? `${((get(metrics, 'success_rate', 'SuccessRate') ?? 0) * 100).toFixed(1)}%` : '—',
    avgTokens: metrics ? fmtTokens(get(metrics, 'average_tokens_per_run', 'AverageTokensPerRun')) : '—',
    p95Latency: metrics ? fmtMs(get(metrics, 'p95_latency_ms', 'P95LatencyMs')) : '—',
    runsLastHour: metrics ? fmtNumber(get(metrics, 'runs_last_hour', 'RunsLastHour')) : '—',
  }

  const events = Array.isArray(eventsResp.events) ? eventsResp.events : []
  const selectedHasRedaction = selectedRun && (hasRedaction(selectedRun.input) || hasRedaction(selectedRun.output) || hasRedaction(events.map((e) => get(e, 'Data', 'data'))))
  const redactionReasons = selectedRun
    ? Array.from(collectRedactionReasons([selectedRun.input, selectedRun.output, events.map((e) => get(e, 'Data', 'data'))]))
    : []

  async function performAction(actionKey, jobId, fn, successMessage) {
    setActionLoading(actionKey)
    const actionAt = new Date().toISOString()
    try {
      await fn()
      const audit = {
        action_by: 'local-ui',
        action_at: actionAt,
        action_result: 'success',
        action_key: actionKey,
        job_id: jobId || null,
      }
      setLastAction(audit)
      if (jobId) {
        setJobActionAudit((prev) => ({ ...prev, [jobId]: audit }))
      }
      toast.success(successMessage)
      await loadBase()
      if (jobId) {
        setSelectedJobId(jobId)
      }
    } catch (e) {
      const audit = {
        action_by: 'local-ui',
        action_at: actionAt,
        action_result: 'failed',
        action_key: actionKey,
        job_id: jobId || null,
      }
      setLastAction(audit)
      if (jobId) {
        setJobActionAudit((prev) => ({ ...prev, [jobId]: audit }))
      }
      toast.error(e.message || 'Action failed')
    } finally {
      setActionLoading('')
    }
  }

  function requestConfirm(config) {
    setConfirmState(config)
  }

  return (
    <>
      <div className="space-y-6 animate-fade-in">
        <div className="flex flex-col xl:flex-row xl:items-end xl:justify-between gap-4">
        <div>
          <div className="text-xs uppercase tracking-[0.2em] text-gray-500 mb-2">Steward Console</div>
          <h1 className="text-2xl md:text-3xl font-semibold text-white">Runs, decisions, IO trace</h1>
          <p className="text-sm text-gray-400 mt-2">Operational debugging view for steward jobs, model calls, and side effects.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button className="btn-ghost" onClick={() => loadBase()} disabled={loading}>Refresh</button>
        </div>
      </div>

      {error ? (
        <div className="rounded-lg border border-red-800 bg-red-900/20 p-4 text-sm text-red-300">
          Failed to load steward console: {error}
        </div>
      ) : null}

      <div className="card p-4 md:p-5">
        <div className="flex flex-wrap items-center gap-2 mb-4">
          <span className="text-xs uppercase tracking-wider text-gray-500">Health</span>
          <span className={`text-xs px-2 py-1 rounded-md border ${get(status, 'enabled', 'Enabled') ? 'bg-emerald-900/40 text-emerald-300 border-emerald-700/40' : 'bg-slate-800 text-slate-300 border-slate-700/40'}`}>
            {get(status, 'enabled', 'Enabled') ? 'enabled' : 'disabled'}
          </span>
          <span className={`text-xs px-2 py-1 rounded-md border ${get(status, 'is_leader', 'IsLeader') ? 'bg-cyan-900/40 text-cyan-300 border-cyan-700/40' : 'bg-slate-800 text-slate-400 border-slate-700/40'}`}>
            {get(status, 'is_leader', 'IsLeader') ? 'leader' : 'follower'}
          </span>
          <span className={`text-xs px-2 py-1 rounded-md border ${get(status, 'paused', 'Paused') ? 'bg-amber-900/40 text-amber-300 border-amber-700/40' : 'bg-emerald-900/40 text-emerald-300 border-emerald-700/40'}`}>
            {get(status, 'paused', 'Paused') ? 'paused' : 'running'}
          </span>
          <span className={`text-xs px-2 py-1 rounded-md border ${get(status, 'dry_run', 'DryRun') ? 'bg-violet-900/40 text-violet-300 border-violet-700/40' : 'bg-slate-800 text-slate-300 border-slate-700/40'}`}>
            {get(status, 'dry_run', 'DryRun') ? 'dry-run' : 'write-enabled'}
          </span>
        </div>
        <div className="mt-4 flex flex-wrap items-center gap-2">
          <div className="text-xs uppercase tracking-wider text-gray-500 mr-1">Controls</div>
          <button
            className="btn-primary py-1.5"
            disabled={!!actionLoading}
            onClick={() => performAction('run_once', null, () => api.triggerStewardRunOnce(), 'Steward run-once triggered')}
          >
            ▶ Run Once
          </button>
          <button
            className="btn-ghost py-1.5"
            disabled={!!actionLoading}
            onClick={() => performAction(
              'mode_pause',
              null,
              () => api.updateStewardMode({ paused: true, dry_run: !!get(status, 'dry_run', 'DryRun') }),
              'Steward paused',
            )}
          >
            ⏸ Pause
          </button>
          <button
            className="btn-ghost py-1.5"
            disabled={!!actionLoading}
            onClick={() => requestConfirm({
              title: 'Resume Steward',
              message: 'Resuming will allow queued jobs to be processed immediately based on current mode. Continue?',
              confirmLabel: 'Resume',
              danger: false,
              onConfirm: () => performAction(
                'mode_resume',
                null,
                () => api.updateStewardMode({ paused: false, dry_run: !!get(status, 'dry_run', 'DryRun') }),
                'Steward resumed',
              ),
            })}
          >
            ▶ Resume
          </button>
          <button
            className="btn-ghost py-1.5"
            disabled={!!actionLoading}
            onClick={() => performAction(
              'mode_dry_run_on',
              null,
              () => api.updateStewardMode({ paused: !!get(status, 'paused', 'Paused'), dry_run: true }),
              'Dry-run enabled',
            )}
          >
            🧪 Dry-Run On
          </button>
          <button
            className="btn-ghost py-1.5"
            disabled={!!actionLoading}
            onClick={() => requestConfirm({
              title: 'Enable Writes',
              message: 'This disables dry-run and allows steward actions to apply writes. Continue?',
              confirmLabel: 'Enable Writes',
              danger: true,
              onConfirm: () => performAction(
                'mode_dry_run_off',
                null,
                () => api.updateStewardMode({ paused: !!get(status, 'paused', 'Paused'), dry_run: false }),
                'Write mode enabled',
              ),
            })}
          >
            ⚠ Enable Writes
          </button>
          <div className="ml-auto flex items-center gap-2">
            <span className="text-xs text-gray-500 uppercase tracking-wider">Auto refresh</span>
            <select
              className="input py-1.5 text-xs w-auto"
              value={refreshSeconds}
              onChange={(e) => setRefreshSeconds(Number(e.target.value))}
              disabled={!!actionLoading}
            >
              <option value={0}>Off</option>
              <option value={5}>5s</option>
              <option value={10}>10s</option>
              <option value={30}>30s</option>
            </select>
          </div>
        </div>
        {lastAction ? (
          <div className="mt-3 rounded-lg border border-border bg-surface-2 p-3 text-xs flex flex-wrap gap-x-4 gap-y-1">
            <span className="text-gray-500 uppercase tracking-wider">Last Action</span>
            <span className="text-gray-300">action_by: <span className="font-mono">local-ui</span></span>
            <span className="text-gray-300">action_at: <span className="font-mono">{fmtDate(lastAction.action_at)}</span></span>
            <span className={`${lastAction.action_result === 'success' ? 'text-emerald-300' : 'text-red-300'}`}>
              action_result: {lastAction.action_result}
            </span>
            <span className="text-gray-400">{lastAction.action_key}</span>
          </div>
        ) : null}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-sm">
          <div className="bg-surface-2 rounded-lg p-3 border border-border">
            <div className="text-xs uppercase tracking-wider text-gray-500 mb-1">Worker</div>
            <div className="text-gray-200 font-mono text-xs break-all">{get(status, 'worker_id', 'WorkerID') || '—'}</div>
          </div>
          <div className="bg-surface-2 rounded-lg p-3 border border-border">
            <div className="text-xs uppercase tracking-wider text-gray-500 mb-1">Model</div>
            <div className="text-gray-200">{get(status, 'model', 'Model') || '—'}</div>
          </div>
          <div className="bg-surface-2 rounded-lg p-3 border border-border">
            <div className="text-xs uppercase tracking-wider text-gray-500 mb-1">Tick Interval</div>
            <div className="text-gray-200">{String(get(status, 'tick_interval', 'TickInterval') ?? '—')}</div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 xl:grid-cols-4 gap-3">
        <StatCard label="Success Rate (24h)" value={kpis.successRate} tone="text-emerald-300" />
        <StatCard label="Avg Tokens / Run (24h)" value={kpis.avgTokens} tone="text-cyan-300" />
        <StatCard label="P95 Latency (24h)" value={kpis.p95Latency} tone="text-amber-300" />
        <StatCard label="Runs Last Hour" value={kpis.runsLastHour} tone="text-brand-300" />
      </div>

      <div className="card p-4 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-300">Run History</h2>
          <div className="text-xs text-gray-500">
            Server page: {fmtNumber(runsResp.offset || 0)} - {fmtNumber((runsResp.offset || 0) + ((runsResp.runs || []).length || 0))}
          </div>
        </div>

        <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-8 gap-3">
          <FilterInput label="Status">
            <select className="input text-sm" value={serverFilters.status} onChange={(e) => setServerFilters((p) => ({ ...p, status: e.target.value }))}>
              <option value="">All</option>
              <option value="queued">queued</option>
              <option value="running">running</option>
              <option value="succeeded">succeeded</option>
              <option value="failed">failed</option>
              <option value="dead_letter">dead_letter</option>
              <option value="cancelled">cancelled</option>
            </select>
          </FilterInput>
          <FilterInput label="Job Type">
            <input className="input" placeholder="derive_memories" value={serverFilters.jobType} onChange={(e) => setServerFilters((p) => ({ ...p, jobType: e.target.value }))} />
          </FilterInput>
          <FilterInput label="Project">
            <input className="input" placeholder="project_id" value={serverFilters.projectId} onChange={(e) => setServerFilters((p) => ({ ...p, projectId: e.target.value }))} />
          </FilterInput>
          <FilterInput label="Model">
            <input className="input" placeholder="llama..." value={serverFilters.model} onChange={(e) => setServerFilters((p) => ({ ...p, model: e.target.value }))} />
          </FilterInput>
          <FilterInput label="From Date">
            <input className="input" type="date" value={clientFilters.from} onChange={(e) => setClientFilters((p) => ({ ...p, from: e.target.value }))} />
          </FilterInput>
          <FilterInput label="To Date">
            <input className="input" type="date" value={clientFilters.to} onChange={(e) => setClientFilters((p) => ({ ...p, to: e.target.value }))} />
          </FilterInput>
          <FilterInput label="Min Tokens">
            <input className="input" type="number" min="0" value={clientFilters.tokenMin} onChange={(e) => setClientFilters((p) => ({ ...p, tokenMin: e.target.value }))} />
          </FilterInput>
          <FilterInput label="Max Tokens">
            <input className="input" type="number" min="0" value={clientFilters.tokenMax} onChange={(e) => setClientFilters((p) => ({ ...p, tokenMax: e.target.value }))} />
          </FilterInput>
        </div>

        <div className="flex flex-wrap gap-2">
          <button
            className="btn-primary"
            onClick={() => {
              setPagination((p) => ({ ...p, offset: 0 }))
              loadBase({ keepSelection: false })
            }}
            disabled={loading}
          >
            Apply Server Filters
          </button>
          <button
            className="btn-ghost"
            onClick={() => {
              setServerFilters({ status: '', jobType: '', projectId: '', model: '' })
              setClientFilters({ from: '', to: '', tokenMin: '', tokenMax: '' })
              setPagination((p) => ({ ...p, offset: 0 }))
            }}
          >
            Reset Filters
          </button>
        </div>

        <div className="grid grid-cols-1 xl:grid-cols-[1.25fr_0.75fr] gap-4 items-start">
          <div className="border border-border rounded-xl overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="bg-surface-2 text-gray-400 text-xs uppercase tracking-wider">
                  <tr>
                    <th className="px-3 py-2 text-left">Started</th>
                    <th className="px-3 py-2 text-left">Job Type</th>
                    <th className="px-3 py-2 text-left">Status</th>
                    <th className="px-3 py-2 text-left">Project</th>
                    <th className="px-3 py-2 text-left">Model</th>
                    <th className="px-3 py-2 text-right">Tokens</th>
                    <th className="px-3 py-2 text-right">Latency</th>
                    <th className="px-3 py-2 text-left">Decision</th>
                    <th className="px-3 py-2 text-left">Side Effects</th>
                    <th className="px-3 py-2 text-left">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {loading ? (
                    Array.from({ length: 6 }).map((_, i) => (
                      <tr key={i} className="border-t border-border">
                        <td colSpan={10} className="px-3 py-3">
                          <div className="h-6 rounded bg-surface-2 animate-pulse-soft" />
                        </td>
                      </tr>
                    ))
                  ) : filteredRuns.length === 0 ? (
                    <tr className="border-t border-border">
                      <td colSpan={10}>
                        <EmptyState icon="🧪" title="No steward runs match filters" description="Try broadening server/client filters or trigger a steward run." />
                      </td>
                    </tr>
                  ) : (
                    filteredRuns.map((run) => (
                      <tr
                        key={run.id}
                        className={`border-t border-border cursor-pointer hover:bg-surface-2/70 ${selectedRun?.id === run.id ? 'bg-brand-600/10' : ''}`}
                        onClick={() => {
                          setSelectedRunId(run.id)
                          setSelectedJobId(run.jobId)
                        }}
                      >
                        <td className="px-3 py-2 whitespace-nowrap text-gray-300">{fmtDate(run.createdAt)}</td>
                        <td className="px-3 py-2 text-gray-200">{run.jobType}</td>
                        <td className="px-3 py-2">
                          <span className={`text-xs px-2 py-0.5 rounded border ${statusClass(run.status)}`}>{run.status}</span>
                        </td>
                        <td className="px-3 py-2 text-gray-300 max-w-[180px] truncate" title={run.projectId}>{run.projectId}</td>
                        <td className="px-3 py-2 text-gray-300 max-w-[180px] truncate" title={run.model}>{run.model}</td>
                        <td className="px-3 py-2 text-right text-gray-200 tabular-nums">{fmtTokens(run.totalTokens)}</td>
                        <td className="px-3 py-2 text-right text-gray-200 tabular-nums">{fmtMs(run.latencyMs)}</td>
                        <td className="px-3 py-2 text-gray-200">{run.decision}</td>
                        <td className="px-3 py-2 text-gray-400 max-w-[220px] truncate" title={run.sideEffectSummary}>{run.sideEffectSummary}</td>
                        <td className="px-3 py-2">
                          <div className="flex flex-wrap items-center gap-1">
                            {(run.status === 'failed' || run.status === 'dead_letter' || run.status === 'cancelled') && run.jobId ? (
                              <button
                                className="btn-ghost py-1 px-2 text-xs"
                                disabled={!!actionLoading}
                                onClick={(e) => {
                                  e.stopPropagation()
                                  requestConfirm({
                                    title: 'Retry Steward Job',
                                    message: `Retry job ${run.jobId}? This will requeue the job for steward processing.`,
                                    confirmLabel: 'Retry Job',
                                    danger: false,
                                    onConfirm: () => performAction(`retry:${run.jobId}`, run.jobId, () => api.retryStewardJob(run.jobId), 'Job requeued'),
                                  })
                                }}
                              >
                                Retry
                              </button>
                            ) : null}
                            {(run.status === 'queued' || run.status === 'running') && run.jobId ? (
                              <button
                                className="btn-danger py-1 px-2 text-xs border border-red-800/60"
                                disabled={!!actionLoading}
                                onClick={(e) => {
                                  e.stopPropagation()
                                  requestConfirm({
                                    title: 'Cancel Steward Job',
                                    message: `Cancel job ${run.jobId}? This stops current/queued processing for this job.`,
                                    confirmLabel: 'Cancel Job',
                                    danger: true,
                                    onConfirm: () => performAction(`cancel:${run.jobId}`, run.jobId, () => api.cancelStewardJob(run.jobId), 'Job cancelled'),
                                  })
                                }}
                              >
                                Cancel
                              </button>
                            ) : null}
                            {jobActionAudit[run.jobId] ? (
                              <span className={`text-[10px] px-2 py-0.5 rounded border ${jobActionAudit[run.jobId].action_result === 'success' ? 'text-emerald-300 border-emerald-700/40 bg-emerald-900/20' : 'text-red-300 border-red-700/40 bg-red-900/20'}`}>
                                {jobActionAudit[run.jobId].action_result}
                              </span>
                            ) : null}
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            <div className="border-t border-border px-3 py-2 flex flex-wrap items-center justify-between gap-2 text-xs text-gray-400">
              <div>Visible {filteredRuns.length} of {(runsResp.runs || []).length} rows in current page</div>
              <div className="flex items-center gap-2">
                <select className="input py-1 text-xs w-auto" value={pagination.limit} onChange={(e) => setPagination({ limit: Number(e.target.value), offset: 0 })}>
                  <option value={25}>25</option>
                  <option value={50}>50</option>
                  <option value={100}>100</option>
                </select>
                <button className="btn-ghost py-1" onClick={() => setPagination((p) => ({ ...p, offset: Math.max(0, p.offset - p.limit) }))} disabled={loading || pagination.offset === 0}>Prev</button>
                <button className="btn-ghost py-1" onClick={() => setPagination((p) => ({ ...p, offset: p.offset + p.limit }))} disabled={loading || (runsResp.runs || []).length < pagination.limit}>Next</button>
              </div>
            </div>
          </div>

          <div className="card p-4 sticky top-16">
            {!selectedRun ? (
              <EmptyState icon="🧭" title="Select a run" description="Choose a row to inspect input/output snapshots and event timeline." />
            ) : (
              <div className="space-y-4">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="text-xs uppercase tracking-wider text-gray-500">Run Detail</div>
                    <div className="text-sm text-gray-200 mt-1 break-all font-mono">{selectedRun.id}</div>
                    <div className="text-xs text-gray-500 mt-1">Job {selectedRun.jobId || '—'}</div>
                  </div>
                  <span className={`text-xs px-2 py-1 rounded border ${statusClass(selectedRun.status)}`}>{selectedRun.status}</span>
                </div>

                {selectedRun?.jobId && jobActionAudit[selectedRun.jobId] ? (
                  <div className="rounded-lg border border-border bg-surface-2 p-3 text-xs space-y-1">
                    <div className="text-gray-500 uppercase tracking-wider">Action Audit</div>
                    <div className="text-gray-300">action_by: <span className="font-mono">{jobActionAudit[selectedRun.jobId].action_by}</span></div>
                    <div className="text-gray-300">action_at: <span className="font-mono">{fmtDate(jobActionAudit[selectedRun.jobId].action_at)}</span></div>
                    <div className={jobActionAudit[selectedRun.jobId].action_result === 'success' ? 'text-emerald-300' : 'text-red-300'}>
                      action_result: {jobActionAudit[selectedRun.jobId].action_result}
                    </div>
                  </div>
                ) : null}

                <div className="grid grid-cols-2 gap-2 text-xs">
                  <div className="bg-surface-2 border border-border rounded-lg p-2">
                    <div className="text-gray-500 uppercase tracking-wider mb-1">Tokens</div>
                    <div className="text-gray-200 tabular-nums">{fmtTokens(selectedRun.totalTokens)}</div>
                  </div>
                  <div className="bg-surface-2 border border-border rounded-lg p-2">
                    <div className="text-gray-500 uppercase tracking-wider mb-1">Latency</div>
                    <div className="text-gray-200 tabular-nums">{fmtMs(selectedRun.latencyMs)}</div>
                  </div>
                  <div className="bg-surface-2 border border-border rounded-lg p-2">
                    <div className="text-gray-500 uppercase tracking-wider mb-1">Decision</div>
                    <div className="text-gray-200">{selectedRun.decision}</div>
                  </div>
                  <div className="bg-surface-2 border border-border rounded-lg p-2">
                    <div className="text-gray-500 uppercase tracking-wider mb-1">Started</div>
                    <div className="text-gray-200">{fmtDate(selectedRun.createdAt)}</div>
                  </div>
                </div>

                {selectedHasRedaction ? (
                  <div className="rounded-lg border border-amber-700/40 bg-amber-900/20 text-amber-300 text-xs px-3 py-2 space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-[10px] px-2 py-0.5 rounded border border-amber-600/50 bg-amber-800/30 font-semibold tracking-wider">REDACTED</span>
                      <span>At least one payload field was redacted before persistence.</span>
                    </div>
                    {redactionReasons.length > 0 ? (
                      <div className="flex flex-wrap gap-1">
                        {redactionReasons.map((reason) => (
                          <span key={reason} className="text-[10px] px-2 py-0.5 rounded border border-amber-700/40 bg-amber-900/20">
                            {reason}
                          </span>
                        ))}
                      </div>
                    ) : null}
                  </div>
                ) : null}

                {(selectedRun.errorClass || selectedRun.errorMessage) ? (
                  <div className="rounded-lg border border-red-800 bg-red-900/20 p-3 text-xs">
                    <div className="text-red-300 font-medium">{selectedRun.errorClass || 'error'}</div>
                    <div className="text-red-200 mt-1 break-words">{selectedRun.errorMessage}</div>
                  </div>
                ) : null}

                <div>
                  <h3 className="text-xs uppercase tracking-wider text-gray-400 mb-2">Input Snapshot</h3>
                  <pre className="bg-surface-2 border border-border rounded-lg p-3 text-xs text-gray-300 overflow-auto max-h-56 whitespace-pre-wrap break-words">{jsonText(selectedRun.input)}</pre>
                </div>

                <div>
                  <h3 className="text-xs uppercase tracking-wider text-gray-400 mb-2">Model Output JSON</h3>
                  <pre className="bg-surface-2 border border-border rounded-lg p-3 text-xs text-gray-300 overflow-auto max-h-56 whitespace-pre-wrap break-words">{jsonText(selectedRun.output)}</pre>
                </div>

                <div>
                  <div className="flex items-center justify-between gap-2 mb-2">
                    <h3 className="text-xs uppercase tracking-wider text-gray-400">Event Timeline</h3>
                    {eventsLoading ? <span className="text-xs text-gray-500">Loading…</span> : null}
                  </div>
                  {eventsError ? (
                    <div className="text-xs text-red-300 bg-red-900/20 border border-red-800 rounded-lg p-2">Failed to load events: {eventsError}</div>
                  ) : events.length === 0 ? (
                    <div className="text-xs text-gray-500 bg-surface-2 border border-border rounded-lg p-3">No events for this job.</div>
                  ) : (
                    <div className="space-y-2 max-h-[22rem] overflow-auto pr-1">
                      {events.map((evt) => {
                        const evtType = get(evt, 'EventType', 'event_type') || 'event'
                        const evtAt = get(evt, 'CreatedAt', 'created_at')
                        const data = get(evt, 'Data', 'data') || {}
                        return (
                          <div key={get(evt, 'ID', 'id')} className="border border-border rounded-lg bg-surface-2 p-2">
                            <div className="flex items-center justify-between gap-2">
                              <div className="text-xs text-gray-200 font-medium">{evtType}</div>
                              <div className="text-[11px] text-gray-500">{fmtDate(evtAt)}</div>
                            </div>
                            <pre className="mt-2 text-[11px] text-gray-400 whitespace-pre-wrap break-words">{jsonText(data)}</pre>
                          </div>
                        )
                      })}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
        </div>
      </div>
      <ConfirmDialog
        open={!!confirmState}
        onClose={() => setConfirmState(null)}
        onConfirm={() => confirmState?.onConfirm?.()}
        title={confirmState?.title}
        message={confirmState?.message}
        confirmLabel={confirmState?.confirmLabel}
        danger={confirmState?.danger ?? true}
      />
    </>
  )
}
