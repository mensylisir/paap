import { templatePlaceholderDefault } from './configTemplateRenderer'

export type ComponentTemplateField = {
  key?: string
  label?: string
  type?: string
  target?: string
  required?: boolean
  default?: any
  description?: string
  itemFields?: ComponentTemplateField[]
}

export type ComponentTemplateFieldValues = Record<string, any>

export type ComponentTemplateInitialFieldOptions = {
  existingTargetKey?: string
  firstOptionValue?: string
}

export type ComponentTemplateRenderTargetOptions = {
  credentials?: any[]
  endpoint?: string
  defaultPort?: number
  credentialValue?: (credentials: any[], keys: string[]) => string
}

export type ComponentTemplateExistingFieldValueOptions = {
  env?: Array<Record<string, any>>
  secrets?: Array<Record<string, any>>
  configMaps?: Array<Record<string, any>>
}

export type ComponentConfigTemplateMatchOptions = {
  componentType?: string
  framework?: string
  componentName?: string
}

const booleanTrueValues = new Set(['1', 'true', 'yes', 'on', 'enabled'])

export const componentConfigTemplateEffectiveType = (options: ComponentConfigTemplateMatchOptions) => {
  const componentType = String(options.componentType || 'custom').toLowerCase()
  const componentFramework = String(options.framework || 'auto').toLowerCase()
  if (componentType === 'frontend' && ['springboot', 'python', 'go'].includes(componentFramework)) {
    return 'backend'
  }
  return componentType || 'custom'
}

export const componentConfigTemplateMatchesComponent = (
  template: any,
  options: ComponentConfigTemplateMatchOptions,
) => {
  const componentType = componentConfigTemplateEffectiveType(options)
  const componentFramework = String(options.framework || 'auto').toLowerCase()
  const templateFramework = String(template?.framework || 'auto').toLowerCase()
  const types = Array.isArray(template?.componentTypes)
    ? template.componentTypes.map((item:any) => String(item).toLowerCase()).filter(Boolean)
    : []
  const componentName = normalizedTemplateMatchText(options.componentName)
  const templateText = normalizedTemplateMatchText(`${template?.key || ''} ${template?.name || ''}`)
  const nameMatches = Boolean(componentName)
    && componentName.split(/\s+/).filter(Boolean).some((token) =>
      templateText.split(/\s+/).includes(token) || (token.length >= 3 && templateText.includes(token))
    )
  const explicitTypeMatch = types.includes(componentType)
  const typeMatches = !types.length || explicitTypeMatch || types.includes('custom') || nameMatches
  const frameworkMatches = templateFramework === 'auto'
    || componentFramework === 'auto'
    || templateFramework === componentFramework

  return typeMatches && (frameworkMatches || explicitTypeMatch || nameMatches)
}

const normalizedTemplateMatchText = (value: unknown) => String(value || '')
  .toLowerCase()
  .replace(/[^a-z0-9]+/g, ' ')
  .trim()

export const componentConfigTemplateRecommendationScore = (
  template: any,
  options: ComponentConfigTemplateMatchOptions,
) => {
  if (!componentConfigTemplateMatchesComponent(template, options)) return -1

  const componentType = componentConfigTemplateEffectiveType(options)
  const componentFramework = String(options.framework || 'auto').toLowerCase()
  const templateFramework = String(template?.framework || 'auto').toLowerCase()
  const types = Array.isArray(template?.componentTypes)
    ? template.componentTypes.map((item:any) => String(item).toLowerCase()).filter(Boolean)
    : []
  const componentName = normalizedTemplateMatchText(options.componentName)
  const templateText = normalizedTemplateMatchText(`${template?.key || ''} ${template?.name || ''}`)
  let score = 10

  if (componentFramework && componentFramework !== 'auto' && templateFramework === componentFramework) score += 30
  if (types.includes(componentType)) score += 20
  if (componentName) {
    const tokens = componentName.split(/\s+/).filter(Boolean)
    if (tokens.some((token) => templateText.split(/\s+/).includes(token))) score += 50
    else if (tokens.some((token) => token.length >= 3 && templateText.includes(token))) score += 35
  }
  if (template?.isBuiltin) score += 2

  return score
}

export const componentConfigTemplateSelectValue = (template: any) => String(template?.id || template?.key || '').trim()

