const API_BASE = '/api/v1'

async function request(path: string, options: RequestInit = {}) {
  const url = `${API_BASE}${path}`
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

export const api = {
  // Auth
  login: (username: string, password: string) =>
    request('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  me: () => request('/auth/me'),

  // Templates
  templates: () => request('/templates'),

  // Applications
  listApps: () => request('/applications'),
  createApp: (data: any) => request('/applications', { method: 'POST', body: JSON.stringify(data) }),
  getApp: (id: number) => request(`/applications/${id}`),
  updateApp: (id: number, data: any) => request(`/applications/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteApp: (id: number) => request(`/applications/${id}`, { method: 'DELETE' }),

  // Environments
  listEnvs: (appId: number) => request(`/applications/${appId}/environments`),
  createEnv: (appId: number, data: any) => request(`/applications/${appId}/environments`, { method: 'POST', body: JSON.stringify(data) }),
  getEnv: (id: number) => request(`/environments/${id}`),
  deleteEnv: (id: number) => request(`/environments/${id}`, { method: 'DELETE' }),

  // Components
  listComponents: (envId: number) => request(`/environments/${envId}/components`),
  createComponent: (envId: number, data: any) => request(`/environments/${envId}/components`, { method: 'POST', body: JSON.stringify(data) }),
  deleteComponent: (id: number) => request(`/components/${id}`, { method: 'DELETE' }),

  // Services
  listServices: (appId: number) => request(`/applications/${appId}/services`),
  installService: (appId: number, data: any) => request(`/applications/${appId}/services`, { method: 'POST', body: JSON.stringify(data) }),
}
