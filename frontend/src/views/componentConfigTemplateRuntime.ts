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
}

const booleanTrueValues = new Set(['1', 'true', 'yes', 'on', 'enabled'])

export const componentConfigTemplateMatchesComponent = (
  template: any,
  options: ComponentConfigTemplateMatchOptions,
) => {
  const componentType = String(options.componentType || 'custom').toLowerCase()
  const componentFramework = String(options.framework || 'auto').toLowerCase()
  const templateFramework = String(template?.framework || 'auto').toLowerCase()
  const types = Array.isArray(template?.componentTypes)
    ? template.componentTypes.map((item:any) => String(item).toLowerCase()).filter(Boolean)
    : []
  const explicitTypeMatch = types.includes(componentType)
  const typeMatches = !types.length || explicitTypeMatch || types.includes('custom')
  const frameworkMatches = templateFramework === 'auto'
    || componentFramework === 'auto'
    || templateFramework === componentFramework

  return typeMatches && (frameworkMatches || explicitTypeMatch)
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

const databaseTargetTokens = new Set(['postgresql', 'postgres', 'mysql', 'mongodb', 'database'])

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

const serviceTypeGroup = (serviceType: string) => {
  const normalized = String(serviceType || '').trim().toLowerCase()
  if (databaseTargetTokens.has(normalized)) return 'database'
  if (normalized === 'redis') return 'redis'
  if (normalized === 'rabbitmq') return 'rabbitmq'
  if (normalized === 'kafka') return 'kafka'
  if (normalized === 'minio') return 'minio'
  return normalized
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
  return targets.some((target) => serviceTypeGroup(target) === fieldGroup)
}

export const componentTemplateServicePasswordFieldKeys = (
  fields: Array<ComponentTemplateField | any>,
  serviceType: string,
) => {
  const group = serviceTypeGroup(serviceType)
  if (!group) return []
  return fields
    .filter((field) => componentTemplateFieldType(field) === 'password' && templateFieldServiceGroup(field) === group)
    .map(componentTemplateFieldKey)
    .filter(Boolean)
}

export const componentTemplateFieldDefaultValue = (field: ComponentTemplateField | any) => {
  const value = field?.default
  if (value !== undefined && value !== null) return String(value)
  return templatePlaceholderDefault(componentTemplateFieldKey(field), '')
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
      return String(env.value)
    }
  }
  for (const group of [...options.secrets || [], ...options.configMaps || []]) {
    const data = group?.data || {}
    for (const [key, value] of Object.entries(data)) {
      if (candidates.includes(String(key).trim().toUpperCase()) && value !== undefined && value !== null && String(value).trim()) {
        return String(value)
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
  const [host, portText] = splitEndpoint(options.endpoint || '', options.defaultPort || 80)
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
  }
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

function splitEndpoint(endpoint: string, fallbackPort: number) {
  const clean = String(endpoint || '').replace(/^https?:\/\//, '')
  const idx = clean.lastIndexOf(':')
  if (idx > 0) {
    const port = Number(clean.slice(idx + 1))
    if (Number.isFinite(port)) return [clean.slice(0, idx), port] as const
  }
  return [clean || 'service', fallbackPort] as const
}
