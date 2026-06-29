export interface CatalogTemplateLike {
  category?: unknown
  description?: unknown
  features?: unknown
  name?: unknown
  type?: unknown
}

export interface CatalogGroupMeta {
  category: string
  label: string
  icon: string
  rank: number
}

const catalogGroupMeta = {
  ci: { category: 'ci', label: 'CI服务', icon: 'CI', rank: 10 },
  cd: { category: 'cd', label: 'CD服务', icon: 'CD', rank: 20 },
  monitor: { category: 'monitor', label: '监控服务', icon: 'MON', rank: 30 },
  log: { category: 'log', label: '日志服务', icon: 'LOG', rank: 40 },
  database: { category: 'database', label: '数据库服务', icon: 'DB', rank: 50 },
  middleware: { category: 'middleware', label: '中间件服务', icon: 'MW', rank: 60 },
  environment: { category: 'environment', label: '环境服务', icon: 'ENV', rank: 70 },
  virtualMachine: { category: 'virtualMachine', label: '虚拟机服务', icon: 'VM', rank: 80 },
  other: { category: 'other', label: '其他', icon: 'SVC', rank: 90 },
} satisfies Record<string, CatalogGroupMeta>

const catalogGroupByServiceType: Record<string, CatalogGroupMeta> = {
  ci: catalogGroupMeta.ci,
  jenkins: catalogGroupMeta.ci,
  tekton: catalogGroupMeta.ci,
  deploy: catalogGroupMeta.cd,
  argocd: catalogGroupMeta.cd,
  cd: catalogGroupMeta.cd,
  monitor: catalogGroupMeta.monitor,
  prometheus: catalogGroupMeta.monitor,
  grafana: catalogGroupMeta.monitor,
  log: catalogGroupMeta.log,
  loki: catalogGroupMeta.log,
  registry: catalogGroupMeta.middleware,
  'docker-registry': catalogGroupMeta.middleware,
  harbor: catalogGroupMeta.middleware,
  postgresql: catalogGroupMeta.database,
  'postgresql-ha': catalogGroupMeta.database,
  mysql: catalogGroupMeta.database,
  'mysql-galera': catalogGroupMeta.database,
  mongodb: catalogGroupMeta.database,
  redis: catalogGroupMeta.middleware,
  'redis-cluster': catalogGroupMeta.middleware,
  rabbitmq: catalogGroupMeta.middleware,
  kafka: catalogGroupMeta.middleware,
  minio: catalogGroupMeta.middleware,
  nacos: catalogGroupMeta.middleware,
  eureka: catalogGroupMeta.middleware,
  environment: catalogGroupMeta.environment,
  kubevirt: catalogGroupMeta.virtualMachine,
  vm: catalogGroupMeta.virtualMachine,
}

export const catalogGroupForTemplate = (template: CatalogTemplateLike): CatalogGroupMeta => {
  const type = String(template.type || '').toLowerCase()
  if (catalogGroupByServiceType[type]) return catalogGroupByServiceType[type]

  const category = String(template.category || '').toLowerCase()
  if (category === 'ci') return catalogGroupMeta.ci
  if (category === 'cd' || category === 'deploy') return catalogGroupMeta.cd
  if (category === 'monitor' || category === 'observability') return catalogGroupMeta.monitor
  if (category === 'log' || category === 'logging') return catalogGroupMeta.log
  if (category === 'database') return catalogGroupMeta.database
  if (category === 'environment') return catalogGroupMeta.environment
  if (category === 'vm' || category === 'kubevirt' || category === 'virtualmachine') return catalogGroupMeta.virtualMachine
  if (category === 'infra' || category === 'middleware' || category === 'tool') return catalogGroupMeta.middleware
  return catalogGroupMeta.other
}

export const compareCatalogGroupMeta = (left: CatalogGroupMeta, right: CatalogGroupMeta) =>
  left.rank - right.rank || left.label.localeCompare(right.label)

const searchableText = (value: unknown) => String(value || '').toLowerCase()

const templateHasFeature = (template: CatalogTemplateLike, featureKey: string) => {
  const raw = template.features
  if (Array.isArray(raw)) {
    return raw.some((item: any) => String(item?.key || item || '').toLowerCase() === featureKey)
  }
  return searchableText(raw).includes(featureKey)
}

export const catalogTemplateMatchesQuery = (template: CatalogTemplateLike, query: string) => {
  const q = searchableText(query).trim()
  if (!q) return true

  const group = catalogGroupForTemplate(template)
  const derivedGroups = templateHasFeature(template, 'kubevirt')
    ? [catalogGroupMeta.virtualMachine.label, catalogGroupMeta.virtualMachine.category]
    : []
  return [
    template.name,
    template.type,
    template.description,
    template.category,
    group.label,
    group.category,
    ...derivedGroups,
  ].some(value => searchableText(value).includes(q))
}
