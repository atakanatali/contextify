import { useState, useEffect, useCallback } from 'react'
import { api } from '../api'
import { useToast } from '../hooks/useToast'
import SuggestionCard from '../components/SuggestionCard'
import MemoryCompare from '../components/MemoryCompare'
import EmptyState from '../components/EmptyState'

const STRATEGIES = [
  { value: 'smart_merge', label: 'Smart Merge' },
  { value: 'latest_wins', label: 'Latest Wins' },
  { value: 'append', label: 'Append' },
]

export default function Consolidation() {
  const [suggestions, setSuggestions] = useState(null)
  const [total, setTotal] = useState(0)
  const [selected, setSelected] = useState(null)
  const [strategy, setStrategy] = useState('smart_merge')
  const [merging, setMerging] = useState(false)
  const [tab, setTab] = useState('suggestions') // 'suggestions' | 'log'
  const [logs, setLogs] = useState(null)
  const [error, setError] = useState(null)
  const toast = useToast()

  const loadSuggestions = useCallback(async () => {
    try {
      const data = await api.getSuggestions({ status: 'pending', limit: 50 })
      setSuggestions(data.suggestions || [])
      setTotal(data.total || 0)
    } catch (e) {
      setError(e.message)
    }
  }, [])

  const loadLogs = useCallback(async () => {
    try {
      const data = await api.getConsolidationLog({ limit: 30 })
      setLogs(data || [])
    } catch (e) {
      setError(e.message)
    }
  }, [])

  useEffect(() => {
    loadSuggestions()
  }, [loadSuggestions])

  useEffect(() => {
    if (tab === 'log' && logs === null) loadLogs()
  }, [tab, logs, loadLogs])

  async function handleDismiss(id) {
    try {
      await api.updateSuggestion(id, 'dismissed')
      setSuggestions(prev => prev.filter(s => s.id !== id))
      setTotal(t => t - 1)
      if (selected?.id === id) setSelected(null)
      toast.success('Suggestion dismissed')
    } catch (e) {
      toast.error('Failed to dismiss: ' + e.message)
    }
  }

  async function handleMerge(targetMemory, sourceMemory) {
    setMerging(true)
    try {
      await api.mergeMemories(targetMemory.id, [sourceMemory.id], strategy)
      // Mark suggestion as accepted
      if (selected) {
        await api.updateSuggestion(selected.id, 'accepted').catch(() => {})
      }
      setSuggestions(prev => prev.filter(s => s.id !== selected?.id))
      setTotal(t => t - 1)
      setSelected(null)
      toast.success('Memories merged successfully')
    } catch (e) {
      toast.error('Merge failed: ' + e.message)
    } finally {
      setMerging(false)
    }
  }

  if (error) {
    return (
      <div className="text-red-400 bg-red-900/20 border border-red-800 rounded-lg p-4 animate-fade-in">
        <span className="font-medium">Error:</span> {error}
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white">Consolidation</h1>
          <p className="text-sm text-gray-500 mt-1">Review and merge duplicate memories</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setTab('suggestions')}
            className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
              tab === 'suggestions' ? 'bg-brand-600/20 text-brand-300' : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            Suggestions {total > 0 && <span className="ml-1 text-xs bg-brand-600/30 rounded-full px-1.5 py-0.5">{total}</span>}
          </button>
          <button
            onClick={() => setTab('log')}
            className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
              tab === 'log' ? 'bg-brand-600/20 text-brand-300' : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            Merge Log
          </button>
        </div>
      </div>

      {tab === 'suggestions' && (
        <SuggestionsView
          suggestions={suggestions}
          selected={selected}
          onSelect={setSelected}
          onDismiss={handleDismiss}
          onMerge={handleMerge}
          strategy={strategy}
          onStrategyChange={setStrategy}
          merging={merging}
        />
      )}

      {tab === 'log' && <LogView logs={logs} />}
    </div>
  )
}

function SuggestionsView({ suggestions, selected, onSelect, onDismiss, onMerge, strategy, onStrategyChange, merging }) {
  if (suggestions === null) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="card p-4 h-28 animate-pulse-soft" />
        ))}
      </div>
    )
  }

  if (suggestions.length === 0) {
    return (
      <EmptyState
        icon="🔗"
        title="No pending suggestions"
        description="The dedup scanner hasn't found any duplicate memories. Suggestions will appear here when similar memories are detected."
      />
    )
  }

  return (
    <div className="grid grid-cols-1 xl:grid-cols-5 gap-6">
      {/* Left panel: suggestion list */}
      <div className="xl:col-span-2 space-y-3 max-h-[70vh] overflow-y-auto pr-1">
        {suggestions.map(s => (
          <SuggestionCard
            key={s.id}
            suggestion={s}
            selected={selected?.id === s.id}
            onSelect={onSelect}
            onDismiss={onDismiss}
          />
        ))}
      </div>

      {/* Right panel: comparison + actions */}
      <div className="xl:col-span-3 flex flex-col">
        <div className="card p-5 flex-1 min-h-[300px]">
          <MemoryCompare
            memoryA={selected?.memory_a}
            memoryB={selected?.memory_b}
          />
        </div>

        {selected && (
          <div className="card p-4 mt-3 flex items-center gap-3 flex-wrap">
            <select
              value={strategy}
              onChange={e => onStrategyChange(e.target.value)}
              className="bg-surface-2 border border-border rounded-lg px-3 py-1.5 text-sm text-gray-300 focus:outline-none focus:border-brand-500/50"
            >
              {STRATEGIES.map(s => (
                <option key={s.value} value={s.value}>{s.label}</option>
              ))}
            </select>

            <div className="flex-1" />

            <button
              onClick={() => onDismiss(selected.id)}
              className="btn-ghost text-sm"
              disabled={merging}
            >
              Keep Both
            </button>
            <button
              onClick={() => onMerge(selected.memory_a, selected.memory_b)}
              className="btn-primary text-sm"
              disabled={merging}
            >
              {merging ? 'Merging...' : 'Merge into A'}
            </button>
            <button
              onClick={() => onMerge(selected.memory_b, selected.memory_a)}
              className="btn-primary text-sm bg-violet-600 hover:bg-violet-500"
              disabled={merging}
            >
              {merging ? 'Merging...' : 'Merge into B'}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}

function LogView({ logs }) {
  if (logs === null) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="card p-4 h-20 animate-pulse-soft" />
        ))}
      </div>
    )
  }

  if (logs.length === 0) {
    return (
      <EmptyState
        icon="📋"
        title="No merge history"
        description="Merge operations will be logged here for audit purposes."
      />
    )
  }

  return (
    <div className="space-y-2">
      {logs.map(log => (
        <div key={log.id} className="card p-4">
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <span className="text-xs font-mono text-gray-500">{log.target_id?.slice(0, 8)}</span>
              <span className="text-xs px-2 py-0.5 rounded-md bg-surface-2 text-gray-400 border border-border">
                {log.merge_strategy}
              </span>
              {log.similarity_score && (
                <span className="text-xs text-gray-500">
                  {(log.similarity_score * 100).toFixed(1)}% similarity
                </span>
              )}
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-600">{log.performed_by}</span>
              <span className="text-xs text-gray-600">
                {new Date(log.created_at).toLocaleString()}
              </span>
            </div>
          </div>
          <div className="flex items-center gap-2 text-xs text-gray-500">
            <span>Merged {log.source_ids?.length || 0} source(s)</span>
            <span className="text-gray-700">|</span>
            <span>Content: {log.content_before?.length || 0} chars {'->'} {log.content_after?.length || 0} chars</span>
          </div>
        </div>
      ))}
    </div>
  )
}
