const TYPE_STYLES = {
  solution:     'bg-emerald-900/50 text-emerald-300 border-emerald-700/40',
  problem:      'bg-red-900/50 text-red-300 border-red-700/40',
  code_pattern: 'bg-blue-900/50 text-blue-300 border-blue-700/40',
  fix:          'bg-amber-900/50 text-amber-300 border-amber-700/40',
  error:        'bg-rose-900/50 text-rose-300 border-rose-700/40',
  workflow:     'bg-purple-900/50 text-purple-300 border-purple-700/40',
  decision:     'bg-orange-900/50 text-orange-300 border-orange-700/40',
  general:      'bg-gray-800/50 text-gray-300 border-gray-600/40',
}

const SCOPE_STYLES = {
  global:  'bg-brand-900/50 text-brand-300 border-brand-700/40',
  project: 'bg-violet-900/50 text-violet-300 border-violet-700/40',
}

export function TypeBadge({ type }) {
  if (!type) return null
  return (
    <span className={`text-xs px-2 py-0.5 rounded-md border font-medium ${TYPE_STYLES[type] || TYPE_STYLES.general}`}>
      {type.replace('_', ' ')}
    </span>
  )
}

export function ScopeBadge({ scope }) {
  if (!scope) return null
  return (
    <span className={`text-xs px-2 py-0.5 rounded-md border font-medium ${SCOPE_STYLES[scope] || ''}`}>
      {scope}
    </span>
  )
}

export function TagBadge({ tag }) {
  return (
    <span className="text-xs px-2 py-0.5 rounded-md bg-surface-2 text-gray-400 border border-border">
      #{tag}
    </span>
  )
}

export function ImportanceDot({ value }) {
  if (value == null) return null
  const color = value >= 0.8 ? 'bg-emerald-400' : value >= 0.5 ? 'bg-amber-400' : 'bg-gray-500'
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-gray-400">
      <span className={`w-2 h-2 rounded-full ${color}`} />
      {value.toFixed(1)}
    </span>
  )
}

export function TtlBadge({ ttlSeconds }) {
  const isPermanent = ttlSeconds == null
  return (
    <span className={`text-xs px-2 py-0.5 rounded-md border font-medium ${
      isPermanent
        ? 'bg-brand-900/50 text-brand-300 border-brand-700/40'
        : 'bg-gray-800/50 text-gray-400 border-gray-600/40'
    }`}>
      {isPermanent ? 'permanent' : 'short-term'}
    </span>
  )
}
