import type { WorkspaceResource } from './serviceWorkspace'

export type ServiceConfigScalar = string | number | boolean | null
export type ServiceConfigForm = Record<string, ServiceConfigScalar>
export type ServiceConfigControl = 'select' | 'number' | 'switch' | 'text'

export interface ServiceConfigOption {
  label: string
  value: string | number | boolean
}

export interface ServiceConfigVisibility {
  key: string
  equals?: ServiceConfigScalar | ServiceConfigScalar[]
  notEquals?: ServiceConfigScalar | ServiceConfigScalar[]
  truthy?: boolean
}

export interface ServiceConfigField {
  key: string
  label: string
  control: ServiceConfigControl
  defaultValue: ServiceConfigScalar
  options?: ServiceConfigOption[]
  min?: number
  max?: number
  placeholder?: string
  showWhen?: ServiceConfigVisibility
}

export interface ServiceConfigProfile {
  serviceType: string
  kind: 'redis' | 'database' | 'message-queue' | 'object-storage' | 'tool' | 'unknown'
  showDeploymentConfig: boolean
  showConnectionBindings: boolean
  showTopology: boolean
  showWorkspaceSummary: boolean
  workspaceTitle: string
  fields: ServiceConfigField[]
  summaryKeys: string[]
}

export interface ConfigRow {
  label: string
  value: string
}

export interface RedisTopologyNode {
  name: string
  role: string
  status: string
  address: string
  slots: string
  parent: string
}

export interface RedisTopology {
  summary: {
    masters: number
    replicas: number
    slots: string
  }
  nodes: RedisTopologyNode[]
}

export interface ServiceTopologyNode {
  name: string
  role: string
  status: string
  address: string
  detail: string
}

export interface ServiceTopology {
  summaryRows: ConfigRow[]
  nodes: ServiceTopologyNode[]
}

export interface ConnectionBinding {
  name: string
  value: string
  source: string
}

export interface ConnectionBindingPreview {
  bindings: ConnectionBinding[]
  conflicts: string[]
}

const dataServiceTypes = new Set(['redis', 'mysql', 'postgresql', 'mongodb', 'rabbitmq', 'kafka', 'minio'])

