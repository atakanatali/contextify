const API_BASE = '/api/v1'

async function request(path, options = {}) {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || 'Request failed')
  }
  return res.json()
}

export const api = {
  getStats: () => request('/stats'),

  getMemory: (id) => request(`/memories/${id}`),

  storeMemory: (data) =>
    request('/memories', { method: 'POST', body: JSON.stringify(data) }),

  updateMemory: (id, data) =>
    request(`/memories/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  deleteMemory: (id) =>
    request(`/memories/${id}`, { method: 'DELETE' }),

  searchMemories: (query, filters = {}) =>
    request('/memories/search', {
      method: 'POST',
      body: JSON.stringify({ query, ...filters }),
    }),

  recallMemories: (query, filters = {}) =>
    request('/memories/recall', {
      method: 'POST',
      body: JSON.stringify({ query, ...filters }),
    }),

  // List memories with pagination (uses search with a broad query)
  listMemories: ({ query = '*', type, scope, limit = 20, offset = 0, sort } = {}) =>
    request('/memories/search', {
      method: 'POST',
      body: JSON.stringify({
        query: query || '*',
        ...(type && { type }),
        ...(scope && { scope }),
        limit,
        offset,
      }),
    }),

  getRelated: (id) => request(`/memories/${id}/related`),

  createRelationship: (data) =>
    request('/relationships', { method: 'POST', body: JSON.stringify(data) }),

  getContext: (project) =>
    request(`/context/${encodeURIComponent(project)}`, { method: 'POST' }),

  promoteMemory: (id) =>
    request(`/memories/${id}/promote`, { method: 'POST' }),

  // Consolidation
  mergeMemories: (targetId, sourceIds, strategy) =>
    request(`/memories/${targetId}/merge`, {
      method: 'POST',
      body: JSON.stringify({ source_ids: sourceIds, strategy }),
    }),

  getDuplicates: ({ projectId, limit } = {}) =>
    request(`/memories/duplicates?${new URLSearchParams({
      ...(projectId && { project_id: projectId }),
      ...(limit && { limit: String(limit) }),
    })}`),

  batchConsolidate: (operations, strategy) =>
    request('/memories/consolidate', {
      method: 'POST',
      body: JSON.stringify({ operations, strategy }),
    }),

  getSuggestions: ({ projectId, status = 'pending', limit = 20, offset = 0 } = {}) =>
    request(`/consolidation/suggestions?${new URLSearchParams({
      ...(projectId && { project_id: projectId }),
      status,
      limit: String(limit),
      offset: String(offset),
    })}`),

  updateSuggestion: (id, status) =>
    request(`/consolidation/suggestions/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ status }),
    }),

  getConsolidationLog: ({ targetId, limit = 20, offset = 0 } = {}) =>
    request(`/consolidation/log?${new URLSearchParams({
      ...(targetId && { target_id: targetId }),
      limit: String(limit),
      offset: String(offset),
    })}`),
}
