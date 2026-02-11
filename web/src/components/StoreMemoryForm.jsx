import { useState } from 'react'
import { api } from '../api'
import { useToast } from '../hooks/useToast'

const TYPES = ['general', 'solution', 'problem', 'code_pattern', 'fix', 'error', 'workflow', 'decision']

export default function StoreMemoryForm({ onSuccess, onCancel }) {
  const toast = useToast()
  const [form, setForm] = useState({
    title: '',
    content: '',
    summary: '',
    type: 'general',
    scope: 'global',
    project_id: '',
    agent_source: '',
    tags: '',
    importance: 0.5,
    ttl_seconds: '',
  })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState(null)

  const update = (key, val) => setForm(f => ({ ...f, [key]: val }))

  const parsedTags = form.tags
    ? form.tags.split(',').map(t => t.trim()).filter(Boolean)
    : []

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!form.title.trim() || !form.content.trim()) {
      setError('Title and content are required')
      return
    }
    setSaving(true)
    setError(null)
    try {
      const payload = {
        title: form.title.trim(),
        content: form.content.trim(),
        type: form.type,
        scope: form.scope,
        tags: parsedTags,
        importance: parseFloat(form.importance),
      }
      if (form.summary.trim()) payload.summary = form.summary.trim()
      if (form.scope === 'project' && form.project_id.trim()) payload.project_id = form.project_id.trim()
      if (form.agent_source.trim()) payload.agent_source = form.agent_source.trim()
      if (form.ttl_seconds && parseInt(form.ttl_seconds) > 0) payload.ttl_seconds = parseInt(form.ttl_seconds)

      await api.storeMemory(payload)
      toast.success('Memory stored successfully')
      onSuccess?.()
    } catch (err) {
      setError(err.message)
      toast.error(`Failed to store: ${err.message}`)
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-5">
      {error && (
        <div className="text-red-400 bg-red-900/20 border border-red-800 rounded-lg p-2.5 text-sm animate-fade-in">
          {error}
        </div>
      )}

      {/* Title */}
      <div>
        <label className="text-xs text-gray-400 font-medium mb-1.5 block">
          Title <span className="text-red-400">*</span>
        </label>
        <input
          className="input"
          value={form.title}
          onChange={e => update('title', e.target.value)}
          placeholder="Short descriptive title"
          autoFocus
        />
      </div>

      {/* Content */}
      <div>
        <label className="text-xs text-gray-400 font-medium mb-1.5 block">
          Content <span className="text-red-400">*</span>
        </label>
        <textarea
          className="input min-h-[140px] resize-y"
          value={form.content}
          onChange={e => update('content', e.target.value)}
          placeholder="Detailed memory content..."
        />
      </div>

      {/* Summary */}
      <div>
        <label className="text-xs text-gray-400 font-medium mb-1.5 block">Summary</label>
        <input
          className="input"
          value={form.summary}
          onChange={e => update('summary', e.target.value)}
          placeholder="Optional brief summary"
        />
      </div>

      {/* Type + Scope row */}
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="text-xs text-gray-400 font-medium mb-1.5 block">Type</label>
          <select className="input" value={form.type} onChange={e => update('type', e.target.value)}>
            {TYPES.map(t => (
              <option key={t} value={t}>{t.replace('_', ' ')}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="text-xs text-gray-400 font-medium mb-1.5 block">Scope</label>
          <select className="input" value={form.scope} onChange={e => update('scope', e.target.value)}>
            <option value="global">Global</option>
            <option value="project">Project</option>
          </select>
        </div>
      </div>

      {/* Project ID (visible when scope=project) */}
      {form.scope === 'project' && (
        <div className="animate-fade-in">
          <label className="text-xs text-gray-400 font-medium mb-1.5 block">Project ID</label>
          <input
            className="input"
            value={form.project_id}
            onChange={e => update('project_id', e.target.value)}
            placeholder="/path/to/project or project-name"
          />
        </div>
      )}

      {/* Agent Source */}
      <div>
        <label className="text-xs text-gray-400 font-medium mb-1.5 block">Agent Source</label>
        <input
          className="input"
          value={form.agent_source}
          onChange={e => update('agent_source', e.target.value)}
          placeholder="e.g. claude-code, cursor, gemini"
        />
      </div>

      {/* Tags */}
      <div>
        <label className="text-xs text-gray-400 font-medium mb-1.5 block">Tags (comma-separated)</label>
        <input
          className="input"
          value={form.tags}
          onChange={e => update('tags', e.target.value)}
          placeholder="go, mcp, fix, auth"
        />
        {parsedTags.length > 0 && (
          <div className="flex flex-wrap gap-1 mt-2">
            {parsedTags.map(tag => (
              <span key={tag} className="text-xs px-2 py-0.5 rounded-md bg-surface-2 text-gray-400 border border-border">
                #{tag}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Importance slider */}
      <div>
        <label className="text-xs text-gray-400 font-medium mb-1.5 block">
          Importance:{' '}
          <span className={`font-semibold ${
            form.importance >= 0.8 ? 'text-emerald-400' : form.importance >= 0.5 ? 'text-amber-400' : 'text-gray-400'
          }`}>
            {parseFloat(form.importance).toFixed(1)}
          </span>
          {form.importance >= 0.8 && (
            <span className="text-emerald-400/70 ml-2 text-xs">→ Auto-permanent</span>
          )}
        </label>
        <input
          type="range" min="0" max="1" step="0.1"
          value={form.importance}
          onChange={e => update('importance', e.target.value)}
          className="w-full accent-brand-500 h-1.5"
        />
        <div className="flex justify-between text-xs text-gray-600 mt-1">
          <span>Low</span>
          <span>Medium</span>
          <span>Critical</span>
        </div>
      </div>

      {/* TTL */}
      <div>
        <label className="text-xs text-gray-400 font-medium mb-1.5 block">
          TTL (seconds)
          <span className="text-gray-600 ml-1.5">Leave empty for default (24h) or auto-permanent if importance ≥ 0.8</span>
        </label>
        <input
          type="number"
          className="input w-40"
          value={form.ttl_seconds}
          onChange={e => update('ttl_seconds', e.target.value)}
          placeholder="86400"
          min="0"
        />
      </div>

      {/* Actions */}
      <div className="flex justify-end gap-2 pt-3 border-t border-border">
        <button type="button" onClick={onCancel} className="btn-ghost">
          Cancel
        </button>
        <button type="submit" disabled={saving} className="btn-primary">
          {saving ? (
            <>
              <span className="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
              Storing...
            </>
          ) : (
            'Store Memory'
          )}
        </button>
      </div>
    </form>
  )
}
