import { useState } from 'react'
import { api } from '../api'
import MemoryCard from '../components/MemoryCard'
import Modal from '../components/Modal'
import EmptyState from '../components/EmptyState'
import { MemoryCard as SkeletonMemoryCard } from '../components/Skeleton'

const TABS = [
  { key: 'semantic', label: 'Semantic', icon: '🧠', placeholder: 'Ask in natural language... e.g. "how did we fix the auth bug?"' },
  { key: 'keyword', label: 'Keyword', icon: '🔤', placeholder: 'Search by exact keywords, tags, or terms...' },
]

export default function Search() {
  const [query, setQuery] = useState('')
  const [tab, setTab] = useState('semantic')
  const [results, setResults] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [searched, setSearched] = useState(false)

  // Detail modal
  const [detailMemory, setDetailMemory] = useState(null)

  const handleSearch = async () => {
    if (!query.trim()) return
    setLoading(true)
    setError(null)
    setSearched(true)
    try {
      const data = tab === 'semantic'
        ? await api.recallMemories(query.trim())
        : await api.searchMemories(query.trim())
      setResults(data || [])
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') handleSearch()
  }

  const handleDelete = (id) => {
    setResults(prev => prev.filter(r => r.memory.id !== id))
  }

  const handlePromote = (id) => {
    setResults(prev => prev.map(r =>
      r.memory.id === id ? { ...r, memory: { ...r.memory, ttl_seconds: null, expires_at: null } } : r
    ))
  }

  const maxScore = results.length > 0 ? Math.max(...results.map(r => r.score || 0)) : 1

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <h2 className="text-xl font-bold text-white">Search</h2>
        <p className="text-sm text-gray-500 mt-0.5">Find memories using semantic or keyword search</p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 p-1 bg-surface-1 border border-border rounded-lg w-fit">
        {TABS.map(t => (
          <button
            key={t.key}
            onClick={() => { setTab(t.key); setResults([]); setSearched(false) }}
            className={`px-4 py-2 text-sm font-medium rounded-md transition-all duration-200 ${
              tab === t.key
                ? 'bg-brand-600 text-white shadow-lg shadow-brand-900/30'
                : 'text-gray-400 hover:text-white hover:bg-surface-2'
            }`}
          >
            <span className="mr-1.5">{t.icon}</span>
            {t.label}
          </button>
        ))}
      </div>

      {/* Search bar */}
      <div className="flex gap-2">
        <div className="relative flex-1">
          <svg
            className="absolute left-3.5 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-500 pointer-events-none"
            fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
          </svg>
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={TABS.find(t => t.key === tab)?.placeholder}
            autoFocus
            className="input pl-11 pr-9 py-3 text-base"
          />
          {query && (
            <button
              onClick={() => { setQuery(''); setResults([]); setSearched(false) }}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300 transition-colors"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
              </svg>
            </button>
          )}
        </div>
        <button
          onClick={handleSearch}
          disabled={loading || !query.trim()}
          className="btn-primary px-6 py-3 text-base"
        >
          {loading ? (
            <span className="inline-block w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
          ) : (
            tab === 'semantic' ? 'Recall' : 'Search'
          )}
        </button>
      </div>

      {/* Error */}
      {error && (
        <div className="text-red-400 bg-red-900/20 border border-red-800 rounded-lg p-3 text-sm animate-fade-in">
          {error}
        </div>
      )}

      {/* Results */}
      {loading ? (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => <SkeletonMemoryCard key={i} />)}
        </div>
      ) : results.length === 0 && searched ? (
        <EmptyState
          icon="🔮"
          title="No results found"
          description={`No memories match "${query}". Try different keywords or switch search mode.`}
        />
      ) : !searched && results.length === 0 ? (
        <div className="text-center py-16">
          <div className="text-5xl opacity-30 mb-4">{tab === 'semantic' ? '🧠' : '🔤'}</div>
          <p className="text-gray-500 text-sm">
            {tab === 'semantic'
              ? 'Ask a question in natural language to find related memories'
              : 'Enter keywords to search through memory titles and content'}
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {/* Result count */}
          <div className="flex items-center justify-between">
            <span className="text-xs text-gray-500">
              {results.length} result{results.length !== 1 ? 's' : ''} found
            </span>
            <span className="text-xs text-gray-600">
              {tab === 'semantic' ? 'Ranked by semantic similarity' : 'Ranked by keyword relevance'}
            </span>
          </div>

          {/* Result cards with score */}
          {results.map((r, idx) => (
            <div key={r.memory.id} className="animate-fade-in" style={{ animationDelay: `${idx * 50}ms` }}>
              {/* Score bar header */}
              <div className="flex items-center gap-3 mb-1.5 px-1">
                <span className="text-xs text-gray-600 tabular-nums w-6 shrink-0">#{idx + 1}</span>
                <div className="flex-1 h-1 bg-surface-2 rounded-full overflow-hidden">
                  <div
                    className="h-full rounded-full transition-all duration-700 bg-gradient-to-r from-brand-500 to-accent-400"
                    style={{ width: `${maxScore ? (r.score / maxScore) * 100 : 0}%` }}
                  />
                </div>
                <span className="text-xs text-gray-500 tabular-nums shrink-0">{r.score?.toFixed(3)}</span>
                <span className={`text-xs px-1.5 py-0.5 rounded font-medium ${
                  r.match_type === 'semantic' ? 'text-brand-400 bg-brand-900/30'
                  : r.match_type === 'keyword' ? 'text-amber-400 bg-amber-900/30'
                  : 'text-purple-400 bg-purple-900/30'
                }`}>
                  {r.match_type}
                </span>
              </div>

              <MemoryCard
                memory={r.memory}
                onClick={(mem) => setDetailMemory(mem)}
                onDelete={handleDelete}
                onPromote={handlePromote}
              />
            </div>
          ))}
        </div>
      )}

      {/* Detail Modal */}
      <Modal
        open={!!detailMemory}
        onClose={() => setDetailMemory(null)}
        title={detailMemory?.title || 'Memory Detail'}
        size="lg"
      >
        {detailMemory && <SearchDetailView memory={detailMemory} />}
      </Modal>
    </div>
  )
}

