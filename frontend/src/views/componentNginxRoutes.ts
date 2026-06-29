export type NginxRouteRow = {
  path: string
  targetKey: string
  targetUrl: string
}

type ComponentBinding = Record<string, any>
type BackendTarget = Record<string, any>
type TemplateListField = Record<string, any>
type ComponentConfigObject = {
  name?: string
  data?: Record<string, any>
}

const stringValue = (value:any) => String(value ?? '').trim()

const normalizeProxyUrl = (value:any) => stringValue(value)
  .replace(/;$/, '')
  .replace(/\/+$/, '')

const normalizeLocationPath = (value:any) => {
  const raw = stringValue(value).replace(/^['"]|['"]$/g, '')
  if (!raw || raw === '/') return ''
  return raw.startsWith('/') ? raw : `/${raw}`
}

const backendUrl = (target:any) => normalizeProxyUrl(`http://${target?.serviceName || target?.name || 'backend'}`)

const proxyHost = (value:any) => {
  const raw = normalizeProxyUrl(value)
  if (!raw) return ''
  try {
    return new URL(raw).host.toLowerCase()
  } catch {
    return raw.replace(/^https?:\/\//, '').split('/')[0].toLowerCase()
  }
}

const proxyHostBase = (value:any) => proxyHost(value).replace(/:\d+$/, '')

const generatedProxyUrl = (generated:Record<string, any>, fallbackTarget?:BackendTarget) => {
  const preferred = [
    generated.proxyPass,
    generated.PROXY_PASS,
    generated.BACKEND_URL,
    generated.BACKEND_1_URL,
  ].map(normalizeProxyUrl).find(Boolean)
  if (preferred) return preferred
  const fromGenerated = Object.values(generated || {})
    .map(normalizeProxyUrl)
    .find((item) => /^https?:\/\//.test(item))
  if (fromGenerated) return fromGenerated
  return fallbackTarget ? backendUrl(fallbackTarget) : ''
}

const findTargetForBinding = (binding:ComponentBinding, backendTargets:BackendTarget[]) =>
  backendTargets.find((target) => stringValue(target.key) && stringValue(target.key) === stringValue(binding.targetKey))
  || backendTargets.find((target) => stringValue(target.name) && stringValue(target.name) === stringValue(binding.targetName))
  || null

const findTargetForProxyUrl = (targetUrl:string, backendTargets:BackendTarget[]) => {
  const normalized = normalizeProxyUrl(targetUrl)
  const host = proxyHost(normalized)
  const hostBase = proxyHostBase(normalized)
  return backendTargets.find((target) => backendUrl(target) === normalized)
    || backendTargets.find((target) => proxyHost(backendUrl(target)) === host)
    || backendTargets.find((target) => proxyHostBase(backendUrl(target)) === hostBase)
    || backendTargets.find((target) => [target.serviceName, target.name]
      .map((item) => stringValue(item).toLowerCase())
      .filter(Boolean)
      .includes(hostBase))
    || null
}

const findBindingForProxyUrl = (targetUrl:string, bindings:ComponentBinding[]) => {
  const normalized = normalizeProxyUrl(targetUrl)
  const hostBase = proxyHostBase(normalized)
  return bindings.find((binding) => {
    const generated = binding.generated || {}
    const generatedUrl = generatedProxyUrl(generated)
    if (generatedUrl && normalizeProxyUrl(generatedUrl) === normalized) return true
    return stringValue(binding.targetName).toLowerCase() === hostBase
  }) || null
}

const bindingMatches = (current:ComponentBinding, incoming:ComponentBinding) =>
  (current.targetKey && current.targetKey === incoming.targetKey)
  || (current.targetName === incoming.targetName && current.role === incoming.role)

export const mergeComponentBinding = (bindings:ComponentBinding[], binding:ComponentBinding) => {
  const idx = bindings.findIndex((item) => bindingMatches(item, binding))
  if (idx < 0) return [...bindings, binding]
  const current = bindings[idx] || {}
  const merged = {
    ...current,
    ...binding,
    generated: {
      ...(current.generated || {}),
      ...(binding.generated || {}),
    },
  }
  const next = [...bindings]
  next.splice(idx, 1, merged)
  return next
}

const fieldKey = (field:any) => stringValue(field?.key)

const fieldType = (field:any) => stringValue(field?.type).toLowerCase()

const fieldTargets = (field:any) => stringValue(field?.target)
  .toLowerCase()
  .split('|')
  .map((item) => item.trim())
  .filter(Boolean)

const templateRouteFieldKeys = (field:TemplateListField) => {
  const itemFields = Array.isArray(field?.itemFields) ? field.itemFields : []
  if (!itemFields.length || fieldType(field) !== 'list') return null
  const pathField = itemFields.find((item:any) => {
    const key = fieldKey(item).toUpperCase()
    return key === 'MATCH' || key === 'PATH' || key === 'API_PATH' || key.endsWith('_PATH') || key.includes('LOCATION')
  })
  const proxyField = itemFields.find((item:any) => {
    const key = fieldKey(item).toUpperCase()
    const targets = fieldTargets(item)
    return targets.includes('backend') || key.includes('PROXY_PASS') || key.includes('BACKEND') || key.includes('TARGET_URL')
  })
  const directivesField = itemFields.find((item:any) => fieldKey(item).toUpperCase().includes('DIRECTIVES'))
  const pathKey = fieldKey(pathField)
  const proxyKey = fieldKey(proxyField)
  const directivesKey = fieldKey(directivesField)
  return pathKey && (proxyKey || directivesKey) ? { pathKey, proxyKey, directivesKey } : null
}

export const nginxTemplateListFieldSupportsRoutes = (field:TemplateListField) =>
  Boolean(templateRouteFieldKeys(field))

export const nginxRouteRowsToTemplateListRows = (
  routes:NginxRouteRow[],
  field:TemplateListField,
) => {
  const keys = templateRouteFieldKeys(field)
  if (!keys) return []
  return (routes || []).map((route) => ({
    [keys.pathKey]: normalizeLocationPath(route.path),
    ...(keys.proxyKey
      ? { [keys.proxyKey]: stringValue(route.targetKey) || normalizeProxyUrl(route.targetUrl) }
      : {}),
    ...(keys.directivesKey
      ? { [keys.directivesKey]: [
        `proxy_pass ${normalizeProxyUrl(route.targetUrl) || 'http://backend:8080'};`,
        'proxy_http_version 1.1;',
        'proxy_set_header Host $host;',
        'proxy_set_header X-Real-IP $remote_addr;',
        'proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;',
        'proxy_set_header X-Forwarded-Proto $scheme;',
      ].join('\n') }
      : {}),
  })).filter((row) => row[keys.pathKey] || row[keys.proxyKey])
}

export const nginxTemplateListRowsToRouteRows = (
  rows:Array<Record<string, any>>,
  field:TemplateListField,
  backendTargets:BackendTarget[],
): NginxRouteRow[] => {
  const keys = templateRouteFieldKeys(field)
  if (!keys) return []
  return (rows || []).map((row) => {
    const directivesTarget = keys.directivesKey
      ? normalizeProxyUrl(/proxy_pass\s+([^;\s]+)\s*;/.exec(stringValue(row?.[keys.directivesKey]))?.[1])
      : ''
    const rawTarget = keys.proxyKey ? stringValue(row?.[keys.proxyKey]) : directivesTarget
    const target = backendTargets.find((item) => stringValue(item.key) === rawTarget)
      || findTargetForProxyUrl(rawTarget, backendTargets)
    return {
      path: normalizeLocationPath(row?.[keys.pathKey]),
      targetKey: stringValue(target?.key),
      targetUrl: target ? backendUrl(target) : normalizeProxyUrl(rawTarget),
    }
  }).filter((row) => row.path || row.targetUrl)
}

const parseNginxProxyRoutes = (content:string): Array<{ path:string; targetUrl:string }> => {
  const routes: Array<{ path:string; targetUrl:string }> = []
  const locationRe = /location\s+(?:=\s*|\^~\s*|~\*?\s*)?("[^"]+"|'[^']+'|[^\s{]+)\s*\{([\s\S]*?)\}/g
  let match: RegExpExecArray | null
  while ((match = locationRe.exec(content)) !== null) {
    const path = normalizeLocationPath(match[1])
    if (!path) continue
    const proxy = /proxy_pass\s+([^;\s]+)\s*;/.exec(match[2] || '')
    const targetUrl = normalizeProxyUrl(proxy?.[1])
    if (!targetUrl) continue
    routes.push({ path, targetUrl })
  }
  return routes
}

const nginxConfigTexts = (configMaps:ComponentConfigObject[]) =>
  (configMaps || []).flatMap((item) =>
    Object.entries(item?.data || {})
      .filter(([key, value]) => key.endsWith('.conf') || stringValue(value).includes('proxy_pass'))
      .map(([, value]) => String(value ?? ''))
  )

export const nginxRouteRowsFromComponentConfig = ({
  bindings,
  configMaps,
  backendTargets,
}: {
  bindings: ComponentBinding[]
  configMaps: ComponentConfigObject[]
  backendTargets: BackendTarget[]
}): NginxRouteRow[] => {
  const backendBindings = (bindings || []).filter((binding) =>
    binding.role === 'backend' || stringValue(binding.targetType).toLowerCase() === 'backend'
  )
  const rows = new Map<string, NginxRouteRow>()
  const addRow = (row:NginxRouteRow) => {
    const path = normalizeLocationPath(row.path)
    const targetUrl = normalizeProxyUrl(row.targetUrl)
    if (!path || !targetUrl) return
    rows.set(`${path}|${targetUrl}`, { path, targetKey: stringValue(row.targetKey), targetUrl })
  }

  backendBindings.forEach((binding) => {
    const generated = binding.generated || {}
    const target = findTargetForBinding(binding, backendTargets || [])
    addRow({
      path: generated.locationPath,
      targetKey: stringValue(binding.targetKey || target?.key),
      targetUrl: generatedProxyUrl(generated, target || undefined),
    })
  })

  for (const content of nginxConfigTexts(configMaps || [])) {
    for (const route of parseNginxProxyRoutes(content)) {
      const target = findTargetForProxyUrl(route.targetUrl, backendTargets || [])
      const binding = target ? null : findBindingForProxyUrl(route.targetUrl, backendBindings)
      addRow({
        path: route.path,
        targetKey: stringValue(target?.key || binding?.targetKey),
        targetUrl: route.targetUrl,
      })
    }
  }

  return Array.from(rows.values())
}
