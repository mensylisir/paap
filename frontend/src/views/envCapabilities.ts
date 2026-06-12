export type CapabilityCategory = 'tool' | 'infra'

export type CapabilityTab = {
  key: string
  label: string
  count: number
  category: CapabilityCategory
  rank: number
}

export const knownCapabilityTabKeys = new Set([
  'code-repository',
  'image-registry',
  'continuous-integration',
  'continuous-deployment',
  'monitoring-center',
  'logging-center',
  'databases',
  'cache',
  'message-queue',
  'middleware',
  'object-storage',
  'platform-tools',
])

export function templateForType(templates: any[], type: string) {
  return templates.find((item: any) => item.type === type)
}

export function serviceStatusText(status?: string) {
  return ({
    running: '运行中',
    installing: '安装中',
    failed: '失败',
    deleting: '删除中',
    pending: '等待中',
    error: '异常',
  } as Record<string, string>)[String(status || '').toLowerCase()] || '未知'
}

export function capabilityServiceInstanceLabel(svc: any, templates: any[] = []) {
  const type = String(svc?.serviceType || svc?.type || '')
  const product = templateForType(templates, type)?.name || type || '服务'
  const instance = String(svc?.serviceName || svc?.name || svc?.releaseName || '').trim()
  const parts = [product]
  if (instance && instance !== product) parts.push(instance)
  parts.push(serviceStatusText(svc?.status))
  return parts.join(' · ')
}

export function serviceCategory(svc: any, templates: any[] = []): CapabilityCategory {
  return templateForType(templates, svc?.serviceType)?.category === 'infra' ? 'infra' : 'tool'
}

export function serviceCapability(svc: any, templates: any[] = []): CapabilityTab {
  const type = String(svc?.serviceType || '')
  const map: Record<string, Omit<CapabilityTab, 'count'>> = {
    git: { key: 'code-repository', label: '代码仓库', category: 'tool', rank: 10 },
    registry: { key: 'image-registry', label: '镜像仓库', category: 'tool', rank: 20 },
    harbor: { key: 'image-registry', label: '镜像仓库', category: 'tool', rank: 20 },
    ci: { key: 'continuous-integration', label: '持续集成', category: 'tool', rank: 30 },
    deploy: { key: 'continuous-deployment', label: '持续部署', category: 'tool', rank: 40 },
    monitor: { key: 'monitoring-center', label: '监控中心', category: 'tool', rank: 50 },
    log: { key: 'logging-center', label: '日志中心', category: 'tool', rank: 60 },
    mysql: { key: 'databases', label: '数据库', category: 'infra', rank: 70 },
    postgresql: { key: 'databases', label: '数据库', category: 'infra', rank: 70 },
    mongodb: { key: 'databases', label: '数据库', category: 'infra', rank: 70 },
    redis: { key: 'cache', label: '缓存', category: 'infra', rank: 80 },
    rabbitmq: { key: 'message-queue', label: '消息队列', category: 'infra', rank: 85 },
    kafka: { key: 'message-queue', label: '消息队列', category: 'infra', rank: 85 },
    minio: { key: 'object-storage', label: '对象存储', category: 'infra', rank: 90 },
  }
  const fallbackCategory = serviceCategory(svc, templates)
  const mapped = map[type] || {
    key: fallbackCategory === 'infra' ? 'middleware' : 'platform-tools',
    label: fallbackCategory === 'infra' ? '中间件' : '平台工具',
    category: fallbackCategory,
    rank: fallbackCategory === 'infra' ? 100 : 95,
  }
  return { ...mapped, count: 0 }
}

export function buildCapabilityTabs(services: any[], templates: any[] = []): CapabilityTab[] {
  const grouped = new Map<string, CapabilityTab>()
  for (const svc of services) {
    const cap = serviceCapability(svc, templates)
    const prev = grouped.get(cap.key)
    grouped.set(cap.key, { ...cap, count: (prev?.count || 0) + 1 })
  }
  return [...grouped.values()].sort((a, b) => a.rank - b.rank || a.label.localeCompare(b.label))
}
