export type PickerMode = 'tool' | 'infra'

export interface PickerTemplate {
  type: string
  name: string
  category: string
  description?: string
  [key: string]: any
}

export interface PickerService {
  serviceType: string
  status?: string
}

export interface PickerSessionState {
  availableServices: ReturnType<typeof buildPickerTemplates>
  selectedType: string
  loading: boolean
  notice: string
  error: string
}

const toolServiceTypes = new Set([
  'git',
  'gitea',
  'ci',
  'jenkins',
  'deploy',
  'argocd',
  'monitor',
  'prometheus',
  'grafana',
  'log',
  'loki',
  'registry',
  'docker-registry',
  'harbor',
])

const infraServiceTypes = new Set([
  'postgresql',
  'postgresql-ha',
  'mysql',
  'mysql-galera',
  'mongodb',
  'redis',
  'redis-cluster',
  'rabbitmq',
  'kafka',
  'minio',
  'nacos',
  'eureka',
])

export function isServiceActive(services: PickerService[], serviceType: string) {
  return services.some((svc) => svc.serviceType === serviceType)
}

function servicePickerStatusText(status?: string) {
  const value = String(status || '').toLowerCase()
  if (value === 'installing') return '安装中'
  if (value === 'draft' || value === 'pending') return '已添加'
  if (value === 'failed' || value === 'error') return '安装失败'
  if (value === 'deleting') return '删除中'
  return '已安装'
}

function pickerTemplateMode(tmpl: PickerTemplate): PickerMode {
  const type = String(tmpl.type || '').toLowerCase()
  if (toolServiceTypes.has(type)) return 'tool'
  if (infraServiceTypes.has(type)) return 'infra'

  const category = String(tmpl.category || '').toLowerCase()
  if (['ci', 'cd', 'deploy', 'monitor', 'log', 'logging', 'tool'].includes(category)) return 'tool'
  if (['infra', 'database', 'middleware'].includes(category)) return 'infra'
  return 'tool'
}

export function buildPickerTemplates(templates: PickerTemplate[], services: PickerService[], mode: PickerMode) {
  return templates
    .filter((tmpl) => pickerTemplateMode(tmpl) === mode)
    .map((tmpl) => {
      const disabled = isServiceActive(services, tmpl.type)
      const active = services.find((svc) => svc.serviceType === tmpl.type)
      return {
        ...tmpl,
        disabled,
        statusText: disabled ? servicePickerStatusText(active?.status) : '可添加',
      }
    })
}

export function createPickerSessionState(templates: PickerTemplate[], services: PickerService[], mode: PickerMode): PickerSessionState {
  const availableServices = buildPickerTemplates(templates, services, mode)
  const selectedType = availableServices.find((svc) => !svc.disabled)?.type || ''
  return {
    availableServices,
    selectedType,
    loading: templates.length === 0,
    notice: templates.length === 0 ? '正在加载可添加模板...' : pickerNotice(mode, availableServices.length, selectedType),
    error: '',
  }
}

export function pickerNotice(mode: PickerMode, templateCount: number, selectedType: string) {
  if (templateCount === 0) {
    return mode === 'infra'
      ? '当前没有可用的中间件模板。'
      : '当前没有可用的工具模板。'
  }
  return !selectedType
      ? (mode === 'infra'
        ? '当前环境中的中间件模板均已添加、安装或正在安装。'
        : '当前环境中的工具模板均已添加、安装或正在安装。')
      : ''
}
