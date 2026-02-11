import { useState, useEffect } from 'react'
import { api } from '../api'
import { TypeBadge, ScopeBadge, TagBadge, ImportanceDot, TtlBadge } from './Badge'
import ConfirmDialog from './ConfirmDialog'
import { useToast } from '../hooks/useToast'

export default function MemoryDetail({ memory: initialMemory, onClose, onRefresh, onDelete }) {
  const toast = useToast()
  const [memory, setMemory] = useState(initialMemory)
  const [related, setRelated] = useState(null)
  const [editing, setEditing] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [promoting, setPromoting] = useState(false)

  // Edit form state
  const [editForm, setEditForm] = useState({
    title: '',
    content: '',
    summary: '',
    type: '',
    tags: '',
    importance: 0.5,
  })

  useEffect(() => {
    setMemory(initialMemory)
  }, [initialMemory])

  // Fetch related memories
  useEffect(() => {
    if (!memory?.id) return
    api.getRelated(memory.id)
      .then(data => setRelated(data?.memories || []))
      .catch(() => setRelated([]))
  }, [memory?.id])

  const startEdit = () => {
    setEditForm({
      title: memory.title || '',
      content: memory.content || '',
      summary: memory.summary || '',
      type: memory.type || 'general',
      tags: (memory.tags || []).join(', '),
      importance: memory.importance ?? 0.5,
    })
    setEditing(true)
  }

  const cancelEdit = () => {
    setEditing(false)
  }

  const saveEdit = async () => {
    try {
      const payload = {}
      if (editForm.title.trim() !== memory.title) payload.title = editForm.title.trim()
      if (editForm.content.trim() !== memory.content) payload.content = editForm.content.trim()
      if ((editForm.summary || '') !== (memory.summary || '')) payload.summary = editForm.summary.trim() || null
      if (editForm.type !== memory.type) payload.type = editForm.type
      const newTags = editForm.tags.split(',').map(t => t.trim()).filter(Boolean)
      if (JSON.stringify(newTags) !== JSON.stringify(memory.tags || [])) payload.tags = newTags
      if (parseFloat(editForm.importance) !== memory.importance) payload.importance = parseFloat(editForm.importance)

      if (Object.keys(payload).length === 0) {
        setEditing(false)
        return
      }

      const updated = await api.updateMemory(memory.id, payload)
      setMemory(updated)
      setEditing(false)
      toast.success('Memory updated')
      onRefresh?.()
    } catch (err) {
      toast.error(`Update failed: ${err.message}`)
    }
  }

  const handlePromote = async () => {
    setPromoting(true)
    try {
      await api.promoteMemory(memory.id)
      setMemory(prev => ({ ...prev, ttl_seconds: null, expires_at: null }))
      toast.success('Promoted to permanent')
      onRefresh?.()
    } catch (err) {
      toast.error(`Promote failed: ${err.message}`)
    } finally {
      setPromoting(false)
    }
  }

  const handleDelete = async () => {
    try {
      await api.deleteMemory(memory.id)
      toast.success('Memory deleted')
      onDelete?.(memory.id)
      onClose?.()
    } catch (err) {
      toast.error(`Delete failed: ${err.message}`)
    }
  }

  if (!memory) return null

  return (
    <div className="space-y-5">
      {/* Action bar */}
      <div className="flex items-center gap-2 justify-end">
        {!editing && (
          <>
            <button onClick={startEdit} className="btn-ghost text-xs py-1.5">
              ✏️ Edit
            </button>
            {memory.ttl_seconds != null && (
              <button onClick={handlePromote} disabled={promoting} className="btn-ghost text-xs py-1.5 text-brand-400">
                {promoting ? '...' : '⬆ Promote'}
              </button>
            )}
            <button onClick={() => setConfirmOpen(true)} className="btn-ghost text-xs py-1.5 text-red-400 hover:text-red-300">
              🗑 Delete
            </button>
          </>
        )}
      </div>

      {/* Meta badges */}
      <div className="flex flex-wrap items-center gap-2">
        <TypeBadge type={memory.type} />
        <ScopeBadge scope={memory.scope} />
        <TtlBadge ttlSeconds={memory.ttl_seconds} />
        <ImportanceDot value={memory.importance} />
        {memory.agent_source && (
          <span className="text-xs text-gray-500">via {memory.agent_source}</span>
        )}
      </div>

      {editing ? (
        /* Edit mode */
        <div className="space-y-4 animate-fade-in">
          <div>
            <label className="text-xs text-gray-400 font-medium mb-1 block">Title</label>
            <input
              className="input"
              value={editForm.title}
              onChange={e => setEditForm(f => ({ ...f, title: e.target.value }))}
            />
          </div>
          <div>
            <label className="text-xs text-gray-400 font-medium mb-1 block">Content</label>
            <textarea
              className="input min-h-[160px] resize-y"
              value={editForm.content}
              onChange={e => setEditForm(f => ({ ...f, content: e.target.value }))}
            />
          </div>
          <div>
            <label className="text-xs text-gray-400 font-medium mb-1 block">Summary</label>
            <input
              className="input"
              value={editForm.summary}
              onChange={e => setEditForm(f => ({ ...f, summary: e.target.value }))}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-xs text-gray-400 font-medium mb-1 block">Type</label>
              <select
                className="input"
                value={editForm.type}
                onChange={e => setEditForm(f => ({ ...f, type: e.target.value }))}
              >
                {['general', 'solution', 'problem', 'code_pattern', 'fix', 'error', 'workflow', 'decision'].map(t => (
                  <option key={t} value={t}>{t.replace('_', ' ')}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-xs text-gray-400 font-medium mb-1 block">
                Importance: {parseFloat(editForm.importance).toFixed(1)}
              </label>
              <input
                type="range" min="0" max="1" step="0.1"
                value={editForm.importance}
                onChange={e => setEditForm(f => ({ ...f, importance: e.target.value }))}
                className="w-full accent-brand-500 h-1.5 mt-2"
              />
            </div>
          </div>
          <div>
            <label className="text-xs text-gray-400 font-medium mb-1 block">Tags (comma-separated)</label>
            <input
              className="input"
              value={editForm.tags}
              onChange={e => setEditForm(f => ({ ...f, tags: e.target.value }))}
            />
          </div>
          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            <button onClick={cancelEdit} className="btn-ghost">Cancel</button>
            <button onClick={saveEdit} className="btn-primary">Save Changes</button>
          </div>
        </div>
      ) : (
        /* View mode */
        <>
          {/* Content */}
          <div>
            <label className="text-xs text-gray-500 uppercase tracking-wider mb-1.5 block">Content</label>
            <div className="bg-surface-2 rounded-lg p-4 text-sm text-gray-300 whitespace-pre-wrap leading-relaxed max-h-80 overflow-y-auto">
              {memory.content}
            </div>
          </div>

          {/* Summary */}
          {memory.summary && (
            <div>
              <label className="text-xs text-gray-500 uppercase tracking-wider mb-1.5 block">Summary</label>
              <p className="text-sm text-gray-400 leading-relaxed">{memory.summary}</p>
            </div>
          )}

          {/* Tags */}
          {memory.tags?.length > 0 && (
            <div>
              <label className="text-xs text-gray-500 uppercase tracking-wider mb-1.5 block">Tags</label>
              <div className="flex flex-wrap gap-1.5">
                {memory.tags.map(tag => <TagBadge key={tag} tag={tag} />)}
              </div>
            </div>
          )}

          {/* Metadata grid */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 py-4 px-1">
            <MetaItem label="Importance" value={memory.importance?.toFixed(1)} highlight={memory.importance >= 0.8} />
            <MetaItem label="Access Count" value={memory.access_count} />
            <MetaItem label="TTL" value={memory.ttl_seconds == null ? 'Permanent' : `${Math.round(memory.ttl_seconds / 3600)}h`} />
            <MetaItem label="Created" value={formatDate(memory.created_at)} />
          </div>

          {/* Project */}
          {memory.project_id && (
            <div>
              <label className="text-xs text-gray-500 uppercase tracking-wider mb-1 block">Project</label>
              <span className="text-sm text-gray-300 font-mono">{memory.project_id}</span>
            </div>
          )}

          {/* Related memories */}
          {related !== null && related.length > 0 && (
            <div>
              <label className="text-xs text-gray-500 uppercase tracking-wider mb-2 block">
                Related Memories ({related.length})
              </label>
              <div className="space-y-1.5">
                {related.map(rel => (
                  <div key={rel.id} className="card-hover p-3 flex items-center justify-between">
                    <div className="flex items-center gap-2 min-w-0">
                      <TypeBadge type={rel.type} />
                      <span className="text-sm text-white truncate">{rel.title}</span>
                    </div>
                    <ImportanceDot value={rel.importance} />
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Footer ID + timestamps */}
          <div className="pt-3 border-t border-border space-y-1">
            <div className="flex items-center justify-between text-xs text-gray-600">
              <span className="font-mono select-all">{memory.id}</span>
              <span>Updated {formatDate(memory.updated_at)}</span>
            </div>
          </div>
        </>
      )}

      <ConfirmDialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={handleDelete}
        title="Delete Memory"
        message={`Are you sure you want to delete "${memory.title}"? This action cannot be undone.`}
      />
    </div>
  )
}

function MetaItem({ label, value, highlight }) {
  return (
    <div>
      <div className="text-xs text-gray-500 mb-0.5">{label}</div>
      <div className={`text-sm font-medium ${highlight ? 'text-emerald-400' : 'text-gray-300'}`}>{value}</div>
    </div>
  )
}

function formatDate(dateStr) {
  if (!dateStr) return '—'
  return new Date(dateStr).toLocaleDateString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
  })
}
