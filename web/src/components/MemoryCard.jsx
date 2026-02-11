const typeColors = {
  solution: 'bg-green-900 text-green-300',
  problem: 'bg-red-900 text-red-300',
  code_pattern: 'bg-blue-900 text-blue-300',
  fix: 'bg-yellow-900 text-yellow-300',
  error: 'bg-red-900 text-red-300',
  workflow: 'bg-purple-900 text-purple-300',
  decision: 'bg-orange-900 text-orange-300',
  general: 'bg-gray-800 text-gray-300',
}

export default function MemoryCard({ memory, onDelete, onPromote }) {
  const isLongTerm = memory.ttl_seconds === null || memory.ttl_seconds === undefined
  const colorClass = typeColors[memory.type] || typeColors.general

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 hover:border-gray-700 transition-colors">
      <div className="flex items-start justify-between mb-2">
        <h3 className="font-medium text-white text-sm">{memory.title}</h3>
        <div className="flex items-center gap-2 ml-2 shrink-0">
          <span className={`text-xs px-2 py-0.5 rounded ${colorClass}`}>
            {memory.type}
          </span>
          {isLongTerm ? (
            <span className="text-xs px-2 py-0.5 rounded bg-indigo-900 text-indigo-300">
              permanent
            </span>
          ) : (
            <span className="text-xs px-2 py-0.5 rounded bg-gray-800 text-gray-400">
              TTL
            </span>
          )}
        </div>
      </div>

      <p className="text-gray-400 text-xs mb-3 line-clamp-3">{memory.content}</p>

      <div className="flex items-center justify-between text-xs text-gray-500">
        <div className="flex items-center gap-3">
          {memory.tags?.length > 0 && (
            <span>{memory.tags.map((t) => `#${t}`).join(' ')}</span>
          )}
          <span>imp: {memory.importance?.toFixed(1)}</span>
          <span>hits: {memory.access_count}</span>
          {memory.agent_source && <span>{memory.agent_source}</span>}
        </div>
        <div className="flex gap-2">
          {!isLongTerm && onPromote && (
            <button
              onClick={() => onPromote(memory.id)}
              className="text-indigo-400 hover:text-indigo-300"
            >
              promote
            </button>
          )}
          {onDelete && (
            <button
              onClick={() => onDelete(memory.id)}
              className="text-red-400 hover:text-red-300"
            >
              delete
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
