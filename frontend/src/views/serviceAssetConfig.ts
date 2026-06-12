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
  redis: {
    kind: 'redis',
    showDeploymentConfig: true,
    showConnectionBindings: true,
    showTopology: true,
    showWorkspaceSummary: false,
    workspaceTitle: '',
    summaryKeys: ['architecture', 'replica.replicaCount', 'sentinel.enabled', 'master.persistence.enabled'],
    fields: [
      { key: 'architecture', label: 'Redis 架构', control: 'select', defaultValue: 'replication', options: [{ label: 'Redis 单节点', value: 'standalone' }, { label: 'Redis 主从复制', value: 'replication' }] },
      { key: 'sentinel.enabled', label: 'Sentinel 高可用', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'replica.replicaCount', label: 'Redis 副本', control: 'number', defaultValue: 3, min: 1, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'master.persistence.enabled', label: 'Master 存储', control: 'switch', defaultValue: false },
      { key: 'master.persistence.size', label: 'Master 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'master.persistence.enabled', equals: true } },
      { key: 'replica.persistence.enabled', label: 'Replica 存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'replica.persistence.size', label: 'Replica 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'replica.persistence.enabled', equals: true } },
    ],
  },
  mysql: {
    kind: 'database',
    showDeploymentConfig: true,
    showConnectionBindings: true,
    showTopology: true,
    showWorkspaceSummary: false,
    workspaceTitle: '',
    summaryKeys: ['architecture', 'secondary.replicaCount', 'primary.persistence.enabled'],
    fields: [
      { key: 'architecture', label: 'MySQL 架构', control: 'select', defaultValue: 'standalone', options: [{ label: '单主库', value: 'standalone' }, { label: '主从复制', value: 'replication' }] },
      { key: 'secondary.replicaCount', label: 'Secondary 副本', control: 'number', defaultValue: 1, min: 1, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'primary.persistence.enabled', label: 'Primary 存储', control: 'switch', defaultValue: false },
      { key: 'primary.persistence.size', label: 'Primary 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'primary.persistence.enabled', equals: true } },
      { key: 'secondary.persistence.enabled', label: 'Secondary 存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'secondary.persistence.size', label: 'Secondary 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'secondary.persistence.enabled', equals: true } },
    ],
  },
  postgresql: {
    kind: 'database',
    showDeploymentConfig: true,
    showConnectionBindings: true,
    showTopology: true,
    showWorkspaceSummary: false,
    workspaceTitle: '',
    summaryKeys: ['architecture', 'readReplicas.replicaCount', 'primary.persistence.enabled'],
    fields: [
      { key: 'architecture', label: 'PostgreSQL 架构', control: 'select', defaultValue: 'standalone', options: [{ label: '单主库', value: 'standalone' }, { label: '主库 + 只读副本', value: 'replication' }] },
      { key: 'readReplicas.replicaCount', label: 'Read Replica 数', control: 'number', defaultValue: 1, min: 1, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'primary.persistence.enabled', label: 'Primary 存储', control: 'switch', defaultValue: false },
      { key: 'primary.persistence.size', label: 'Primary 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'primary.persistence.enabled', equals: true } },
      { key: 'readReplicas.persistence.enabled', label: 'Read Replica 存储', control: 'switch', defaultValue: false, showWhen: { key: 'architecture', equals: 'replication' } },
      { key: 'readReplicas.persistence.size', label: 'Read Replica 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'readReplicas.persistence.enabled', equals: true } },
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
      { key: 'persistence.size', label: '存储容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'persistence.enabled', equals: true } },
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
      { key: 'persistence.size', label: '存储容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'persistence.enabled', equals: true } },
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
      { key: 'controller.persistence.size', label: 'Controller 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'controller.persistence.enabled', equals: true } },
      { key: 'broker.persistence.enabled', label: 'Broker 存储', control: 'switch', defaultValue: false },
      { key: 'broker.persistence.size', label: 'Broker 容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'broker.persistence.enabled', equals: true } },
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
      { key: 'persistence.size', label: '存储容量', control: 'text', defaultValue: '8Gi', placeholder: '8Gi', showWhen: { key: 'persistence.enabled', equals: true } },
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

export function serviceConfigProfile(service: any): ServiceConfigProfile {
  const serviceType = normalizedServiceType(service)
  const base = serviceProfiles[serviceType] || (serviceType ? toolProfile : unknownProfile)
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
  const values = serviceConfigValues(service)
  const form: ServiceConfigForm = {}
  for (const field of serviceConfigFields(service)) {
    form[field.key] = formValueFromValues(field, values)
  }
  return normalizeServiceForm(normalizedServiceType(service), form)
}

export function serviceConfigValuesFromForm(serviceType: string, form: ServiceConfigForm): Record<string, string> {
  const type = String(serviceType || '').toLowerCase()
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
  for (const key of profile.summaryKeys) {
    if (key === '$endpoint') {
      rows.push({ label: '连接地址', value: serviceInternalEndpoint(service) || '等待生成' })
      continue
    }
    const field = profile.fields.find((item) => item.key === key)
    if (!field) continue
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

  const nodes = resources
    .filter((resource) => {
      const type = String(resource.type || '').toLowerCase()
      return !type.includes('secret') && !type.includes('config')
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

export function connectionBindingPreview(component: any, service: any): ConnectionBindingPreview {
  const serviceType = normalizedServiceType(service)
  const serviceName = String(service?.serviceName || service?.name || serviceType || 'service')
  const namespace = String(service?.namespace || '')
  const port = defaultServicePort(serviceType)
  const prefix = connectionEnvPrefix(serviceType)
  const host = namespace ? `${serviceName}.${namespace}.svc.cluster.local` : serviceName
  const bindings = [
    { name: `${prefix}_HOST`, value: host, source: serviceName },
    { name: `${prefix}_PORT`, value: String(port), source: serviceName },
  ]
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
    if (form.architecture !== 'replication') form['sentinel.enabled'] = false
    if (form['sentinel.enabled'] === true) form.architecture = 'replication'
  }
  if ((serviceType === 'mysql' || serviceType === 'postgresql') && form.architecture !== 'replication') {
    if (serviceType === 'mysql') form['secondary.persistence.enabled'] = false
    if (serviceType === 'postgresql') form['readReplicas.persistence.enabled'] = false
  }
  if (serviceType === 'minio' && form.mode !== 'distributed') {
    form['statefulset.replicaCount'] = form['statefulset.replicaCount'] || 4
  }
  return form
}

function normalizeServiceValues(serviceType: string, values: Record<string, string>, form: ServiceConfigForm): Record<string, string> {
  if (serviceType === 'redis') {
    if (form.architecture !== 'replication') {
      values['sentinel.enabled'] = 'false'
    }
    if (form['sentinel.enabled'] === true) {
      values.architecture = 'replication'
      values['sentinel.enabled'] = 'true'
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
  const serviceName = String(service?.serviceName || service?.serviceType || service?.name || '')
  const namespace = String(service?.namespace || '')
  if (!serviceName || !namespace) return ''
  return `${serviceName}.${namespace}.svc.cluster.local:${defaultServicePort(service?.serviceType)}`
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
  }
  return ports[String(serviceType || '').toLowerCase()] || 80
}

function connectionEnvPrefix(serviceType: string): string {
  const normalized = String(serviceType || 'SERVICE').toUpperCase().replace(/[^A-Z0-9]+/g, '_')
  if (normalized === 'POSTGRESQL') return 'POSTGRES'
  return normalized || 'SERVICE'
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