const serviceProfiles: Record<string, Omit<ServiceConfigProfile, 'serviceType'>> = {
  git: {
    kind: 'tool',
    showDeploymentConfig: true,
    showConnectionBindings: false,
    showTopology: true,
    showWorkspaceSummary: true,
    workspaceTitle: 'Gitea 仓库配置',
    summaryKeys: ['replicaCount', 'persistence.enabled'],
    fields: [
      { key: 'replicaCount', label: 'Gitea 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'persistence.enabled', label: '仓库存储', control: 'switch', defaultValue: true },
      { key: 'persistence.size', label: '仓库存储容量', control: 'text', defaultValue: '2Gi', placeholder: '2Gi' },
    ],
  },
  ci: {
    kind: 'tool',
    showDeploymentConfig: true,
    showConnectionBindings: false,
    showTopology: true,
    showWorkspaceSummary: true,
    workspaceTitle: 'Jenkins 配置',
    summaryKeys: ['controller.numExecutors', 'persistence.enabled'],
    fields: [
      { key: 'controller.numExecutors', label: 'Controller 执行器', control: 'number', defaultValue: 0, min: 0 },
      { key: 'controller.javaOpts', label: 'Controller JVM 参数', control: 'text', defaultValue: '-Xms256m -Xmx1024m', placeholder: '-Xms256m -Xmx1024m' },
      { key: 'persistence.enabled', label: 'Jenkins Home 存储', control: 'switch', defaultValue: false },
      { key: 'persistence.size', label: 'Jenkins Home 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi' },
    ],
  },
  deploy: {
    kind: 'tool',
    showDeploymentConfig: true,
    showConnectionBindings: false,
    showTopology: true,
    showWorkspaceSummary: true,
    workspaceTitle: 'ArgoCD 配置',
    summaryKeys: ['controller.replicas', 'server.replicas', 'repoServer.replicas'],
    fields: [
      { key: 'controller.replicas', label: 'Application Controller 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'server.replicas', label: 'API Server 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'repoServer.replicas', label: 'Repo Server 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'applicationSet.replicas', label: 'ApplicationSet 副本', control: 'number', defaultValue: 1, min: 1 },
    ],
  },
  monitor: {
    kind: 'tool',
    showDeploymentConfig: true,
    showConnectionBindings: false,
    showTopology: true,
    showWorkspaceSummary: true,
    workspaceTitle: 'Prometheus + Grafana 配置',
    summaryKeys: ['prometheus.prometheusSpec.replicas', 'prometheus.prometheusSpec.shards', 'grafana.persistence.enabled'],
    fields: [
      { key: 'prometheus.prometheusSpec.replicas', label: 'Prometheus 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'prometheus.prometheusSpec.shards', label: 'Prometheus Shards', control: 'number', defaultValue: 1, min: 1 },
      { key: 'grafana.persistence.enabled', label: 'Grafana 存储', control: 'switch', defaultValue: false },
      { key: 'grafana.persistence.size', label: 'Grafana 存储容量', control: 'text', defaultValue: '10Gi', placeholder: '10Gi' },
    ],
  },
  log: {
    kind: 'tool',
    showDeploymentConfig: true,
    showConnectionBindings: false,
    showTopology: true,
    showWorkspaceSummary: true,
    workspaceTitle: 'Loki 日志配置',
    summaryKeys: ['loki.replicas', 'loki.persistence.enabled'],
    fields: [
      { key: 'loki.replicas', label: 'Loki 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'loki.persistence.enabled', label: 'Loki 存储', control: 'switch', defaultValue: false },
      { key: 'loki.persistence.size', label: 'Loki 存储容量', control: 'text', defaultValue: '10Gi', placeholder: '10Gi' },
    ],
  },
  registry: {
    kind: 'tool',
    showDeploymentConfig: true,
    showConnectionBindings: false,
    showTopology: true,
    showWorkspaceSummary: true,
    workspaceTitle: 'Docker Registry 配置',
    summaryKeys: ['replicaCount', 'persistence.enabled', 'deleteEnabled'],
    fields: [
      { key: 'replicaCount', label: 'Registry 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'persistence.enabled', label: '镜像存储', control: 'switch', defaultValue: true },
      { key: 'persistence.size', label: '镜像存储容量', control: 'text', defaultValue: '5Gi', placeholder: '5Gi' },
      { key: 'deleteEnabled', label: '允许删除镜像', control: 'switch', defaultValue: true },
    ],
  },
  harbor: {
    kind: 'tool',
    showDeploymentConfig: true,
    showConnectionBindings: false,
    showTopology: true,
    showWorkspaceSummary: true,
    workspaceTitle: 'Harbor 配置',
    summaryKeys: ['core.replicas', 'registry.replicas', 'persistence.enabled'],
    fields: [
      { key: 'core.replicas', label: 'Core 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'portal.replicas', label: 'Portal 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'registry.replicas', label: 'Registry 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'jobservice.replicas', label: 'JobService 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'trivy.enabled', label: 'Trivy 镜像扫描', control: 'switch', defaultValue: false },
      { key: 'trivy.replicas', label: 'Trivy 副本', control: 'number', defaultValue: 1, min: 1, showWhen: { key: 'trivy.enabled', equals: true } },
      { key: 'persistence.enabled', label: 'Harbor 存储', control: 'switch', defaultValue: false },
      { key: 'persistence.persistentVolumeClaim.registry.size', label: 'Registry 存储容量', control: 'text', defaultValue: '5Gi', placeholder: '5Gi' },
      { key: 'persistence.persistentVolumeClaim.database.size', label: 'Database 存储容量', control: 'text', defaultValue: '1Gi', placeholder: '1Gi' },
      { key: 'persistence.persistentVolumeClaim.redis.size', label: 'Redis 存储容量', control: 'text', defaultValue: '1Gi', placeholder: '1Gi' },
      { key: 'persistence.persistentVolumeClaim.jobservice.jobLog.size', label: 'Job 日志容量', control: 'text', defaultValue: '1Gi', placeholder: '1Gi' },
      { key: 'persistence.persistentVolumeClaim.trivy.size', label: 'Trivy 缓存容量', control: 'text', defaultValue: '5Gi', placeholder: '5Gi' },
    ],
  },
  redis: {
    kind: 'redis',
    showDeploymentConfig: true,
    showConnectionBindings: true,
    showTopology: true,
    showWorkspaceSummary: false,
    workspaceTitle: '',
    summaryKeys: ['architecture', 'replica.replicaCount', 'master.persistence.enabled'],
    fields: [
      { key: 'architecture', label: 'Redis 架构', control: 'select', defaultValue: 'replication', options: [{ label: 'Redis 单节点', value: 'standalone' }, { label: 'Redis 主从复制', value: 'replication' }, { label: 'Redis Sentinel', value: 'sentinel' }, { label: 'Redis Cluster', value: 'cluster' }] },
      { key: 'replica.replicaCount', label: 'Redis 副本', control: 'number', defaultValue: 3, min: 1, showWhen: { key: 'architecture', equals: ['replication', 'sentinel'] } },
      { key: 'sentinel.masterSet', label: '哨兵监控名称', control: 'text', defaultValue: 'mymaster', placeholder: 'mymaster', showWhen: { key: 'architecture', equals: 'sentinel' } },
      { key: 'cluster.nodes', label: '集群节点数', control: 'number', defaultValue: 6, min: 3, showWhen: { key: 'architecture', equals: 'cluster' } },
      { key: 'cluster.replicas', label: '每主节点副本', control: 'number', defaultValue: 1, min: 0, showWhen: { key: 'architecture', equals: 'cluster' } },
      { key: 'persistence.enabled', label: '集群节点存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', equals: 'cluster' } },
      { key: 'persistence.size', label: '集群节点容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'architecture', equals: 'cluster' } },
      { key: 'master.persistence.enabled', label: '主节点存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', notEquals: 'cluster' } },
      { key: 'master.persistence.size', label: '主节点容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'architecture', notEquals: 'cluster' } },
      { key: 'replica.persistence.enabled', label: '副本节点存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', equals: ['replication', 'sentinel'] } },
      { key: 'replica.persistence.size', label: '副本节点容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'architecture', equals: ['replication', 'sentinel'] } },
    ],
  },
  mysql: {
    kind: 'database',
    showDeploymentConfig: true,
    showConnectionBindings: true,
    showTopology: true,
    showWorkspaceSummary: false,
    workspaceTitle: '',
    summaryKeys: ['architecture', 'secondary.replicaCount', 'replicaCount', 'primary.persistence.enabled', 'persistence.enabled'],
    fields: [
      { key: 'architecture', label: 'MySQL 架构', control: 'select', defaultValue: 'standalone', options: [{ label: '单主库', value: 'standalone' }, { label: '主从复制', value: 'replication' }, { label: 'MariaDB Galera 双主 (MySQL 协议)', value: 'dual-master' }, { label: 'MariaDB Galera 多主集群 (MySQL 协议)', value: 'galera' }] },
      { key: 'secondary.replicaCount', label: '从库数量', control: 'number', defaultValue: 1, min: 1, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'replicaCount', label: 'Galera 节点数', control: 'number', defaultValue: 3, min: 2, showWhen: { key: 'architecture', equals: ['dual-master', 'galera'] } },
      { key: 'primary.persistence.enabled', label: '主库数据存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', notEquals: ['dual-master', 'galera'] } },
      { key: 'primary.persistence.size', label: '主库容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'architecture', notEquals: ['dual-master', 'galera'] } },
      { key: 'secondary.persistence.enabled', label: '从库数据存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'secondary.persistence.size', label: '从库容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'persistence.enabled', label: 'Galera 节点存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', equals: ['dual-master', 'galera'] } },
      { key: 'persistence.size', label: 'Galera 单节点容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'architecture', equals: ['dual-master', 'galera'] } },
    ],
  },
  postgresql: {
    kind: 'database',
    showDeploymentConfig: true,
    showConnectionBindings: true,
    showTopology: true,
    showWorkspaceSummary: false,
    workspaceTitle: '',
    summaryKeys: ['architecture', 'readReplicas.replicaCount', 'postgresql.replicaCount', 'pgpool.replicaCount', 'primary.persistence.enabled', 'persistence.enabled'],
    fields: [
      { key: 'architecture', label: 'PostgreSQL 架构', control: 'select', defaultValue: 'standalone', options: [{ label: '单主库', value: 'standalone' }, { label: '主库 + 只读副本', value: 'replication' }, { label: 'PostgreSQL HA 集群 (repmgr + Pgpool)', value: 'ha-cluster' }] },
      { key: 'readReplicas.replicaCount', label: '只读副本数', control: 'number', defaultValue: 1, min: 1, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'postgresql.replicaCount', label: 'PostgreSQL 节点数', control: 'number', defaultValue: 3, min: 3, showWhen: { key: 'architecture', equals: 'ha-cluster' } },
      { key: 'pgpool.replicaCount', label: 'Pgpool 节点数', control: 'number', defaultValue: 1, min: 1, showWhen: { key: 'architecture', equals: 'ha-cluster' } },
      { key: 'primary.persistence.enabled', label: '主库数据存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', notEquals: 'ha-cluster' } },
      { key: 'primary.persistence.size', label: '主库容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'architecture', notEquals: 'ha-cluster' } },
      { key: 'readReplicas.persistence.enabled', label: '只读副本存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'readReplicas.persistence.size', label: '只读副本容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'persistence.enabled', label: 'HA 数据存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', equals: 'ha-cluster' } },
      { key: 'persistence.size', label: 'HA 单节点容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'architecture', equals: 'ha-cluster' } },
    ],
  },
  mongodb: {
    kind: 'database',
    showDeploymentConfig: true,
    showConnectionBindings: true,
    showTopology: true,
    showWorkspaceSummary: false,
    workspaceTitle: '',
    summaryKeys: ['architecture', 'replicaCount', 'persistence.enabled'],
    fields: [
      { key: 'architecture', label: 'MongoDB 架构', control: 'select', defaultValue: 'standalone', options: [{ label: '单实例', value: 'standalone' }, { label: '副本集', value: 'replicaset' }] },
      { key: 'replicaCount', label: '副本集节点', control: 'number', defaultValue: 2, min: 1, showWhen: { key: 'architecture', equals: 'replicaset' } },
      { key: 'persistence.enabled', label: '数据存储', control: 'switch', defaultValue: false },
      { key: 'persistence.size', label: '存储容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi' },
    ],
  },
  rabbitmq: {
    kind: 'message-queue',
    showDeploymentConfig: true,
    showConnectionBindings: true,
    showTopology: true,
    showWorkspaceSummary: false,
    workspaceTitle: '',
    summaryKeys: ['replicaCount', 'persistence.enabled'],
    fields: [
      { key: 'replicaCount', label: 'RabbitMQ 副本', control: 'number', defaultValue: 1, min: 1 },
      { key: 'persistence.enabled', label: '队列存储', control: 'switch', defaultValue: false },
      { key: 'persistence.size', label: '存储容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi' },
    ],
  },
  kafka: {
    kind: 'message-queue',
    showDeploymentConfig: true,
    showConnectionBindings: true,
    showTopology: true,
    showWorkspaceSummary: false,
    workspaceTitle: '',
    summaryKeys: ['controller.replicaCount', 'broker.replicaCount', 'controller.persistence.enabled'],
    fields: [
      { key: 'controller.replicaCount', label: 'Controller 节点', control: 'number', defaultValue: 3, min: 1 },
      { key: 'broker.replicaCount', label: 'Broker 节点', control: 'number', defaultValue: 0, min: 0 },
      { key: 'controller.persistence.enabled', label: 'Controller 存储', control: 'switch', defaultValue: false },
      { key: 'controller.persistence.size', label: 'Controller 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi' },
      { key: 'broker.persistence.enabled', label: 'Broker 存储', control: 'switch', defaultValue: false },
      { key: 'broker.persistence.size', label: 'Broker 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi' },
    ],
  },
  minio: {
    kind: 'object-storage',
    showDeploymentConfig: true,
    showConnectionBindings: true,
    showTopology: true,
    showWorkspaceSummary: false,
    workspaceTitle: '',
    summaryKeys: ['mode', 'statefulset.replicaCount', 'persistence.enabled'],
    fields: [
      { key: 'mode', label: 'MinIO 模式', control: 'select', defaultValue: 'standalone', options: [{ label: '单实例', value: 'standalone' }, { label: '分布式', value: 'distributed' }] },
      { key: 'statefulset.replicaCount', label: '分布式节点', control: 'number', defaultValue: 4, min: 4, showWhen: { key: 'mode', equals: 'distributed' } },
      { key: 'persistence.enabled', label: '对象存储数据盘', control: 'switch', defaultValue: false },
      { key: 'persistence.size', label: '存储容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi' },
    ],
  },
}

const toolProfile: Omit<ServiceConfigProfile, 'serviceType'> = {
  kind: 'tool',
  showDeploymentConfig: false,
  showConnectionBindings: false,
  showTopology: false,
  showWorkspaceSummary: true,
  workspaceTitle: '工作台入口',
  fields: [],
  summaryKeys: [],
}

const unknownProfile: Omit<ServiceConfigProfile, 'serviceType'> = {
  kind: 'unknown',
  showDeploymentConfig: false,
  showConnectionBindings: false,
  showTopology: false,
  showWorkspaceSummary: true,
  workspaceTitle: '运行入口',
  fields: [],
  summaryKeys: [],
}

const serviceProfileAliases: Record<string, string> = {
  gitea: 'git',
  jenkins: 'ci',
  argocd: 'deploy',
  loki: 'log',
  'prometheus-grafana': 'monitor',
  'docker-registry': 'registry',
}

export function serviceConfigProfile(service: any): ServiceConfigProfile {
  const serviceType = serviceConfigType(service)
  const profileKey = serviceProfileAliases[serviceType] || serviceType
  const base = serviceProfiles[profileKey] || (serviceType ? toolProfile : unknownProfile)
  return {
    serviceType,
    kind: base.kind,
    showDeploymentConfig: base.showDeploymentConfig,
    showConnectionBindings: base.showConnectionBindings,
    showTopology: base.showTopology,
    showWorkspaceSummary: base.showWorkspaceSummary,
    workspaceTitle: base.workspaceTitle,
    fields: base.fields,
    summaryKeys: base.summaryKeys,
  }
}

export function serviceConfigFields(service: any): ServiceConfigField[] {
  return serviceConfigProfile(service).fields
}

export function serviceConfigFieldVisible(field: ServiceConfigField, form: ServiceConfigForm): boolean {
  const condition = field.showWhen
  if (!condition) return true
  const actual = form[condition.key]
  if (condition.truthy !== undefined) return Boolean(actual) === condition.truthy
  if (condition.equals !== undefined) return valueIn(actual, condition.equals)
  if (condition.notEquals !== undefined) return !valueIn(actual, condition.notEquals)
  return true
}

export function serviceConfigValues(service: any): Record<string, string> {
  const raw = service?.values
  if (!raw) return {}
  if (typeof raw === 'object' && !Array.isArray(raw)) return flattenRecord(raw)
  if (typeof raw !== 'string') return {}
  try {
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) return flattenRecord(parsed)
  } catch {
    return {}
  }
  return {}
}

export function serviceConfigFormFromInstallation(service: any): ServiceConfigForm {
  const type = serviceConfigType(service)
  const values = serviceConfigValues(service)
  const form: ServiceConfigForm = {}
  for (const field of serviceConfigFields(service)) {
    form[field.key] = formValueFromValues(field, values)
  }
  if (type === 'redis' && values['sentinel.enabled'] === 'true') {
    form.architecture = 'sentinel'
  }
  if (type === 'redis' && values.architecture === 'cluster') {
    form.architecture = 'cluster'
  }
  if ((type === 'mysql' || type === 'postgresql') && values['paap.architecture']) {
    form.architecture = values['paap.architecture']
  }
  return normalizeServiceForm(type, form)
}

export function serviceConfigValuesFromForm(serviceType: string, form: ServiceConfigForm): Record<string, string> {
  const type = serviceConfigType({ serviceType })
  const nextForm = normalizeServiceForm(type, { ...form })
  const values: Record<string, string> = {}
  for (const field of serviceConfigFields({ serviceType: type })) {
    if (!serviceConfigFieldVisible(field, nextForm)) continue
    const raw = nextForm[field.key]
    if (raw === undefined || raw === null || raw === '') continue
    values[field.key] = String(raw)
  }
  return normalizeServiceValues(type, values, nextForm)
}

export function serviceConfigPrimaryRows(service: any, form = serviceConfigFormFromInstallation(service)): ConfigRow[] {
  const profile = serviceConfigProfile(service)
  const rows: ConfigRow[] = []
  if (profile.serviceType === 'redis' && form.architecture === 'cluster') {
    for (const key of ['architecture', 'cluster.nodes', 'cluster.replicas', 'persistence.enabled']) {
      const field = profile.fields.find((item) => item.key === key)
      if (!field) continue
      rows.push({ label: field.label, value: serviceConfigDisplayValue(field, form) })
    }
    return rows
  }
  for (const key of profile.summaryKeys) {
    if (key === '$endpoint') {
      rows.push({ label: '连接地址', value: serviceInternalEndpoint(service) || '等待生成' })
      continue
    }
    const field = profile.fields.find((item) => item.key === key)
    if (!field) continue
    if (!serviceConfigFieldVisible(field, form)) continue
    rows.push({ label: field.label, value: serviceConfigDisplayValue(field, form) })
  }
  return rows
}

export function serviceRuntimeDetailRows(service: any): ConfigRow[] {
  const runtime = service?.runtimeConfig || {}
  return [
    { label: '命名空间', value: stringValue(service?.namespace || runtime.namespace, '-') },
    { label: '工作负载类型', value: stringValue(runtime.workloadKind, '-') },
    { label: '工作负载名称', value: stringValue(runtime.workloadName, '-') },
    { label: '容器', value: stringValue(runtime.container, '-') },
  ]
}

export function redisTopologyFromWorkspace(resources: WorkspaceResource[] = []): RedisTopology {
  const nodes = resources
    .filter((resource) => {
      const type = String(resource.type || '').toLowerCase()
      return type === 'redis node' || type === 'redis-node' || type === 'redis cluster node'
    })
    .map((resource) => {
      const annotations = resource.annotations || {}
      const roleText = stringValue(annotations.role || annotations.flags || resource.description, '').toLowerCase()
      const role = roleText.includes('replica') || roleText.includes('slave')
        ? 'replica'
        : roleText.includes('master')
          ? 'master'
          : stringValue(annotations.role, 'unknown')
      return {
        name: resource.name,
        role,
        status: resource.status || 'Unknown',
        address: stringValue(annotations.address || annotations.addr, ''),
        slots: stringValue(annotations.slots, ''),
        parent: stringValue(annotations.master || annotations.parent, ''),
      }
    })

  const slotRanges = nodes.map((node) => node.slots).filter(Boolean)
  return {
    summary: {
      masters: nodes.filter((node) => node.role === 'master').length,
      replicas: nodes.filter((node) => node.role === 'replica').length,
      slots: slotRanges.length ? slotRanges.join(',') : '未采集',
    },
    nodes,
  }
}

export function serviceTopologyFromWorkspace(service: any, resources: WorkspaceResource[] = []): ServiceTopology {
  if (normalizedServiceType(service) === 'redis') {
    const topology = redisTopologyFromWorkspace(resources)
    if (!topology.nodes.length) {
      const runtimeTopology = serviceRuntimeTopology(service)
      if (runtimeTopology) return runtimeTopology
    }
    return {
      summaryRows: [
        { label: '主节点', value: String(topology.summary.masters) },
        { label: '副本', value: String(topology.summary.replicas) },
        { label: 'Slots', value: topology.summary.slots },
      ],
      nodes: topology.nodes.map((node) => ({
        name: node.name,
        role: node.role,
        status: node.status,
        address: node.address,
        detail: node.parent ? `上游 ${node.parent}` : (node.slots ? `Slots ${node.slots}` : ''),
      })),
    }
  }

  const runtimeTopology = serviceRuntimeTopology(service)
  if (runtimeTopology) return runtimeTopology

  const nodes = resources
    .filter((resource) => {
      const type = String(resource.type || '').toLowerCase()
      return ['pod', 'statefulset', 'deployment', 'daemonset', 'service', 'storage'].includes(type)
    })
    .map((resource) => ({
      name: resource.name,
      role: stringValue(resource.type, '实例'),
      status: stringValue(resource.status, 'Unknown'),
      address: stringValue(resource.annotations?.address || resource.annotations?.addr || resource.annotations?.endpoint, ''),
      detail: stringValue(resource.description, ''),
    }))

  return {
    summaryRows: [{ label: '实例', value: String(nodes.length) }],
    nodes,
  }
}

function serviceRuntimeTopology(service: any): ServiceTopology | null {
  const runtime = service?.runtimeConfig || {}
  const workloadName = stringValue(runtime.workloadName || service?.releaseName || service?.serviceName, '')
  if (!workloadName) return null
  const workloadKind = stringValue(runtime.workloadKind, 'Workload')
  const namespace = stringValue(runtime.namespace || service?.namespace, '')
  const container = stringValue(runtime.container, '')
  const image = stringValue(runtime.image, '')
  const replicas = runtime.replicas === undefined || runtime.replicas === null || runtime.replicas === ''
    ? '1'
    : String(runtime.replicas)
  const detail = [
    container ? `Container ${container}` : '',
    image ? `Image ${image}` : '',
  ].filter(Boolean).join(' · ')

  return {
    summaryRows: [
      { label: '实例', value: replicas },
      { label: '类型', value: workloadKind },
    ],
    nodes: [{
      name: workloadName,
      role: workloadKind,
      status: stringValue(service?.status, 'Unknown'),
      address: namespace ? `${namespace}/${workloadName}` : workloadName,
      detail,
    }],
  }
}

export function connectionBindingPreview(component: any, service: any): ConnectionBindingPreview {
  const serviceType = normalizedServiceType(service)
  const serviceName = runtimeServiceName(service, serviceType)
  const namespace = String(service?.namespace || '')
  const port = defaultServicePort(serviceType)
  const prefix = connectionEnvPrefix(serviceType)
  const host = namespace ? `${serviceName}.${namespace}.svc.cluster.local` : serviceName
  const bindings = serviceType === 'eureka'
    ? [{ name: 'EUREKA_URL', value: `http://${host}:${port}/eureka/`, source: serviceName }]
    : [
        { name: `${prefix}_HOST`, value: host, source: serviceName },
        { name: `${prefix}_PORT`, value: String(port), source: serviceName },
      ]
  if (serviceType === 'redis' && redisClusterEnabled(service)) {
    bindings.push(
      { name: 'REDIS_CLUSTER_HOST', value: host, source: serviceName },
      { name: 'REDIS_CLUSTER_PORT', value: '6379', source: serviceName },
      { name: 'REDIS_CLUSTER_NODES', value: redisClusterStartupNodes(service, host), source: serviceName },
    )
  }
  if (serviceType === 'redis' && redisSentinelEnabled(service)) {
    bindings.push(
      { name: 'REDIS_SENTINEL_HOST', value: host, source: serviceName },
      { name: 'REDIS_SENTINEL_PORT', value: '26379', source: serviceName },
      { name: 'REDIS_SENTINEL_MASTER_NAME', value: serviceConfigValues(service)['sentinel.masterSet'] || 'mymaster', source: serviceName },
    )
  }
  const existing = new Set((component?.env || component?.runtimeConfig?.env || []).map((item: any) => String(item?.name || '').trim()).filter(Boolean))
  return {
    bindings,
    conflicts: bindings.filter((binding) => existing.has(binding.name)).map((binding) => binding.name),
  }
}

export function isConfigurableDataService(service: any): boolean {
  return dataServiceTypes.has(normalizedServiceType(service))
}

function normalizeServiceForm(serviceType: string, form: ServiceConfigForm): ServiceConfigForm {
  if (serviceType === 'redis') {
    if (form.architecture === 'cluster') {
      form['sentinel.enabled'] = false
      form['master.persistence.enabled'] = false
      form['replica.persistence.enabled'] = false
      const replicas = Math.max(0, Number(form['cluster.replicas'] ?? 1) || 0)
      const minNodes = Math.max(3, 3 * (replicas + 1))
      const nodes = Math.max(minNodes, Number(form['cluster.nodes'] ?? minNodes) || minNodes)
      form['cluster.replicas'] = replicas
      form['cluster.nodes'] = nodes
    }
    if (form.architecture !== 'replication' && form.architecture !== 'sentinel') form['sentinel.enabled'] = false
    if (form.architecture === 'sentinel') form['sentinel.enabled'] = true
  }
  if (serviceType === 'mysql') {
    if (!['standalone', 'replication', 'dual-master', 'galera'].includes(String(form.architecture))) {
      form.architecture = 'standalone'
    }
    if (form.architecture === 'dual-master' || form.architecture === 'galera') {
      const minNodes = form.architecture === 'dual-master' ? 2 : 3
      form.replicaCount = Math.max(minNodes, Number(form.replicaCount ?? minNodes) || minNodes)
      form['primary.persistence.enabled'] = false
      form['secondary.persistence.enabled'] = false
    } else {
      form['persistence.enabled'] = false
    }
    if (form.architecture !== 'replication') {
      form['secondary.persistence.enabled'] = false
    }
  }
  if (serviceType === 'postgresql') {
    if (!['standalone', 'replication', 'ha-cluster'].includes(String(form.architecture))) {
      form.architecture = 'standalone'
    }
    if (form.architecture === 'ha-cluster') {
      form['postgresql.replicaCount'] = Math.max(3, Number(form['postgresql.replicaCount'] ?? 3) || 3)
      form['pgpool.replicaCount'] = Math.max(1, Number(form['pgpool.replicaCount'] ?? 1) || 1)
      form['primary.persistence.enabled'] = false
      form['readReplicas.persistence.enabled'] = false
    } else {
      form['persistence.enabled'] = false
    }
    if (form.architecture !== 'replication') {
      form['readReplicas.persistence.enabled'] = false
    }
  }
  if (serviceType === 'minio' && form.mode !== 'distributed') {
    form['statefulset.replicaCount'] = form['statefulset.replicaCount'] || 4
  }
  return form
}

function normalizeServiceValues(serviceType: string, values: Record<string, string>, form: ServiceConfigForm): Record<string, string> {
  if (serviceType === 'redis') {
    if (form.architecture === 'cluster') {
      const replicas = Math.max(0, Number(form['cluster.replicas'] ?? values['cluster.replicas'] ?? 1) || 0)
      const minNodes = Math.max(3, 3 * (replicas + 1))
      const nodes = Math.max(minNodes, Number(form['cluster.nodes'] ?? values['cluster.nodes'] ?? minNodes) || minNodes)
      values.architecture = 'cluster'
      values['cluster.init'] = 'true'
      values['cluster.nodes'] = String(nodes)
      values['cluster.replicas'] = String(replicas)
      values.usePassword = 'true'
      values['sentinel.enabled'] = 'false'
    } else if (form.architecture === 'sentinel') {
      values.architecture = 'replication'
      values['sentinel.enabled'] = 'true'
      values['sentinel.masterSet'] = values['sentinel.masterSet'] || 'mymaster'
    } else if (form.architecture !== 'replication') {
      values['sentinel.enabled'] = 'false'
    } else {
      values.architecture = 'replication'
      values['sentinel.enabled'] = 'false'
    }
  }
  if (serviceType === 'mysql') {
    if (form.architecture === 'dual-master' || form.architecture === 'galera') {
      const minNodes = form.architecture === 'dual-master' ? 2 : 3
      values['paap.architecture'] = String(form.architecture)
      values.replicaCount = String(Math.max(minNodes, Number(form.replicaCount ?? values.replicaCount ?? minNodes) || minNodes))
      delete values.architecture
      delete values['primary.persistence.enabled']
      delete values['primary.persistence.size']
      delete values['secondary.replicaCount']
      delete values['secondary.persistence.enabled']
      delete values['secondary.persistence.size']
    } else {
      values.architecture = form.architecture === 'replication' ? 'replication' : 'standalone'
      delete values['paap.architecture']
      delete values.replicaCount
      delete values['persistence.enabled']
      delete values['persistence.size']
      if (values.architecture !== 'replication') {
        delete values['secondary.replicaCount']
        delete values['secondary.persistence.enabled']
        delete values['secondary.persistence.size']
      }
    }
  }
  if (serviceType === 'postgresql') {
    if (form.architecture === 'ha-cluster') {
      values['paap.architecture'] = 'ha-cluster'
      values['postgresql.replicaCount'] = String(Math.max(3, Number(form['postgresql.replicaCount'] ?? values['postgresql.replicaCount'] ?? 3) || 3))
      values['pgpool.replicaCount'] = String(Math.max(1, Number(form['pgpool.replicaCount'] ?? values['pgpool.replicaCount'] ?? 1) || 1))
      delete values.architecture
      delete values['primary.persistence.enabled']
      delete values['primary.persistence.size']
      delete values['readReplicas.replicaCount']
      delete values['readReplicas.persistence.enabled']
      delete values['readReplicas.persistence.size']
    } else {
      values.architecture = form.architecture === 'replication' ? 'replication' : 'standalone'
      delete values['paap.architecture']
      delete values['postgresql.replicaCount']
      delete values['pgpool.replicaCount']
      delete values['persistence.enabled']
      delete values['persistence.size']
      if (values.architecture !== 'replication') {
        delete values['readReplicas.replicaCount']
        delete values['readReplicas.persistence.enabled']
        delete values['readReplicas.persistence.size']
      }
    }
  }
  return values
}

function formValueFromValues(field: ServiceConfigField, values: Record<string, string>): ServiceConfigScalar {
  if (!Object.prototype.hasOwnProperty.call(values, field.key)) return field.defaultValue
  const value = values[field.key]
  if (field.control === 'switch') return booleanValue(value, Boolean(field.defaultValue))
  if (field.control === 'number') return optionalNumber(value) ?? Number(field.defaultValue || 0)
  return stringValue(value, String(field.defaultValue ?? ''))
}

function serviceConfigDisplayValue(field: ServiceConfigField, form: ServiceConfigForm): string {
  const raw = form[field.key]
  if (field.control === 'switch') {
    if (field.key.endsWith('.persistence.enabled') || field.key === 'persistence.enabled') {
      const sizeKey = field.key.replace(/enabled$/, 'size')
      return raw ? stringValue(form[sizeKey], '已启用') : '临时存储'
    }
    return raw ? '启用' : '未启用'
  }
  if (field.options?.length) {
    return field.options.find((option) => String(option.value) === String(raw))?.label || stringValue(raw, '-')
  }
  return stringValue(raw, '-')
}

function valueIn(actual: ServiceConfigScalar, expected: ServiceConfigScalar | ServiceConfigScalar[]): boolean {
  const items = Array.isArray(expected) ? expected : [expected]
  return items.some((item) => String(item) === String(actual))
}

export function serviceInternalEndpoint(service: any): string {
  const serviceType = normalizedServiceType(service)
  const serviceName = runtimeServiceName(service, serviceType)
  const namespace = String(service?.namespace || '')
  if (!serviceName || !namespace) return ''
  return `${serviceName}.${namespace}.svc.cluster.local:${defaultServicePort(serviceType)}`
}

function runtimeServiceName(service: any, serviceType = normalizedServiceType(service)): string {
  const runtime = service?.runtimeConfig || {}
  const values = parseServiceValues(service?.values)
  const configured = stringValue(
    runtime.serviceName
      || runtime.service
      || values.fullnameOverride
      || values.nameOverride
      || service?.releaseName
      || service?.serviceName
      || service?.name
      || serviceType,
    ''
  )
  if (!configured) return ''
  if (serviceType === 'redis') {
    if (redisClusterEnabled(service)) return configured.replace(/-(master|replicas|headless)$/i, '')
    if (redisSentinelEnabled(service)) return configured.replace(/-(master|replicas|headless)$/i, '')
    if (configured.endsWith('-master') || configured.endsWith('-replicas') || configured.endsWith('-headless')) return configured
    return `${configured}-master`
  }
  return configured
}

function redisSentinelEnabled(service: any): boolean {
  const values = serviceConfigValues(service)
  return values['sentinel.enabled'] === 'true' || values.architecture === 'sentinel'
}

function redisClusterEnabled(service: any): boolean {
  const values = serviceConfigValues(service)
  return values.architecture === 'cluster'
}

function redisClusterStartupNodes(service: any, serviceHost: string): string {
  const values = serviceConfigValues(service)
  const namespace = String(service?.namespace || '')
  const serviceName = runtimeServiceName(service, 'redis')
  const nodes = Math.max(3, Number(values['cluster.nodes'] || 6) || 6)
  if (!namespace || !serviceName) return serviceHost
  return Array.from({ length: nodes }, (_, index) => `${serviceName}-${index}.${serviceName}-headless.${namespace}.svc.cluster.local:6379`).join(',')
}

function parseServiceValues(raw: any): Record<string, any> {
  if (!raw) return {}
  if (typeof raw === 'object') return raw
  if (typeof raw !== 'string') return {}
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function defaultServicePort(serviceType: string): number {
  const ports: Record<string, number> = {
    redis: 6379,
    mysql: 3306,
    postgresql: 5432,
    mongodb: 27017,
    rabbitmq: 5672,
    kafka: 9092,
    minio: 9000,
    eureka: 8761,
  }
  return ports[String(serviceType || '').toLowerCase()] || 80
}

function connectionEnvPrefix(serviceType: string): string {
  const normalized = String(serviceType || 'SERVICE').toUpperCase().replace(/[^A-Z0-9]+/g, '_')
  if (normalized === 'POSTGRESQL') return 'POSTGRES'
  return normalized || 'SERVICE'
}

export function serviceConfigType(service: any): string {
  const candidates = [
    service?.productKey,
    service?.templateType,
    service?.templateName,
    service?.chartName,
    service?.chart,
    service?.template?.chartName,
    service?.template?.name,
    service?.template?.s3Key,
    service?.template?.type,
    service?.serviceType,
    service?.type,
    service,
  ]
  for (const candidate of candidates) {
    const normalized = normalizeServiceConfigType(candidate)
    if (normalized) return normalized
  }
  return ''
}

function normalizeServiceConfigType(value: any): string {
  const text = String(value || '').trim().toLowerCase()
  if (!text) return ''
  if (text.includes('harbor')) return 'harbor'
  if (text.includes('docker-registry') || text.includes('registry.tar.gz') || text === 'registry' || text.includes('docker registry')) return 'docker-registry'
  if (text.includes('gitea')) return 'gitea'
  if (text.includes('jenkins')) return 'jenkins'
  if (text.includes('argocd') || text.includes('argo cd')) return 'argocd'
  if (text.includes('prometheus') || text.includes('grafana') || text.includes('kube-prometheus') || text.includes('monitor.tar.gz')) return 'prometheus-grafana'
  if (text.includes('loki') || text.includes('promtail') || text.includes('logging')) return 'loki'
  if (text.includes('postgres')) return 'postgresql'
  if (text.includes('mysql')) return 'mysql'
  if (text.includes('mongo')) return 'mongodb'
  if (text.includes('redis')) return 'redis'
  if (text.includes('rabbit')) return 'rabbitmq'
  if (text.includes('kafka')) return 'kafka'
  if (text.includes('minio')) return 'minio'
  return text.replace(/\.tar\.gz$/, '').replace(/^charts\//, '').replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '')
}

function normalizedServiceType(service: any): string {
  return String(service?.serviceType || service?.type || service || '').toLowerCase()
}

function flattenRecord(value: Record<string, any>, prefix = ''): Record<string, string> {
  const result: Record<string, string> = {}
  for (const [key, item] of Object.entries(value)) {
    if (item === undefined || item === null) continue
    const nextKey = prefix ? `${prefix}.${key}` : key
    if (item && typeof item === 'object' && !Array.isArray(item)) {
      Object.assign(result, flattenRecord(item, nextKey))
    } else {
      result[nextKey] = String(item)
    }
  }
  return result
}

function booleanValue(value: any, fallback: boolean): boolean {
  if (value === undefined || value === null || value === '') return fallback
  return String(value).toLowerCase() === 'true'
}

function optionalNumber(value: any): number | null {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null
}

function stringValue(value: any, fallback = ''): string {
  const text = String(value ?? '').trim()
  return text || fallback
}
