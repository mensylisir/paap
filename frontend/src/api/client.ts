const API_BASE = '/api/v1'
const inflightGetRequests = new Map<string, Promise<any>>()
const completedGetRequests = new Map<string, { expiresAt: number; value: any }>()
const GET_CACHE_TTL_MS = 1500

function headersToRecord(headers?: HeadersInit): Record<string, string> {
  if (!headers) return {}
  if (headers instanceof Headers) {
    const result: Record<string, string> = {}
    headers.forEach((value, key) => { result[key] = value })
    return result
  }
  if (Array.isArray(headers)) {
    return Object.fromEntries(headers.map(([key, value]) => [key, value]))
  }
  return { ...headers }
}

function storedAuthToken() {
  if (typeof localStorage === 'undefined') return ''
  return localStorage.getItem('paap_token') || ''
}

async function request(path: string, options: RequestInit = {}) {
  const url = `${API_BASE}${path}`
  const method = String(options.method || 'GET').toUpperCase()
  const token = storedAuthToken()
  const cacheKey = token ? `${url}::auth=${token}` : url
  const canDedupe = method === 'GET' && !options.body
  if (canDedupe) {
    const completed = completedGetRequests.get(cacheKey)
    if (completed && completed.expiresAt > Date.now()) return completed.value
    if (completed) completedGetRequests.delete(cacheKey)
    const existing = inflightGetRequests.get(cacheKey)
    if (existing) return existing
  }
  const headers = headersToRecord(options.headers)
  if (!(options.body instanceof FormData) && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json'
  }
  if (token && !headers.Authorization) {
    headers.Authorization = `Bearer ${token}`
  }
  const promise = fetch(url, {
    ...options,
    method,
    headers,
  }).then(async (res) => {
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error(err.error || res.statusText)
    }
    return res.json()
  })
  if (canDedupe) inflightGetRequests.set(cacheKey, promise)
  try {
    const value = await promise
    if (canDedupe) {
      completedGetRequests.set(cacheKey, { expiresAt: Date.now() + GET_CACHE_TTL_MS, value })
    } else {
      completedGetRequests.clear()
    }
    return value
  } finally {
    if (canDedupe && inflightGetRequests.get(cacheKey) === promise) {
      inflightGetRequests.delete(cacheKey)
    }
  }
}