export const componentConfigTemplateMatchesSelection = (template: any, selection: unknown) => {
  const selected = String(selection || '').trim()
  if (!selected) return false
  return componentConfigTemplateSelectValue(template) === selected
    || String(template?.key || '').trim() === selected
    || String(template?.name || '').trim() === selected
    || (Number(selected) > 0 && Number(template?.id || 0) === Number(selected))
}

export const resolveComponentConfigTemplateSelection = (templates: any[], selection: unknown) => {
  const selected = String(selection || '').trim()
  if (!selected) return ''
  const match = templates.find((template) => componentConfigTemplateMatchesSelection(template, selected))
  return match ? componentConfigTemplateSelectValue(match) : selected
}

export const componentTemplateFieldKey = (field: ComponentTemplateField | any) => String(field?.key || '').trim()

export const componentTemplateFieldLabel = (field: ComponentTemplateField | any) =>
  String(field?.label || componentTemplateFieldKey(field) || '配置项').trim()

export const componentTemplateFieldType = (field: ComponentTemplateField | any) =>
  String(field?.type || 'text').trim().toLowerCase()

export const componentTemplateFieldRequired = (field: ComponentTemplateField | any) => Boolean(field?.required)

export const componentTemplateFieldTargetTokens = (field: ComponentTemplateField | any) => String(field?.target || '')
  .toLowerCase()
  .split('|')
  .map((item) => item.trim())
  .filter(Boolean)

const databaseTargetTokens = new Set(['postgresql', 'postgres', 'mysql', 'mariadb', 'mongodb', 'database'])

const normalizedTemplateKey = (field: ComponentTemplateField | any) =>
  componentTemplateFieldKey(field).replace(/([a-z0-9])([A-Z])/g, '$1_$2').replace(/[.\-]/g, '_').toUpperCase()

const templateFieldServiceGroup = (field: ComponentTemplateField | any) => {
  const normalized = normalizedTemplateKey(field)
  const firstSegment = componentTemplateFieldKey(field).split('.')[0]?.toLowerCase() || ''
  if (/(^|_)(JDBC|DATABASE|DATASOURCE|DB|POSTGRES|POSTGRESQL|MYSQL|MONGODB)(_|$)/.test(normalized)) return 'database'
  if (normalized.includes('REDIS')) return 'redis'
  if (normalized.includes('RABBIT') || normalized.includes('AMQP')) return 'rabbitmq'
  if (normalized.includes('KAFKA')) return 'kafka'
  if (normalized.includes('MINIO') || normalized.includes('S3') || normalized.includes('AWS')) return 'minio'
  return firstSegment
}

export const componentTemplateServiceTypeGroup = (serviceType: string) => {
  const normalized = String(serviceType || '').trim().toLowerCase()
  if (['postgresql-ha', 'postgres-ha'].includes(normalized)) return 'database'
  if (normalized === 'mysql-galera') return 'database'
  if (normalized === 'redis-cluster') return 'redis'
  if (normalized === 'rabbitmq-cluster') return 'rabbitmq'
  if (databaseTargetTokens.has(normalized)) return 'database'
  if (normalized === 'redis') return 'redis'
  if (normalized === 'rabbitmq') return 'rabbitmq'
  if (normalized === 'kafka') return 'kafka'
  if (normalized === 'minio') return 'minio'
  if (normalized === 'harbor' || normalized === 'docker-registry') return 'registry'
  return normalized
}

const serviceTypeAliases = (serviceType: string) => {
  const normalized = String(serviceType || '').trim().toLowerCase()
  const aliases = new Set<string>([normalized, componentTemplateServiceTypeGroup(normalized)])
  if (normalized === 'postgres' || normalized === 'postgresql-ha' || normalized === 'postgres-ha') aliases.add('postgresql')
  if (normalized === 'mysql-galera') aliases.add('mysql')
  if (normalized === 'mariadb') aliases.add('mysql')
  if (normalized === 'redis-cluster') aliases.add('redis')
  if (normalized === 'rabbitmq-cluster') aliases.add('rabbitmq')
  if (normalized === 'harbor' || normalized === 'docker-registry') aliases.add('registry')
  return aliases
}

export const componentTemplateServiceTypeMatchesTarget = (serviceType: string, target: string) => {
  const token = String(target || '').trim().toLowerCase()
  if (!token) return false
  return serviceTypeAliases(serviceType).has(token)
}

export const componentTemplateServiceTypeMatchesTargets = (serviceType: string, targets: string[]) => {
  if (!targets.length) return true
  return targets.some((target) => componentTemplateServiceTypeMatchesTarget(serviceType, target))
}

