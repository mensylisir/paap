export type WorkspaceKind = 'repository' | 'gitops' | 'observability' | 'pipeline' | 'registry' | 'data' | 'generic'

export interface WorkspaceContext {
  installation: {
    serviceType: string
    serviceName?: string
    status?: string
    namespace?: string
    releaseName?: string
    values?: string
    [key: string]: any
  }
  app?: {
    identifier?: string
    name?: string
    [key: string]: any
  } | null
  env?: {
    identifier?: string
    name?: string
    [key: string]: any
  } | null
  components?: Array<{
    name?: string
    status?: string
    image?: string
    type?: string
    gitRepoUrl?: string
    gitPath?: string
    argocdApp?: string
    version?: string
    registryImage?: string
    jenkinsJob?: string
    pipelineStatus?: string
    sourceBranch?: string
    [key: string]: any
  }>
  services?: Array<{
    id?: number | string
    environmentId?: number | string
    serviceType?: string
    namespace?: string
    [key: string]: any
  }>
  namespace?: string
}

export interface WorkspaceAction {
  key?: string
  label: string
  description: string
  tone?: 'primary' | 'ghost' | 'danger'
  target?: string
  fields?: WorkspaceActionField[]
}

export interface WorkspaceActionField {
  name: string
  label: string
  type?: 'text' | 'number' | 'textarea' | 'checkbox'
  required?: boolean
  placeholder?: string
  default?: string
}

export interface WorkspaceResource {
  name: string
  type: string
  status: string
  description: string
  externalUrl?: string
  actions?: WorkspaceAction[]
  annotations?: Record<string, any>
  children?: WorkspaceResource[]
}

export interface WorkspaceConfigItem {
  label: string
  value: string
}

export interface ServiceWorkspace {
  kind: WorkspaceKind
  title: string
  description: string
  actions: WorkspaceAction[]
  resources: WorkspaceResource[]
  config: WorkspaceConfigItem[]
}

export function validateWorkspaceActionParams(fields: WorkspaceActionField[], params: Record<string, string>) {
  for (const field of fields) {
    if (field.required && !String(params[field.name] || '').trim()) {
      return `请填写：${field.label}`
    }
  }
  return ''
}

const workspaceKinds: Record<string, WorkspaceKind> = {
  git: 'repository',
  deploy: 'gitops',
  monitor: 'observability',
  log: 'observability',
  ci: 'pipeline',
  registry: 'registry',
  harbor: 'registry',
  mysql: 'data',
  postgresql: 'data',
  mongodb: 'data',
  redis: 'data',
  rabbitmq: 'data',
  kafka: 'data',
  minio: 'data',
}

const accessServiceNames: Record<string, string | ((namespace: string) => string)> = {
  git: (namespace) => namespace,
  deploy: (namespace) => `${namespace}-argocd-server`,
  monitor: (namespace) => `${namespace}-grafana`,
  log: (namespace) => `${namespace}-loki`,
  ci: (namespace) => namespace,
  registry: (namespace) => namespace,
  harbor: 'harbor-portal',
  mysql: 'mysql',
  postgresql: 'postgresql',
  mongodb: 'mongodb',
  redis: 'redis-master',
  rabbitmq: 'rabbitmq',
  kafka: 'kafka',
  minio: 'minio',
}

export function serviceWorkspaceKind(serviceType: string): WorkspaceKind {
  return workspaceKinds[serviceType] || 'generic'
}

export function serviceAccessUrl(namespace: string, serviceType: string) {
  const mapped = accessServiceNames[serviceType] || serviceType
  const serviceName = typeof mapped === 'function' ? mapped(namespace) : mapped
  const port = ({ git: ':3000', ci: ':8080', registry: ':5000', log: ':3100' } as Record<string, string>)[serviceType] || ''
  const scheme = serviceType === 'registry' ? 'https' : 'http'
  return `${scheme}://${serviceName}.${namespace}.svc.cluster.local${port}`
}

export function serviceProxyUrl(environmentId?: number | string, serviceId?: number | string, path = '') {
  if (environmentId === undefined || environmentId === null || serviceId === undefined || serviceId === null) return ''
  const cleanPath = String(path || '').replace(/^\/+/, '')
  return `/api/v1/environments/${environmentId}/services/${serviceId}/proxy/${cleanPath}`
}

export function withEmbeddedProxyAuthToken(url: string, token?: string) {
  if (!url) return ''
  const authToken = token !== undefined
    ? token
    : (typeof localStorage === 'undefined' ? '' : localStorage.getItem('paap_token') || '')
  if (!authToken) return url

  const baseOrigin = typeof window === 'undefined' ? 'http://paap.local' : window.location.origin
  try {
    const parsed = new URL(url, baseOrigin)
    if (parsed.origin !== baseOrigin) return url
    if (!parsed.pathname.startsWith('/api/v1/environments/') || !parsed.pathname.includes('/proxy/')) {
      return parsed.pathname + parsed.search + parsed.hash
    }
    parsed.searchParams.set('paap_token', authToken)
    return parsed.pathname + parsed.search + parsed.hash
  } catch {
    return url
  }
}
