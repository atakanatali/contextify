import { useState, useEffect, useCallback } from 'react'
import { api } from '../api'
import MemoryCard from '../components/MemoryCard'
import SearchBar from '../components/SearchBar'
import Modal from '../components/Modal'
import EmptyState from '../components/EmptyState'
import StoreMemoryForm from '../components/StoreMemoryForm'
import MemoryDetail from '../components/MemoryDetail'
import { MemoryCard as SkeletonMemoryCard } from '../components/Skeleton'

const PAGE_SIZE = 12

export default function MemoryBrowser() {
  const [results, setResults] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [query, setQuery] = useState('')
  const [filters, setFilters] = useState({})
  const [sort, setSort] = useState('relevance')
  const [offset, setOffset] = useState(0)
  const [hasMore, setHasMore] = useState(false)

  // Store memory modal
  const [storeOpen, setStoreOpen] = useState(false)

  // Detail modal
  const [detailMemory, setDetailMemory] = useState(null)

  const fetchMemories = useCallback(async (opts = {}) => {
    const q = opts.query ?? query
    const f = opts.filters ?? filters
    const o = opts.offset ?? offset
    setLoading(true)
    setError(null)
    try {
      const data = await api.listMemories({
        query: q || '*',
        type: f.type,
        scope: f.scope,
        limit: PAGE_SIZE,
        offset: o,
        sort: opts.sort ?? sort,
      })
      const items = data || []
      setResults(items)
      setHasMore(items.length >= PAGE_SIZE)
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [query, filters, offset, sort])

  useEffect(() => {
    fetchMemories()
  }, []) // initial load

  const handleSearch = () => {
    setOffset(0)
    fetchMemories({ offset: 0 })
  }

  const handleFilterChange = (newFilters) => {
    setFilters(newFilters)
    setOffset(0)
    fetchMemories({ filters: newFilters, offset: 0 })
  }

  const handleSortChange = (newSort) => {
    setSort(newSort)
    setOffset(0)
    fetchMemories({ sort: newSort, offset: 0 })
  }

  const handlePrev = () => {
    const newOffset = Math.max(0, offset - PAGE_SIZE)
    setOffset(newOffset)
    fetchMemories({ offset: newOffset })
  }

  const handleNext = () => {
    const newOffset = offset + PAGE_SIZE
    setOffset(newOffset)
    fetchMemories({ offset: newOffset })
  }

  const handleDelete = (id) => {
    setResults(prev => prev.filter(r => r.memory.id !== id))
  }

  const handlePromote = (id) => {
    setResults(prev => prev.map(r =>
      r.memory.id === id ? { ...r, memory: { ...r.memory, ttl_seconds: null, expires_at: null } } : r
    ))
  }

  const handleMemoryCreated = () => {
    setStoreOpen(false)
    fetchMemories({ offset: 0 })
    setOffset(0)
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold text-white">Memories</h2>
          <p className="text-sm text-gray-500 mt-0.5">Browse, search and manage your memories</p>
        </div>
        <button onClick={() => setStoreOpen(true)} className="btn-primary shrink-0">
          <span>＋</span> New Memory
        </button>
      </div>

      {/* Search */}
      <SearchBar
        value={query}
        onChange={setQuery}
        onSearch={handleSearch}
        placeholder="Search memories by title, content or tags..."
        showFilters
        showSort
        filters={filters}
        onFilterChange={handleFilterChange}
        sort={sort}
        onSortChange={handleSortChange}
        loading={loading}
      />

      {/* Error */}
      {error && (
        <div className="text-red-400 bg-red-900/20 border border-red-800 rounded-lg p-3 text-sm animate-fade-in">
          {error}
        </div>
      )}

      {/* Results grid */}
      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => <SkeletonMemoryCard key={i} />)}
        </div>
      ) : results.length === 0 ? (
        <EmptyState
          icon="📭"
          title="No memories found"
          description={query ? `No results for "${query}". Try different keywords or filters.` : 'Store your first memory to get started.'}
          action={
            <button onClick={() => setStoreOpen(true)} className="btn-primary text-sm">
              ＋ Store Memory
            </button>
          }
        />
      ) : (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {results.map(r => (
              <MemoryCard
                key={r.memory.id}
                memory={r.memory}
                onClick={(mem) => setDetailMemory(mem)}
                onDelete={handleDelete}
                onPromote={handlePromote}
                showScore={!!query.trim()}
                score={r.score}
              />
            ))}
          </div>

          {/* Pagination */}
          <div className="flex items-center justify-between pt-4 border-t border-border">
            <span className="text-xs text-gray-500">
              Showing {offset + 1}–{offset + results.length}
            </span>
            <div className="flex gap-2">
              <button
                onClick={handlePrev}
                disabled={offset === 0}
                className="btn-ghost text-xs py-1.5 disabled:opacity-30"
              >
                ← Prev
              </button>
              <button
                onClick={handleNext}
                disabled={!hasMore}
                className="btn-ghost text-xs py-1.5 disabled:opacity-30"
              >
                Next →
              </button>
            </div>
          </div>
        </>
      )}

      {/* Store Memory Modal */}
      <Modal open={storeOpen} onClose={() => setStoreOpen(false)} title="Store Memory" size="lg">
        <StoreMemoryForm onSuccess={handleMemoryCreated} onCancel={() => setStoreOpen(false)} />
      </Modal>

      {/* Detail Modal */}
      <Modal
        open={!!detailMemory}
        onClose={() => setDetailMemory(null)}
        title={detailMemory?.title || 'Memory Detail'}
        size="lg"
      >
        {detailMemory && (
          <MemoryDetail
            memory={detailMemory}
            onClose={() => setDetailMemory(null)}
            onDelete={handleDelete}
            onRefresh={() => { setDetailMemory(null); fetchMemories() }}
          />
        )}
      </Modal>
    </div>
  )
}
