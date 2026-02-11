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

  getRelated: (id) => request(`/memories/${id}/related`),

  createRelationship: (data) =>
    request('/relationships', { method: 'POST', body: JSON.stringify(data) }),

  getContext: (project) =>
    request(`/context/${encodeURIComponent(project)}`, { method: 'POST' }),

  promoteMemory: (id) =>
    request(`/memories/${id}/promote`, { method: 'POST' }),
}