export const api = {
  // Auth
  login: (username: string, password: string) =>
    request('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  me: () => request('/auth/me'),

  // Templates
  templates: () => request('/templates'),
  getTemplate: (id: number | string) => request(`/templates/${id}`),
  createTemplate: (data: any) => request('/templates', { method: 'POST', body: JSON.stringify(data) }),
  updateTemplate: (id: number | string, data: any) => request(`/templates/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTemplate: (id: number | string) => request(`/templates/${id}`, { method: 'DELETE' }),
  listServiceTemplates: () => request('/service-templates'),
  uploadServiceTemplate: (data: FormData) => request('/service-templates/upload', { method: 'POST', body: data }),
  syncBuiltinServiceTemplates: () => request('/service-templates/sync', { method: 'POST' }),
  listComponentConfigTemplates: () => request('/component-config-templates'),
  createComponentConfigTemplate: (data: any) => request('/component-config-templates', { method: 'POST', body: JSON.stringify(data) }),
  updateComponentConfigTemplate: (id: number | string, data: any) => request(`/component-config-templates/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteComponentConfigTemplate: (id: number | string) => request(`/component-config-templates/${id}`, { method: 'DELETE' }),
  syncBuiltinComponentConfigTemplates: () => request('/component-config-templates/sync', { method: 'POST' }),

  // Applications
  listApps: () => request('/applications'),
  createApp: (data: any) => request('/applications', { method: 'POST', body: JSON.stringify(data) }),
  getApp: (id: number) => request(`/applications/${id}`),
  updateApp: (id: number, data: any) => request(`/applications/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteApp: (id: number) => request(`/applications/${id}`, { method: 'DELETE' }),
  listAppMembers: (appId: number) => request(`/applications/${appId}/members`),
  inviteAppMember: (appId: number, data: any) => request(`/applications/${appId}/members`, { method: 'POST', body: JSON.stringify(data) }),
  updateAppMember: (appId: number, memberId: number, data: any) => request(`/applications/${appId}/members/${memberId}`, { method: 'PUT', body: JSON.stringify(data) }),
  removeAppMember: (appId: number, memberId: number) => request(`/applications/${appId}/members/${memberId}`, { method: 'DELETE' }),

  // Environments
  listEnvs: (appId: number) => request(`/applications/${appId}/environments`),
  createEnv: (appId: number, data: any) => request(`/applications/${appId}/environments`, { method: 'POST', body: JSON.stringify(data) }),
  getEnv: (id: number) => request(`/environments/${id}`),
  getEnvironmentCanvasState: (id: number) => request(`/environments/${id}/canvas-state`),
  saveEnvironmentCanvasState: (id: number, data: any) => request(`/environments/${id}/canvas-state`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteEnv: (id: number) => request(`/environments/${id}`, { method: 'DELETE' }),

  // Components
  listComponents: (envId: number) => request(`/environments/${envId}/components`),
  createComponent: (envId: number, data: any) => request(`/environments/${envId}/components`, { method: 'POST', body: JSON.stringify(data) }),
  getComponentRuntimeMetrics: (envId: number, componentId: number) => request(`/environments/${envId}/components/${componentId}/runtime-metrics`),
  getComponentRuntimeLogs: (envId: number, componentId: number, tail = 200) => request(`/environments/${envId}/components/${componentId}/runtime-logs?tail=${tail}`),
  listAdoptableResources: (envId: number) => request(`/environments/${envId}/adoptable-resources`),
  adoptResource: (envId: number, data: any) => request(`/environments/${envId}/adoptable-resources`, { method: 'POST', body: JSON.stringify(data) }),
  updateComponent: (id: number, data: any) => request(`/components/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deployComponent: (id: number, data: any) => request(`/components/${id}/deploy`, { method: 'POST', body: JSON.stringify(data) }),
  deleteComponent: (id: number) => request(`/components/${id}`, { method: 'DELETE' }),

  // Services
  listServices: (envId: number) => request(`/environments/${envId}/services`),
  getServiceWorkspace: (envId: number, serviceId: number) => request(`/environments/${envId}/services/${serviceId}/workspace`),
  getServiceRuntimeMetrics: (envId: number, serviceId: number) => request(`/environments/${envId}/services/${serviceId}/runtime-metrics`),
  getServiceRuntimeLogs: (envId: number, serviceId: number, tail = 200) => request(`/environments/${envId}/services/${serviceId}/runtime-logs?tail=${tail}`),
  getServiceCredentials: (envId: number, serviceId: number) => request(`/environments/${envId}/services/${serviceId}/credentials`),
  runServiceWorkspaceAction: (envId: number, serviceId: number, action: string, target?: string, params?: Record<string, string>) =>
    request(`/environments/${envId}/services/${serviceId}/workspace/actions`, { method: 'POST', body: JSON.stringify({ action, target, params }) }),
  createServiceDraft: (envId: number, data: any) => request(`/environments/${envId}/services/drafts`, { method: 'POST', body: JSON.stringify(data) }),
  installService: (envId: number, data: any) => request(`/environments/${envId}/services`, { method: 'POST', body: JSON.stringify(data) }),
  updateService: (envId: number, serviceId: number, data: any) => request(`/environments/${envId}/services/${serviceId}`, { method: 'PUT', body: JSON.stringify(data) }),
   setServiceExternalAccess: (envId: number, serviceId: number, enabled: boolean) =>
     request(`/environments/${envId}/services/${serviceId}/external-access`, { method: 'PUT', body: JSON.stringify({ enabled }) }),
   setComponentExternalAccess: (envId: number, componentId: number, enabled: boolean) =>
     request(`/environments/${envId}/components/${componentId}/external-access`, { method: 'PUT', body: JSON.stringify({ enabled }) }),
   setComponentNodePortAccess: (envId: number, componentId: number, enabled: boolean) =>
     request(`/environments/${envId}/components/${componentId}/nodeport-access`, { method: 'PUT', body: JSON.stringify({ enabled }) }),
   uninstallService: (envId: number, serviceId: number) => request(`/environments/${envId}/services/${serviceId}`, { method: 'DELETE' }),
}
