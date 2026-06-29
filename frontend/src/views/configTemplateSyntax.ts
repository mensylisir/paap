export const nativeConfigTemplateSyntax = '普通模式上传用户自己的配置文件，只把可变字段替换成 __TEMPLATE__KEY__显示名__；支持 DEFAULT、IF、FOR。template.json/schema.json 只用于高级模板包。'

type ParseOptions = {
  framework?: string
  fileName?: string
}

type TemplateField = Record<string, any>

export type ParsedNativeConfigTemplate = {
  syntax: string
  nativeConfigs: Array<Record<string, any>>
  fields: TemplateField[]
  env: Array<Record<string, any>>
  configMaps: Array<{ name?: string; data: Record<string, string> }>
  secrets: Array<{ name?: string; data: Record<string, string> }>
  files: Array<Record<string, any>>
  command: string[]
  args: string[]
}

const blockTokenRE = /__TEMPLATE__(FOR|IF|END)__([A-Z0-9_]+)(?:__([^_\n][\s\S]*?))?__/g
const valueTokenRE = /__TEMPLATE__(ITEM_[A-Z0-9_]+|[A-Z0-9_]+)__([^_\n][\s\S]*?)__(?:DEFAULT__([\s\S]*?)__)?/g

export function parseNativeConfigTemplate(source: string, options: ParseOptions = {}): ParsedNativeConfigTemplate {
  const framework = String(options.framework || 'auto').toLowerCase()
  const fieldMap = new Map<string, TemplateField>()
  const listFields = new Map<string, TemplateField>()
  const blockStack: Array<{ kind: 'FOR' | 'IF'; key: string }> = []
  let output = ''
  let cursor = 0

  const matches = Array.from(source.matchAll(new RegExp(`${blockTokenRE.source}|${valueTokenRE.source}`, 'g')))
  for (const match of matches) {
    output += source.slice(cursor, match.index)
    const token = match[0]
    cursor = Number(match.index) + token.length

    if (token.startsWith('__TEMPLATE__FOR__')) {
      const parsed = parseBlockToken(token, 'FOR')
      blockStack.push({ kind: 'FOR', key: parsed.key })
      listFields.set(parsed.key, listFields.get(parsed.key) || {
        key: parsed.key,
        label: parsed.label || labelFromKey(parsed.key),
        type: 'list',
        itemFields: [],
      })
      output += `[[paap:for ${parsed.key}]]`
      continue
    }
    if (token.startsWith('__TEMPLATE__IF__')) {
      const parsed = parseBlockToken(token, 'IF')
      blockStack.push({ kind: 'IF', key: parsed.key })
      addUniqueField(fieldMap, {
        key: parsed.key,
        label: parsed.label || labelFromKey(parsed.key),
        type: 'boolean',
      })
      output += `[[paap:if ${parsed.key}]]`
      continue
    }
    if (token.startsWith('__TEMPLATE__END__')) {
      const key = token.replace(/^__TEMPLATE__END__/, '').replace(/__$/, '')
      const block = blockStack.pop()
      output += block?.kind === 'FOR' ? `[[paap:end ${key}]]` : `[[paap:end ${key}]]`
      continue
    }

    const parsed = parseValueToken(token)
    if (parsed.key.startsWith('ITEM_')) {
      const list = [...blockStack].reverse().find(item => item.kind === 'FOR')
      const listKey = list?.key || 'ITEMS'
      const itemKey = parsed.key.replace(/^ITEM_/, '')
      const listField = listFields.get(listKey) || { key: listKey, label: labelFromKey(listKey), type: 'list', itemFields: [] }
      const itemFields = Array.isArray(listField.itemFields) ? listField.itemFields : []
      if (!itemFields.some((item:any) => item.key === itemKey)) {
        itemFields.push(inferField({ key: itemKey, label: parsed.label, defaultValue: parsed.defaultValue, item: true }))
      }
      listField.itemFields = itemFields
      listFields.set(listKey, listField)
      output += `[[paap:item.${itemKey}${parsed.defaultValue ? ` default=${parsed.defaultValue}` : ''}]]`
      continue
    }

    addUniqueField(fieldMap, inferField({ key: parsed.key, label: parsed.label, defaultValue: parsed.defaultValue }))
    output += `[[paap:${parsed.key}${parsed.defaultValue ? ` default=${parsed.defaultValue}` : ''}]]`
  }
  output += source.slice(cursor)

  const fileKey = options.fileName || defaultConfigFileName(framework)
  const fields = [...fieldMap.values(), ...listFields.values()]
  const env = defaultEnvForFramework(framework)
  const files = [{
    key: fileKey,
    name: fileKey,
    recommendedMountPath: defaultRecommendedMountPath(framework, fileKey),
    readOnly: true,
  }]

  return {
    syntax: nativeConfigTemplateSyntax,
    nativeConfigs: [{ name: fileKey, content: source }],
    fields,
    env,
    configMaps: [{ data: { [fileKey]: output } }],
    secrets: [],
    files,
    command: [],
    args: [],
  }
}

