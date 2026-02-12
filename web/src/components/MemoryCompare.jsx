import { TypeBadge, ImportanceDot, ScopeBadge, TagBadge, TtlBadge } from './Badge'

function timeAgo(dateStr) {
  if (!dateStr) return ''
  const seconds = Math.floor((Date.now() - new Date(dateStr)) / 1000)
  if (seconds < 60) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`
  return new Date(dateStr).toLocaleDateString()
}

function MemoryPanel({ memory, label }) {
  if (!memory) return null

  return (
    <div className="flex-1 min-w-0">
      <div className="flex items-center gap-2 mb-3">
        <span className="text-xs font-mono text-gray-500 bg-surface-2 rounded px-1.5 py-0.5">{label}</span>
        <span className="text-xs text-gray-600">{timeAgo(memory.created_at)}</span>
      </div>

      <h3 className="text-sm font-semibold text-white mb-2">{memory.title}</h3>

      <div className="flex flex-wrap items-center gap-2 mb-3">
        <TypeBadge type={memory.type} />
        <ScopeBadge scope={memory.scope} />
        <ImportanceDot value={memory.importance} />
        <TtlBadge ttlSeconds={memory.ttl_seconds} />
      </div>

      <div className="bg-surface-0 rounded-lg border border-border p-3 mb-3 max-h-64 overflow-y-auto">
        <pre className="text-xs text-gray-300 whitespace-pre-wrap font-sans leading-relaxed">
          {memory.content}
        </pre>
      </div>

      {memory.tags?.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {memory.tags.map(tag => <TagBadge key={tag} tag={tag} />)}
        </div>
      )}

      <div className="mt-3 text-xs text-gray-600 space-y-0.5">
        {memory.agent_source && <div>Agent: {memory.agent_source}</div>}
        {memory.project_id && <div className="truncate">Project: {memory.project_id}</div>}
        <div>Access count: {memory.access_count}</div>
        {memory.version > 1 && <div>Version: {memory.version}</div>}
      </div>
    </div>
  )
}

export default function MemoryCompare({ memoryA, memoryB }) {
  if (!memoryA || !memoryB) {
    return (
      <div className="flex items-center justify-center h-full text-gray-600 text-sm">
        Select a suggestion to compare memories
      </div>
    )
  }

  return (
    <div className="flex gap-4">
      <MemoryPanel memory={memoryA} label="Memory A" />
      <div className="w-px bg-border shrink-0" />
      <MemoryPanel memory={memoryB} label="Memory B" />
    </div>
  )
}
