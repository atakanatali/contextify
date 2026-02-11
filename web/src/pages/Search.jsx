import { useState } from 'react'
import { api } from '../api'
import MemoryCard from '../components/MemoryCard'

export default function Search() {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  const handleSearch = async () => {
    if (!query.trim()) return
    setLoading(true)
    setError(null)
    try {
      const data = await api.recallMemories(query.trim())
      setResults(data || [])
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Semantic Search</h2>

      <div className="flex gap-3 mb-6">
        <input
          type="text"
          placeholder="Ask in natural language... e.g. 'how did we fix the auth bug?'"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          className="flex-1 bg-gray-900 border border-gray-700 rounded px-4 py-3 text-white placeholder-gray-500 focus:border-indigo-500 focus:outline-none"
        />
        <button
          onClick={handleSearch}
          disabled={loading || !query.trim()}
          className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white px-6 py-3 rounded font-medium"
        >
          {loading ? 'Searching...' : 'Recall'}
        </button>
      </div>

      {error && (
        <div className="text-red-400 bg-red-900/20 border border-red-800 rounded p-3 mb-4 text-sm">
          {error}
        </div>
      )}

      <div className="space-y-3">
        {results.length === 0 && !loading && query && (
          <div className="text-gray-600 text-sm text-center py-12">No results found</div>
        )}
        {results.map((r) => (
          <div key={r.memory.id}>
            <div className="text-xs text-gray-600 mb-1">
              Score: {r.score?.toFixed(4)} | Match: {r.match_type}
            </div>
            <MemoryCard memory={r.memory} />
          </div>
        ))}
      </div>
    </div>
  )
}