export const componentTemplateFieldMatchesServiceRef = (
  field: ComponentTemplateField | any,
  serviceRefField: ComponentTemplateField | any,
) => {
  if (componentTemplateFieldType(field) !== 'password') return false
  if (componentTemplateFieldType(serviceRefField) !== 'serviceref') return false
  const fieldGroup = templateFieldServiceGroup(field)
  const targets = componentTemplateFieldTargetTokens(serviceRefField)
  if (!fieldGroup || !targets.length) return false
  return targets.some((target) => componentTemplateServiceTypeGroup(target) === fieldGroup)
}

export const componentTemplateServicePasswordFieldKeys = (
  fields: Array<ComponentTemplateField | any>,
  serviceType: string,
) => {
  const group = componentTemplateServiceTypeGroup(serviceType)
  if (!group) return []
  return fields
    .filter((field) => componentTemplateFieldType(field) === 'password' && templateFieldServiceGroup(field) === group)
    .map(componentTemplateFieldKey)
    .filter(Boolean)
}

export const componentTemplateServiceUsernameFieldKeys = (
  fields: Array<ComponentTemplateField | any>,
  serviceType: string,
) => {
  const group = componentTemplateServiceTypeGroup(serviceType)
  if (!group) return []
  return fields
    .filter((field) => {
      if (componentTemplateFieldType(field) === 'password') return false
      const normalized = normalizedTemplateKey(field)
      if (!/(^|_)(USER|USERNAME)(_|$)/.test(normalized)) return false
      return templateFieldServiceGroup(field) === group
    })
    .map(componentTemplateFieldKey)
    .filter(Boolean)
}

export const componentTemplateFieldDefaultValue = (field: ComponentTemplateField | any) => {
  const value = field?.default
  if (value !== undefined && value !== null) return componentTemplateInputValue(String(value))
  return templatePlaceholderDefault(componentTemplateFieldKey(field), '')
}

export const componentTemplateInputValue = (value: string) => {
  const text = String(value ?? '').trim()
  const token = text.match(/^\[\[\s*paap:([^\]\s]+)([^\]]*)\]\]$/)
  if (!token) return String(value ?? '')
  return templatePlaceholderDefault(String(token[1] || ''), String(token[2] || ''))
}

export const componentTemplateFieldInputType = (field: ComponentTemplateField | any) => {
  const type = componentTemplateFieldType(field)
  if (type === 'password') return 'password'
  if (type === 'number') return 'number'
  return 'text'
}

export const componentTemplateListItemFields = (field: ComponentTemplateField | any) => Array.isArray(field?.itemFields)
  ? field.itemFields.filter((item:any) => componentTemplateFieldKey(item))
  : []

export const componentTemplateListRows = (
  field: ComponentTemplateField | any,
  fieldValues: ComponentTemplateFieldValues,
) => {
  const rows = fieldValues[componentTemplateFieldKey(field)]
  return Array.isArray(rows) ? rows : []
}

export const defaultComponentTemplateListRow = (field: ComponentTemplateField | any) => {
  const row: Record<string, string> = {}
  for (const itemField of componentTemplateListItemFields(field)) {
    row[componentTemplateFieldKey(itemField)] = componentTemplateFieldDefaultValue(itemField)
  }
  return row
}

export const componentTemplateInitialFieldValue = (
  field: ComponentTemplateField | any,
  options: ComponentTemplateInitialFieldOptions = {},
) => {
  if (componentTemplateFieldType(field) === 'list') {
    return [defaultComponentTemplateListRow(field)]
  }
  if (componentTemplateFieldType(field) === 'boolean') {
    return booleanTrueValues.has(componentTemplateFieldDefaultValue(field).toLowerCase())
  }
  if (componentTemplateFieldType(field) === 'serviceref') {
    return options.existingTargetKey || options.firstOptionValue || ''
  }
  return componentTemplateFieldDefaultValue(field)
}

export const componentTemplateExistingFieldValue = (
  field: ComponentTemplateField | any,
  options: ComponentTemplateExistingFieldValueOptions = {},
) => {
  const candidates = componentTemplateExistingFieldNames(field)
  if (!candidates.length) return ''

  for (const env of options.env || []) {
    const name = String(env?.name || '').trim().toUpperCase()
    if (candidates.includes(name) && env?.value !== undefined && env?.value !== null && String(env.value).trim()) {
      return componentTemplateInputValue(String(env.value))
    }
  }
  for (const group of [...options.secrets || [], ...options.configMaps || []]) {
    const data = group?.data || {}
    for (const [key, value] of Object.entries(data)) {
      if (candidates.includes(String(key).trim().toUpperCase()) && value !== undefined && value !== null && String(value).trim()) {
        return componentTemplateInputValue(String(value))
      }
    }
  }
  return ''
}

