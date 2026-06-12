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

const activeStatuses = new Set(['running', 'installing', 'pending', 'deleting'])

export interface PickerSessionState {
  availableServices: ReturnType<typeof buildPickerTemplates>
  selectedType: string
  loading: boolean
  notice: string
  error: string
}

export function isServiceActive(services: PickerService[], serviceType: string) {
  return services.some((svc) => svc.serviceType === serviceType && activeStatuses.has(String(svc.status || '').toLowerCase()))
}

export function buildPickerTemplates(templates: PickerTemplate[], services: PickerService[], mode: PickerMode) {
  return templates
    .filter((tmpl) => (mode === 'infra' ? tmpl.category === 'infra' : tmpl.category === 'tool'))
    .map((tmpl) => {
      const disabled = isServiceActive(services, tmpl.type)
      const active = services.find((svc) => svc.serviceType === tmpl.type)
      return {
        ...tmpl,
        disabled,
        statusText: disabled ? (String(active?.status || '').toLowerCase() === 'installing' ? '安装中' : '已安装') : '可安装',
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
    notice: templates.length === 0 ? '正在加载可安装模板...' : pickerNotice(mode, availableServices.length, selectedType),
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
        ? '当前环境中的中间件模板均已安装或正在安装。'
        : '当前环境中的工具模板均已安装或正在安装。')
      : ''
}
