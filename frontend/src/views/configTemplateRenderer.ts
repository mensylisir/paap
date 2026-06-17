type RenderContext = {
  componentName?: string
  configMapName?: string
  secretName?: string
}

type RenderOptions = {
  context?: RenderContext
  fieldValues?: Record<string, any>
  resolveFieldValue?: (key: string, fieldValues: Record<string, any>) => string
}

export function renderPaapTemplateValue(value: string, options: RenderOptions = {}) {
  const context = options.context || {}
  const fieldValues = options.fieldValues || {}
  const resolveFieldValue = options.resolveFieldValue || ((key, values) => String(values[key] ?? '').trim())
  return renderBlocks(String(value || ''), { context, fieldValues, resolveFieldValue })
}

function renderBlocks(value: string, options: Required<RenderOptions>) {
  let rendered = value
    .replace(/\{\{\s*componentName\s*\}\}/g, options.context.componentName || '')
    .replace(/\{\{\s*configMapName\s*\}\}/g, options.context.configMapName || '')
    .replace(/\{\{\s*secretName\s*\}\}/g, options.context.secretName || '')

  rendered = renderForBlocks(rendered, options)
  rendered = renderIfBlocks(rendered, options)
  return renderScalarTokens(rendered, options)
}

function renderForBlocks(value: string, options: Required<RenderOptions>) {
  return value.replace(/\[\[paap:for\s+([^\]\s]+)\]\]([\s\S]*?)\[\[paap:end\s+\1\]\]/g, (_match, key, body) => {
    const rows = Array.isArray(options.fieldValues[key]) ? options.fieldValues[key] : []
    return rows.map((row:any) => renderItemBody(String(body), row, options)).join('')
  })
}

function renderItemBody(body: string, row: Record<string, any>, options: Required<RenderOptions>) {
  const withItems = body.replace(/\[\[paap:item\.([^\]\s]+)([^\]]*)\]\]/g, (_match, key, tokenOptions) => {
    const value = String(row[key] ?? '').trim()
    return value || templatePlaceholderDefault(String(key), String(tokenOptions || ''))
  })
  return renderBlocks(withItems, {
    ...options,
    fieldValues: { ...options.fieldValues, ...row },
  })
}

function renderIfBlocks(value: string, options: Required<RenderOptions>) {
  return value.replace(/\[\[paap:if\s+([^\]\s]+)\]\]([\s\S]*?)\[\[paap:end\s+\1\]\]/g, (_match, key, body) => {
    return truthy(options.fieldValues[key]) ? renderBlocks(String(body), options) : ''
  })
}

function renderScalarTokens(value: string, options: Required<RenderOptions>) {
  return value.replace(/\[\[\s*paap:([^\]\s]+)([^\]]*)\]\]/g, (_match, key, tokenOptions) => {
    if (String(key).startsWith('for') || String(key).startsWith('end') || String(key).startsWith('if')) return ''
    const rendered = options.resolveFieldValue(String(key), options.fieldValues)
    return rendered || templatePlaceholderDefault(String(key), String(tokenOptions || ''))
  })
}

export function templatePlaceholderDefault(key: string, options: string) {
  const defaultMatch = options.match(/\bdefault=("[^"]*"|'[^']*'|[^\s\]]+)/)
  if (defaultMatch) return defaultMatch[1].replace(/^['"]|['"]$/g, '')
  if (key === 'listen.port') return '80'
  if (key.endsWith('.port')) return ''
  return ''
}

function truthy(value: any) {
  if (typeof value === 'boolean') return value
  const text = String(value ?? '').trim().toLowerCase()
  return ['1', 'true', 'yes', 'y', 'on', 'enabled'].includes(text)
}
