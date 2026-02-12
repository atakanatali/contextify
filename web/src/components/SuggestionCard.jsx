import { TypeBadge, ImportanceDot } from './Badge'

function truncate(str, len = 80) {
  if (!str) return ''
  return str.length > len ? str.slice(0, len) + '...' : str
}

function similarityColor(sim) {
  if (sim >= 0.92) return 'text-red-400 bg-red-900/40 border-red-700/40'
  if (sim >= 0.85) return 'text-amber-400 bg-amber-900/40 border-amber-700/40'
  return 'text-yellow-400 bg-yellow-900/40 border-yellow-700/40'
}

export default function SuggestionCard({ suggestion, selected, onSelect, onDismiss }) {
  const { memory_a, memory_b, similarity } = suggestion
  if (!memory_a || !memory_b) return null

  return (
    <div
      onClick={() => onSelect(suggestion)}
      className={`card p-4 cursor-pointer transition-all ${
        selected ? 'border-brand-500/60 bg-brand-950/20' : 'hover:border-gray-600'
      }`}
    >
      <div className="flex items-center justify-between mb-3">
        <span className={`text-xs px-2 py-0.5 rounded-md border font-mono font-medium ${similarityColor(similarity)}`}>
          {(similarity * 100).toFixed(1)}% match
        </span>
        <button
          onClick={(e) => { e.stopPropagation(); onDismiss(suggestion.id) }}
          className="text-xs text-gray-600 hover:text-red-400 transition-colors px-2 py-1"
          title="Dismiss suggestion"
        >
          Dismiss
        </button>
      </div>

      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-600 font-mono w-4 shrink-0">A</span>
          <h4 className="text-sm text-white truncate flex-1">{memory_a.title}</h4>
          <TypeBadge type={memory_a.type} />
          <ImportanceDot value={memory_a.importance} />
        </div>
        <p className="text-xs text-gray-500 pl-6">{truncate(memory_a.content)}</p>

        <div className="flex items-center gap-2 pt-1">
          <span className="text-xs text-gray-600 font-mono w-4 shrink-0">B</span>
          <h4 className="text-sm text-white truncate flex-1">{memory_b.title}</h4>
          <TypeBadge type={memory_b.type} />
          <ImportanceDot value={memory_b.importance} />
        </div>
        <p className="text-xs text-gray-500 pl-6">{truncate(memory_b.content)}</p>
      </div>
    </div>
  )
}
