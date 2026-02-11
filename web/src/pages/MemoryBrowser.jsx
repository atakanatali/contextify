import { useState, useEffect } from 'react'
import { api } from '../api'
import MemoryCard from '../components/MemoryCard'

const TYPES = ['', 'solution', 'problem', 'code_pattern', 'fix', 'error', 'workflow', 'decision', 'general']
const SCOPES = ['', 'global', 'project']

export default function MemoryBrowser() {
  const [memories, setMemories] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [filters, setFilters] = useState({ query: '', type: '', scope: '' })

  const search = async () => {
    if (!filters.query) return
    setLoading(true)
    setError(null)
    try {
      const searchFilters = {}
      if (filters.type) searchFilters.type = filters.type
      if (filters.scope) searchFilters.scope = filters.scope
      const results = await api.searchMemories(filters.query, searchFilters)
      setMemories(results?.map((r) => r.memory) || [])
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id) => {
    try {
      await api.deleteMemory(id)
      setMemories((prev) => prev.filter((m) => m.id !== id))
    } catch (e) {
      setError(e.message)
    }
  }

  const handlePromote = async (id) => {
    try {
      await api.promoteMemory(id)
      setMemories((prev) =>
        prev.map((m) =>
          m.id === id ? { ...m, ttl_seconds: null, expires_at: null } : m
        )
      )
    } catch (e) {
      setError(e.message)
    }
  }

  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Memories</h2>

      {/* Filters */}
      <div className="flex gap-3 mb-6">
        <input
          type="text"
          placeholder="Search memories..."
          value={filters.query}
          onChange={(e) => setFilters({ ...filters, query: e.target.value })}
          onKeyDown={(e) => e.key === 'Enter' && search()}
          className="flex-1 bg-gray-900 border border-gray-700 rounded px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-indigo-500 focus:outline-none"
        />
        <select
          value={filters.type}
          onChange={(e) => setFilters({ ...filters, type: e.target.value })}
          className="bg-gray-900 border border-gray-700 rounded px-3 py-2 text-sm text-gray-300"
        >
          {TYPES.map((t) => (
            <option key={t} value={t}>{t || 'All types'}</option>
          ))}
        </select>
        <select
          value={filters.scope}
          onChange={(e) => setFilters({ ...filters, scope: e.target.value })}
          className="bg-gray-900 border border-gray-700 rounded px-3 py-2 text-sm text-gray-300"
        >
          {SCOPES.map((s) => (
            <option key={s} value={s}>{s || 'All scopes'}</option>
          ))}
        </select>
        <button
          onClick={search}
          disabled={loading || !filters.query}
          className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white px-4 py-2 rounded text-sm font-medium"
        >
          {loading ? '...' : 'Search'}
        </button>
      </div>

      {error && (
        <div className="text-red-400 bg-red-900/20 border border-red-800 rounded p-3 mb-4 text-sm">
          {error}
        </div>
      )}

      {/* Results */}
      <div className="space-y-3">
        {memories.length === 0 && !loading && (
          <div className="text-gray-600 text-sm text-center py-12">
            Search for memories to get started
          </div>
        )}
        {memories.map((mem) => (
          <MemoryCard
            key={mem.id}
            memory={mem}
            onDelete={handleDelete}
            onPromote={handlePromote}
          />
        ))}
      </div>
    </div>
  )
}