function parseBlockToken(token: string, kind: 'FOR' | 'IF') {
  const prefix = `__TEMPLATE__${kind}__`
  const body = token.slice(prefix.length, -2)
  const [key, ...labelParts] = body.split('__')
  return { key: key.trim(), label: labelParts.join('__').trim() }
}

function parseValueToken(token: string) {
  const body = token.slice('__TEMPLATE__'.length, -2)
  const defaultIndex = body.indexOf('__DEFAULT__')
  const main = defaultIndex >= 0 ? body.slice(0, defaultIndex) : body
  const defaultValue = defaultIndex >= 0 ? body.slice(defaultIndex + '__DEFAULT__'.length) : ''
  const [key, ...labelParts] = main.split('__')
  return { key: key.trim(), label: labelParts.join('__').trim(), defaultValue }
}

function addUniqueField(fields: Map<string, TemplateField>, field: TemplateField) {
  if (!field.key || fields.has(field.key)) return
  fields.set(field.key, field)
}

function inferField({ key, label, defaultValue, item = false }: { key: string; label: string; defaultValue?: string; item?: boolean }): TemplateField {
  const upper = key.toUpperCase()
  const field: TemplateField = {
    key,
    label: label || labelFromKey(key),
    type: inferFieldType(upper),
  }
  if (defaultValue) field.default = defaultValue
  if (/(PASSWORD|SECRET|TOKEN|PRIVATE_KEY|ACCESS_KEY)/.test(upper)) {
    field.type = 'password'
    field.output = 'secret'
    field.sensitive = true
  }
  if (!item) {
    const target = inferServiceTarget(upper)
    if (target) {
      field.type = 'serviceRef'
      field.target = target
      field.format = inferServiceFormat(upper)
    }
  }
  return field
}

function inferFieldType(key: string) {
  if (/DIRECTIVES|CONFIG_BLOCK|CONTENT|BLOCK$/.test(key)) return 'textarea'
  if (/PORT$|_PORT_/.test(key)) return 'number'
  if (/(ENABLED|ENABLE|USE)_?$/.test(key)) return 'boolean'
  return 'text'
}

function inferServiceTarget(key: string) {
  if (/(USER|USERNAME|PASSWORD|SECRET|TOKEN|KEY)$/.test(key)) return ''
  if (/(JDBC|DATABASE|DATASOURCE|POSTGRES|MYSQL)/.test(key) && /(URL|URI|HOST|ADDR|JDBC)/.test(key)) return 'postgresql|mysql'
  if (/REDIS/.test(key) && /(URL|URI|HOST|ADDR)/.test(key)) return 'redis'
  if (/(RABBIT|MQ)/.test(key) && /(URL|URI|HOST|ADDR)/.test(key)) return 'rabbitmq'
  if (/KAFKA/.test(key) && /(URL|URI|HOST|ADDR|BOOTSTRAP)/.test(key)) return 'kafka'
  if (/MINIO|S3/.test(key) && /(URL|URI|HOST|ADDR|ENDPOINT)/.test(key)) return 'minio'
  return ''
}

function inferServiceFormat(key: string) {
  if (/JDBC/.test(key)) return 'jdbcUrl'
  if (/(HOST|HOSTNAME)$/.test(key)) return 'host'
  if (/PORT$/.test(key)) return 'port'
  if (/(ADDR|ADDRESS|BOOTSTRAP|BOOTSTRAP_SERVERS)$/.test(key)) return 'addr'
  if (/(URL|URI|DSN)$/.test(key)) return 'url'
  return ''
}

function labelFromKey(key: string) {
  return key.toLowerCase().split('_').filter(Boolean).map(part => part.charAt(0).toUpperCase() + part.slice(1)).join(' ')
}

function defaultConfigFileName(framework: string) {
  if (framework === 'nginx') return 'default.conf'
  if (framework === 'springboot' || framework === 'spring') return 'application-paap.yml'
  if (framework === 'node' || framework === 'go' || framework === 'python') return '.env'
  return 'application.conf'
}

function defaultRecommendedMountPath(framework: string, fileKey: string) {
  if (framework === 'nginx') return '/etc/nginx/conf.d/default.conf'
  if (framework === 'springboot' || framework === 'spring') return `/etc/paap/${fileKey}`
  return `/etc/paap/${fileKey}`
}

function defaultEnvForFramework(framework: string) {
  if (framework === 'springboot' || framework === 'spring') {
    return [{ name: 'SPRING_CONFIG_ADDITIONAL_LOCATION', source: 'value', value: 'file:/etc/paap/' }]
  }
  return []
}
