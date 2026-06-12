export interface AppEnvironmentSummary {
  name: string
  status?: string
  toolCount?: number
  middlewareCount?: number
  componentCount?: number
  errorMessage?: string
  services?: AppServiceSummary[]
}

export interface AppServiceSummary {
  serviceType: string
  status?: string
  errorMessage?: string
}

export interface AppResourceSummary {
  toolCount: number
  middlewareCount: number
  componentCount: number
}

const middlewareServiceTypes = new Set([
  'mysql',
  'postgresql',
  'postgres',
  'mongodb',
  'mongo',
  'redis',
  'rabbitmq',
  'kafka',
  'minio',
])

export function isMiddlewareServiceType(serviceType: string) {
  return middlewareServiceTypes.has(String(serviceType || '').trim().toLowerCase())
}

export function environmentResourceSummary(env: AppEnvironmentSummary): AppResourceSummary {
  const services = Array.isArray(env.services) ? env.services : []
  const componentCount = Number(env.componentCount || 0)
  if (services.length === 0) {
    return {
      toolCount: Number(env.toolCount || 0),
      middlewareCount: Number(env.middlewareCount || 0),
      componentCount,
    }
  }

  const middlewareCount = services.filter((svc) => isMiddlewareServiceType(svc.serviceType)).length
  return {
    toolCount: services.length - middlewareCount,
    middlewareCount,
    componentCount,
  }
}

export function sumApplicationResources(app: { environments?: AppEnvironmentSummary[] } | AppEnvironmentSummary[]): AppResourceSummary {
  const environments = Array.isArray(app) ? app : (app.environments || [])
  return environments.reduce((sum, env) => {
    const item = environmentResourceSummary(env)
    return {
      toolCount: sum.toolCount + item.toolCount,
      middlewareCount: sum.middlewareCount + item.middlewareCount,
      componentCount: sum.componentCount + item.componentCount,
    }
  }, { toolCount: 0, middlewareCount: 0, componentCount: 0 })
}

export function hasErrorEnvironment(environments: AppEnvironmentSummary[]) {
  return environments.some((env) => env.status === 'error' || Boolean(env.errorMessage))
}

export function buildRecentEvents(environments: AppEnvironmentSummary[]) {
  const events: string[] = []
  for (const env of environments) {
    if (env.errorMessage) {
      events.push(`${env.name}：${env.errorMessage}`)
    } else if (env.services?.some((svc) => svc.errorMessage || svc.status === 'failed')) {
      const svc = env.services.find((svc) => svc.errorMessage || svc.status === 'failed')
      events.push(`${env.name} ${svc?.serviceType || '工具'} 异常：${svc?.errorMessage || svc?.status || '未知错误'}`)
    } else if (env.services?.some((svc) => svc.status === 'installing' || svc.status === 'pending')) {
      const svc = env.services.find((svc) => svc.status === 'installing' || svc.status === 'pending')
      events.push(`${env.name} ${svc?.serviceType || '工具'} ${svc?.status === 'pending' ? '等待安装' : '安装中'}`)
    } else if (env.status === 'running') {
      const { toolCount: tools, middlewareCount: middleware, componentCount: comps } = environmentResourceSummary(env)
      const middlewareText = middleware > 0 ? `，${middleware} 个中间件` : ''
      events.push(`${env.name} 已运行，${tools} 个工具${middlewareText}，${comps} 个组件`)
    } else if (env.status === 'creating') {
      events.push(`${env.name} 正在创建中`)
    }
  }
  return events.slice(0, 5)
}

export function countServiceIssues(environments: AppEnvironmentSummary[]) {
  return environments.reduce((sum, env) => {
    const services = env.services || []
    return sum + services.filter((svc) => svc.errorMessage || ['failed', 'installing', 'pending', 'deleting'].includes(String(svc.status || '').toLowerCase())).length
  }, 0)
}
