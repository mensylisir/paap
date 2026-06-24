export interface CatalogTemplateLike {
  category?: unknown
  type?: unknown
}

export interface CatalogGroupMeta {
  category: string
  label: string
  icon: string
  rank: number
}

const catalogGroupMeta = {
  tool: { category: 'tool', label: '工具类', icon: '🔧', rank: 10 },
  database: { category: 'database', label: '数据库', icon: '🗄️', rank: 20 },
  cache: { category: 'cache', label: '缓存', icon: '⚡', rank: 30 },
  mq: { category: 'mq', label: '消息队列', icon: '📨', rank: 40 },
  objectStorage: { category: 'objectStorage', label: '对象存储', icon: '🪣', rank: 50 },
  middleware: { category: 'middleware', label: '中间件', icon: '🧩', rank: 60 },
  other: { category: 'other', label: '其他', icon: '📦', rank: 90 },
} satisfies Record<string, CatalogGroupMeta>

const catalogGroupByServiceType: Record<string, CatalogGroupMeta> = {
  postgresql: catalogGroupMeta.database,
  mysql: catalogGroupMeta.database,
  mongodb: catalogGroupMeta.database,
  redis: catalogGroupMeta.cache,
  rabbitmq: catalogGroupMeta.mq,
  kafka: catalogGroupMeta.mq,
  minio: catalogGroupMeta.objectStorage,
}

export const catalogGroupForTemplate = (template: CatalogTemplateLike): CatalogGroupMeta => {
  const type = String(template.type || '').toLowerCase()
  if (catalogGroupByServiceType[type]) return catalogGroupByServiceType[type]

  const category = String(template.category || '').toLowerCase()
  if (category === 'tool') return catalogGroupMeta.tool
  if (category === 'infra' || category === 'middleware') return catalogGroupMeta.middleware
  return catalogGroupMeta.other
}

export const compareCatalogGroupMeta = (left: CatalogGroupMeta, right: CatalogGroupMeta) =>
  left.rank - right.rank || left.label.localeCompare(right.label)