function SearchDetailView({ memory }) {
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        <span className="text-xs px-2 py-0.5 rounded-md border font-medium bg-brand-900/50 text-brand-300 border-brand-700/40">
          {memory.type?.replace('_', ' ')}
        </span>
        <span className="text-xs px-2 py-0.5 rounded-md border font-medium bg-surface-2 text-gray-400 border-border">
          {memory.scope}
        </span>
      </div>
      <div className="bg-surface-2 rounded-lg p-4 text-sm text-gray-300 whitespace-pre-wrap leading-relaxed">
        {memory.content}
      </div>
      {memory.summary && (
        <p className="text-sm text-gray-400"><span className="text-gray-500 font-medium">Summary:</span> {memory.summary}</p>
      )}
      {memory.tags?.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {memory.tags.map(tag => (
            <span key={tag} className="text-xs px-2 py-0.5 rounded-md bg-surface-2 text-gray-400 border border-border">#{tag}</span>
          ))}
        </div>
      )}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
        <div><span className="text-gray-500">Importance:</span> <span className="text-gray-300">{memory.importance?.toFixed(1)}</span></div>
        <div><span className="text-gray-500">Accesses:</span> <span className="text-gray-300">{memory.access_count}</span></div>
        <div><span className="text-gray-500">TTL:</span> <span className="text-gray-300">{memory.ttl_seconds == null ? 'Permanent' : `${Math.round(memory.ttl_seconds / 3600)}h`}</span></div>
        <div><span className="text-gray-500">Agent:</span> <span className="text-gray-300">{memory.agent_source || 'unknown'}</span></div>
      </div>
      <div className="pt-2 border-t border-border">
        <span className="text-xs text-gray-600 font-mono select-all">{memory.id}</span>
      </div>
    </div>
  )
}