export const componentTemplateRequiredFieldsComplete = (
  fields: Array<ComponentTemplateField | any>,
  fieldValues: ComponentTemplateFieldValues,
  options: { isRequiredForUser?: (field: ComponentTemplateField | any) => boolean } = {},
) => fields.every((field) => {
  const required = options.isRequiredForUser ? options.isRequiredForUser(field) : componentTemplateFieldRequired(field)
  if (!required) return true
  const key = componentTemplateFieldKey(field)
  if (componentTemplateFieldType(field) === 'list') {
    const rows = componentTemplateListRows(field, fieldValues)
    const requiredItems = componentTemplateListItemFields(field).filter(componentTemplateFieldRequired)
    if (!rows.length) return false
    if (!requiredItems.length) return true
    return rows.every((row:any) => requiredItems.every((itemField:any) =>
      String(row?.[componentTemplateFieldKey(itemField)] || '').trim().length > 0
    ))
  }
  return String(fieldValues[key] || '').trim().length > 0
})

export const componentTemplateCredentialPasswordKeys = (serviceType: string) => {
  if (serviceType === 'postgresql') return ['postgres-password', 'password']
  if (serviceType === 'mysql') return ['mysql-root-password', 'mysql-password', 'password']
  if (serviceType === 'redis') return ['redis-password', 'password']
  if (serviceType === 'mongodb') return ['mongodb-root-password', 'mongodb-password', 'password']
  return ['password']
}

export const componentTemplateCredentialUsernameKeys = (serviceType: string) => {
  if (serviceType === 'postgresql') return ['postgres-username', 'username']
  if (serviceType === 'mysql') return ['mysql-root-user', 'mysql-username', 'username']
  if (serviceType === 'mongodb') return ['mongodb-root-user', 'mongodb-username', 'username']
  return ['username']
}

export const componentTemplateDefaultCredentialUsername = (serviceType: string) => {
  if (serviceType === 'mysql') return 'root'
  if (serviceType === 'mongodb') return 'root'
  return 'postgres'
}

export const componentTemplateDefaultCredentialDatabase = (serviceType: string) => {
  if (serviceType === 'mysql') return 'mysql'
  if (serviceType === 'mongodb') return 'admin'
  return 'postgres'
}

export const componentTemplateRenderTargetValue = (
  field: ComponentTemplateField | any,
  target: any,
  options: ComponentTemplateRenderTargetOptions = {},
) => {
  if (!target) return ''
  if (target.kind === 'component') return `http://${target.serviceName || target.name}`

  const serviceType = String(target.type || '').toLowerCase()
  const [host, portText] = componentTemplateSplitEndpoint(options.endpoint || '', options.defaultPort || 80)
  const readCredential = options.credentialValue || defaultCredentialValue
  const credentials = options.credentials || []
  const password = readCredential(credentials, componentTemplateCredentialPasswordKeys(serviceType))
  const passwordPart = password ? `:${encodeURIComponent(password)}` : ''
  const username = readCredential(credentials, componentTemplateCredentialUsernameKeys(serviceType))
    || componentTemplateDefaultCredentialUsername(serviceType)
  const database = componentTemplateDefaultCredentialDatabase(serviceType)
  const format = componentTemplateFieldFormat(field)

  if (format === 'jdbcUrl') {
    if (serviceType === 'mysql') return `jdbc:mysql://${host}:${portText}/${database}`
    if (serviceType === 'postgresql') return `jdbc:postgresql://${host}:${portText}/${database}`
  }
  if (format === 'url') {
    if (serviceType === 'postgresql') return `postgresql://${username}${passwordPart}@${host}:${portText}/${database}`
    if (serviceType === 'mysql') return `mysql://${username}${passwordPart}@${host}:${portText}/${database}`
    if (serviceType === 'mongodb') return `mongodb://${username}${passwordPart}@${host}:${portText}/${database}`
    if (serviceType === 'redis') return password ? `redis://:${encodeURIComponent(password)}@${host}:${portText}` : `redis://${host}:${portText}`
    if (serviceType === 'eureka') return `http://${host}:${portText}/eureka/`
  }
  if (format === 'eurekaUrl') return `http://${host}:${portText}/eureka/`
  if (format === 'addr') return `${host}:${portText}`
  if (format === 'host') return host
  if (format === 'port') return String(portText)
  return options.endpoint || host
}

