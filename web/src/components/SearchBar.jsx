import { useState } from 'react'

const TYPES = ['', 'solution', 'problem', 'code_pattern', 'fix', 'error', 'workflow', 'decision', 'general']
const SCOPES = ['', 'global', 'project']
const SORT_OPTIONS = [
  { value: 'relevance', label: 'Relevance' },
  { value: 'newest', label: 'Newest' },
  { value: 'oldest', label: 'Oldest' },
  { value: 'importance', label: 'Importance' },
  { value: 'accessed', label: 'Most Accessed' },
]

export default function SearchBar({
  value = '',
  onChange,
  onSearch,
  placeholder = 'Search memories...',
  showFilters = false,
  showSort = false,
  filters = {},
  onFilterChange,
  sort,
  onSortChange,
  loading = false,
  autoFocus = false,
}) {
  const [focused, setFocused] = useState(false)

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') onSearch?.()
  }

  const handleClear = () => {
    onChange?.('')
    onSearch?.()
  }

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <div className={`relative flex-1 transition-all duration-200 ${focused ? 'ring-1 ring-brand-500/30' : ''} rounded-lg`}>
          {/* Search icon */}
          <svg
            className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500 pointer-events-none"
            fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
          </svg>

          <input
            type="text"
            value={value}
            onChange={(e) => onChange?.(e.target.value)}
            onKeyDown={handleKeyDown}
            onFocus={() => setFocused(true)}
            onBlur={() => setFocused(false)}
            placeholder={placeholder}
            autoFocus={autoFocus}
            className="input pl-10 pr-9 py-2.5"
          />

          {/* Clear */}
          {value && (
            <button
              onClick={handleClear}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300 transition-colors"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
              </svg>
            </button>
          )}
        </div>

        <button
          onClick={() => onSearch?.()}
          disabled={loading || !value.trim()}
          className="btn-primary whitespace-nowrap"
        >
          {loading ? (
            <span className="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
          ) : (
            'Search'
          )}
        </button>
      </div>

      {/* Filters row */}
      {(showFilters || showSort) && (
        <div className="flex flex-wrap gap-2 items-center">
          {showFilters && (
            <>
              <select
                value={filters.type || ''}
                onChange={(e) => onFilterChange?.({ ...filters, type: e.target.value || undefined })}
                className="input w-auto py-1.5 text-xs"
              >
                {TYPES.map((t) => (
                  <option key={t} value={t}>{t ? t.replace('_', ' ') : 'All types'}</option>
                ))}
              </select>

              <select
                value={filters.scope || ''}
                onChange={(e) => onFilterChange?.({ ...filters, scope: e.target.value || undefined })}
                className="input w-auto py-1.5 text-xs"
              >
                {SCOPES.map((s) => (
                  <option key={s} value={s}>{s || 'All scopes'}</option>
                ))}
              </select>
            </>
          )}

          {showSort && (
            <select
              value={sort || 'relevance'}
              onChange={(e) => onSortChange?.(e.target.value)}
              className="input w-auto py-1.5 text-xs ml-auto"
            >
              {SORT_OPTIONS.map(opt => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          )}
        </div>
      )}
    </div>
  )
}
