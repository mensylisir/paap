export interface ReadinessEnvironment {
  id: number
  name: string
  identifier: string
  status?: string
  [key: string]: any
}

export interface ReadinessService {
  serviceType: string
  status?: string
  errorMessage?: string
  namespace?: string
  [key: string]: any
}

export interface ReadinessGroup {
  ready: boolean
  missing: string[]
}

export interface EnvironmentReadiness {
  env: ReadinessEnvironment
  services: ReadinessService[]
  serviceTypes: string[]
  componentCount: number
  runningServiceCount: number
  failedServices: ReadinessService[]
  installingServices: ReadinessService[]
  deploy: ReadinessGroup
  ci: ReadinessGroup
  observability: ReadinessGroup
}

const readyStatuses = new Set(['running'])
const inProgressStatuses = new Set(['installing', 'pending', 'deleting', 'creating'])

export function serviceIsReady(service?: ReadinessService) {
  return readyStatuses.has(String(service?.status || '').toLowerCase())
}

function hasReadyService(services: ReadinessService[], type: string) {
  return services.some((svc) => svc.serviceType === type && serviceIsReady(svc))
}

function hasReadyAny(services: ReadinessService[], types: string[]) {
  return types.some((type) => hasReadyService(services, type))
}

function missingFor(services: ReadinessService[], required: Array<string | string[]>) {
  return required
    .filter((item) => Array.isArray(item) ? !hasReadyAny(services, item) : !hasReadyService(services, item))
    .map((item) => Array.isArray(item) ? item[0] : item)
}

export function buildEnvironmentReadiness(
  env: ReadinessEnvironment,
  services: ReadinessService[],
  components: any[],
): EnvironmentReadiness {
  const runningServices = services.filter(serviceIsReady)
  const failedServices = services.filter((svc) => String(svc.status || '').toLowerCase() === 'failed' || !!svc.errorMessage)
  const installingServices = services.filter((svc) => inProgressStatuses.has(String(svc.status || '').toLowerCase()))
  const deployMissing = missingFor(services, ['deploy'])
  const ciMissing = missingFor(services, ['git', 'registry', 'ci'])
  const observabilityMissing = missingFor(services, ['monitor', 'log'])

  return {
    env,
    services,
    serviceTypes: services.map((svc) => svc.serviceType),
    componentCount: components.length,
    runningServiceCount: runningServices.length,
    failedServices,
    installingServices,
    deploy: {
      ready: deployMissing.length === 0,
      missing: deployMissing,
    },
    ci: {
      ready: ciMissing.length === 0,
      missing: ciMissing,
    },
    observability: {
      ready: observabilityMissing.length === 0,
      missing: observabilityMissing,
    },
  }
}

export function toolLabel(type: string) {
  const labels: Record<string, string> = {
    deploy: 'ArgoCD',
    git: 'Gitea',
    registry: '镜像仓库',
    ci: 'CI',
    monitor: '监控',
    log: '日志',
  }
  return labels[type] || type
}