function componentTemplateFieldFormat(field: ComponentTemplateField | any) {
  const declared = String(field?.format || '').trim()
  if (declared) return declared === 'jdbcurl' ? 'jdbcUrl' : declared

  const key = componentTemplateFieldKey(field)
  const normalized = key.replace(/([a-z0-9])([A-Z])/g, '$1_$2').replace(/[.\-]/g, '_').toUpperCase()
  if (normalized.includes('JDBC')) return 'jdbcUrl'
  if (/(^|_)HOST(NAME)?$/.test(normalized) || normalized.endsWith('_HOST')) return 'host'
  if (/(^|_)PORT$/.test(normalized) || normalized.endsWith('_PORT')) return 'port'
  if (/(^|_)(ADDR|ADDRESS|BOOTSTRAP|BOOTSTRAP_SERVERS)$/.test(normalized) || /(ADDR|ADDRESS|BOOTSTRAP|BOOTSTRAP_SERVERS)$/.test(normalized)) return 'addr'
  if (/(^|_)(URL|URI|DSN)$/.test(normalized) || /(URL|URI|DSN)$/.test(normalized)) return 'url'
  return ''
}

function defaultCredentialValue(credentials: any[], keys: string[]) {
  for (const key of keys) {
    const match = credentials.find((item:any) => String(item.key || '').toLowerCase() === key)
    if (match?.value) return String(match.value)
  }
  return ''
}

function componentTemplateExistingFieldNames(field: ComponentTemplateField | any) {
  const normalized = componentTemplateFieldKey(field).replace(/([a-z0-9])([A-Z])/g, '$1_$2').replace(/[.\-]/g, '_').toUpperCase()
  const names = new Set<string>([normalized])
  if (normalized === 'DATABASE_PASSWORD' || normalized === 'DB_PASSWORD' || normalized === 'JDBC_PASSWORD') {
    names.add('POSTGRES_PASSWORD')
    names.add('POSTGRESQL_PASSWORD')
    names.add('MYSQL_PASSWORD')
    names.add('MYSQL_ROOT_PASSWORD')
    names.add('SPRING_DATASOURCE_PASSWORD')
    names.add('DATABASE_PASSWORD')
    names.add('DB_PASSWORD')
  }
  if (normalized === 'DATABASE_URL' || normalized === 'JDBC_URL') {
    names.add('DATABASE_URL')
    names.add('JDBC_URL')
    names.add('SPRING_DATASOURCE_URL')
  }
  if (normalized === 'DATABASE_USER' || normalized === 'DATABASE_USERNAME' || normalized === 'JDBC_USER') {
    names.add('POSTGRES_USER')
    names.add('POSTGRES_USERNAME')
    names.add('MYSQL_USER')
    names.add('MYSQL_USERNAME')
    names.add('SPRING_DATASOURCE_USERNAME')
  }
  if (normalized === 'REDIS_PASSWORD') {
    names.add('REDIS_PASSWORD')
  }
  if (normalized === 'REDIS_HOST') {
    names.add('REDIS_HOST')
  }
  if (normalized === 'REDIS_PORT') {
    names.add('REDIS_PORT')
  }
  return [...names].filter(Boolean)
}

export function componentTemplateSplitEndpoint(endpoint: string, fallbackPort: number) {
  const clean = String(endpoint || '').trim()
  if (!clean) return ['service', fallbackPort] as const

  const parseCandidate = /^[a-z][a-z0-9+.-]*:\/\//i.test(clean) ? clean : `tcp://${clean}`
  try {
    const parsed = new URL(parseCandidate)
    const host = parsed.hostname.replace(/^\[|\]$/g, '')
    const port = Number(parsed.port)
    if (host) return [host, Number.isFinite(port) && port > 0 ? port : fallbackPort] as const
  } catch {
    // Fall through to the permissive parser for incomplete user-entered values.
  }

  const authority = clean
    .replace(/^[a-z][a-z0-9+.-]*:\/\//i, '')
    .split(/[/?#]/)[0]
    .split('@')
    .pop() || ''
  const idx = authority.lastIndexOf(':')
  if (idx > 0) {
    const port = Number(authority.slice(idx + 1))
    if (Number.isFinite(port) && port > 0) return [authority.slice(0, idx), port] as const
  }
  return [authority || 'service', fallbackPort] as const
}
