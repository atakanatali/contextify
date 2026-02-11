import { useState } from 'react'
import { TypeBadge, TtlBadge, TagBadge, ImportanceDot } from './Badge'
import ConfirmDialog from './ConfirmDialog'
import { useToast } from '../hooks/useToast'
import { api } from '../api'

function timeAgo(dateStr) {
  if (!dateStr) return ''
  const seconds = Math.floor((Date.now() - new Date(dateStr)) / 1000)
  if (seconds < 60) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`
  return new Date(dateStr).toLocaleDateString()
}

export default function MemoryCard({ memory, onClick, onDelete, onPromote, showScore, score }) {
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [promoting, setPromoting] = useState(false)
  const toast = useToast()

  const handleDelete = async () => {
    try {
      await api.deleteMemory(memory.id)
      toast.success('Memory deleted')
      onDelete?.(memory.id)
    } catch (err) {
      toast.error(`Delete failed: ${err.message}`)
    }
  }

  const handlePromote = async (e) => {
    e.stopPropagation()
    setPromoting(true)
    try {
      await api.promoteMemory(memory.id)
      toast.success('Promoted to permanent')
      onPromote?.(memory.id)
    } catch (err) {
      toast.error(`Promote failed: ${err.message}`)
    } finally {
      setPromoting(false)
    }
  }

  return (
    <>
      <div className="card-hover p-4 animate-fade-in flex flex-col gap-3" onClick={() => onClick?.(memory)}>
        {/* Header */}
        <div className="flex items-start justify-between gap-2">
          <h3 className="text-sm font-semibold text-white line-clamp-2 flex-1">{memory.title}</h3>
          <TypeBadge type={memory.type} />
        </div>

        {/* Score */}
        {showScore && score != null && (
          <div className="flex items-center gap-2">
            <div className="flex-1 h-1 bg-surface-3 rounded-full overflow-hidden">
              <div className="h-full bg-brand-500 rounded-full transition-all duration-500" style={{ width: `${Math.min(score * 100, 100)}%` }} />
            </div>
            <span className="text-xs text-gray-500 tabular-nums">{score.toFixed(2)}</span>
          </div>
        )}

        {/* Content */}
        <p className="text-xs text-gray-400 line-clamp-3 leading-relaxed">{memory.content}</p>

        {/* Tags */}
        {memory.tags?.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {memory.tags.slice(0, 5).map(tag => <TagBadge key={tag} tag={tag} />)}
            {memory.tags.length > 5 && <span className="text-xs text-gray-600">+{memory.tags.length - 5}</span>}
          </div>
        )}

        {/* Footer */}
        <div className="flex items-center justify-between pt-2 border-t border-border/50">
          <div className="flex items-center gap-3">
            <ImportanceDot value={memory.importance} />
            <TtlBadge ttlSeconds={memory.ttl_seconds} />
            <span className="text-xs text-gray-600">{timeAgo(memory.created_at)}</span>
          </div>
          <div className="flex items-center gap-1" onClick={e => e.stopPropagation()}>
            {memory.ttl_seconds != null && (
              <button onClick={handlePromote} disabled={promoting} className="btn-ghost text-xs py-1 px-2 text-brand-400 hover:text-brand-300" title="Promote">
                {promoting ? '...' : '⬆'}
              </button>
            )}
            <button onClick={(e) => { e.stopPropagation(); setConfirmOpen(true) }} className="btn-ghost text-xs py-1 px-2 text-gray-600 hover:text-red-400" title="Delete">
              🗑
            </button>
          </div>
        </div>
      </div>

      <ConfirmDialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={handleDelete}
        title="Delete Memory"
        message={`Are you sure you want to delete "${memory.title}"? This cannot be undone.`}
      />
    </>
  )
}
