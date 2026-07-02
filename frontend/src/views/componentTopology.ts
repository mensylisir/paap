export type ComponentTopologyComponent = {
  id?: string | number
  name?: string
  type?: string
  serviceName?: string
  serviceType?: string
  namespace?: string
  status?: string
  config?: unknown
  dependencies?: unknown
  dependsOn?: unknown
  dependencyNames?: unknown
  dependencyComponents?: unknown
  topologyId?: string
  topologyKind?: 'component' | 'service' | 'capability'
  source?: string
  serviceId?: string | number
  componentId?: string | number
  externalUrl?: string
  runtimeServiceName?: string
  runtimeServiceType?: string
  clusterIP?: string
  loadBalancerIP?: string
  displayName?: string
}

export type ComponentTopologyEdge = {
  from: string
  to: string
  fromId: number
  toId: number
  fromKey?: string
  toKey?: string
  source?: 'auto' | 'manual'
}

export type ComponentTopologyLane = {
  key: string
  label: string
  nodes: ComponentTopologyComponent[]
}

export type ComponentTopologyZone = {
  key: 'environment' | 'shared' | 'external'
  label: string
  nodes: ComponentTopologyComponent[]
}

export type ComponentTopologyPosition = {
  x: number
  y: number
  width?: number
  height?: number
}

export type ComponentTopologyPositions = Record<string, ComponentTopologyPosition>

export type ComponentTopologyManualEdge = {
  fromKey: string
  toKey: string
}

export type ComponentTopologyDragPoint = {
  startX: number
  startY: number
  currentX: number
  currentY: number
}

export type ComponentTopologyDragPositionInput = ComponentTopologyDragPoint & {
  originX: number
  originY: number
  minX?: number
  minY?: number
  maxX?: number
  maxY?: number
  zoom?: number
}

export type ComponentTopologyClickSuppressState = {
  suppressNext: boolean
  suppressKey?: string
  recentDragKey?: string
  recentDragAt?: number
  now?: number
  windowMs?: number
}

export type ComponentTopologyEdgeNode = {
  x: number
  y: number
  width: number
  height: number
  topologyId?: string
  id?: string | number
  name?: string
}

export type ComponentTopologyEdgePathInput = {
  fromNode?: ComponentTopologyEdgeNode | null
  toNode?: ComponentTopologyEdgeNode | null
}

export type ComponentTopologyCanvasSize = {
  width: number
  height: number
}

export type ComponentTopologyBounds = {
  left: number
  top: number
  width: number
  height: number
}

export type ComponentTopologyZonePadding = {
  paddingX: number
  paddingTop: number
  paddingBottom: number
  minLeft?: number
  minTop?: number
  minWidth?: number
  minHeight?: number
}

export type ComponentTopologyZoneResizeInput = {
  originBounds: ComponentTopologyBounds
  contentBounds?: { left: number; top: number; right: number; bottom: number } | null
  edges: Array<'left' | 'right' | 'top' | 'bottom'>
  dx: number
  dy: number
  minLeft?: number
  minTop?: number
  minWidth?: number
  minHeight?: number
}

export type ComponentTopologyDisplayNames = Record<string, string>

const laneLabels: Record<string, string> = {
  frontend: '入口层',
  backend: '服务层',
  data: '数据/中间件',
  tools: '平台工具',
  other: '其他组件',
}

const laneOrder = ['frontend', 'backend', 'data', 'tools', 'other']
const zoneLabels: Record<ComponentTopologyZone['key'], string> = {
  environment: '本环境',
  shared: '平台公共',
  external: '集群外部',
}
const zoneOrder: ComponentTopologyZone['key'][] = ['environment', 'shared', 'external']

const dataServiceTypes = new Set(['postgresql', 'postgres', 'mysql', 'mongodb', 'mongo', 'redis', 'rabbitmq', 'kafka', 'minio', 'database', 'middleware', 'infra'])
const toolServiceTypes = new Set(['git', 'gitea', 'ci', 'jenkins', 'deploy', 'argocd', 'monitor', 'log', 'loki', 'registry', 'harbor', 'tool'])

export const componentLaneKey = (comp: ComponentTopologyComponent) => {
  const type = String(comp?.type || '').toLowerCase()
  if (comp?.topologyKind === 'service') {
    if (dataServiceTypes.has(type)) return 'data'
    if (toolServiceTypes.has(type)) return 'tools'
  }
  if (type === 'frontend') return 'frontend'
  if (type === 'backend') return 'backend'
  if (type === 'database' || type === 'middleware') return 'data'
  return 'other'
}

const stableNodeId = (kind: 'component' | 'service', node: { id?: string | number; name?: string }) => {
  const id = String(node?.id || '').trim()
  if (id) return `${kind}:${id}`
  const name = String(node?.name || '').trim()
  return `${kind}:${name.toLowerCase()}`
}

export const buildComponentTopologyNodes = (
  components: ComponentTopologyComponent[],
  services: ComponentTopologyComponent[] = [],
  displayNames?: ComponentTopologyDisplayNames
): ComponentTopologyComponent[] => {
  const applyDisplayName = (node: ComponentTopologyComponent): ComponentTopologyComponent => {
    if (!displayNames) return node
    const key = String(node.topologyId || '')
    const override = key ? String(displayNames[key] || '').trim() : ''
    if (!override) return node
    return { ...node, displayName: override }
  }
  const componentNodes = components.map((comp) => applyDisplayName({
    ...comp,
    topologyKind: 'component' as const,
    topologyId: stableNodeId('component', comp),
    componentId: comp.id,
  }))
  const serviceNodes = services.map((svc) => {
    const normalized = {
      ...svc,
      name: svc.name || svc.serviceName || svc.serviceType,
      type: svc.type || svc.serviceType,
    }
    return applyDisplayName({
      ...normalized,
      topologyKind: 'service' as const,
      topologyId: stableNodeId('service', normalized),
      serviceId: svc.id,
      status: svc.status || 'running',
    })
  })
  return [...componentNodes, ...serviceNodes]
}

export const buildComponentTopologyLanes = (components: ComponentTopologyComponent[]): ComponentTopologyLane[] =>
  laneOrder
    .map((key) => ({
      key,
      label: laneLabels[key],
      nodes: components.filter((comp) => componentLaneKey(comp) === key),
    }))
    .filter((lane) => lane.nodes.length > 0)

export const componentTopologyZoneKey = (node: ComponentTopologyComponent): ComponentTopologyZone['key'] => {
  if (node?.topologyKind === 'capability') {
    if (String(node.source || '').toLowerCase() === 'shared') return 'shared'
    if (String(node.source || '').toLowerCase() === 'external') return 'external'
  }
  return 'environment'
}

export const buildComponentTopologyZones = (nodes: ComponentTopologyComponent[]): ComponentTopologyZone[] =>
  zoneOrder
    .map((key) => ({
      key,
      label: zoneLabels[key],
      nodes: nodes.filter((node) => componentTopologyZoneKey(node) === key),
    }))
    .filter((zone) => zone.nodes.length > 0)

export const serviceNetworkSummary = (node: ComponentTopologyComponent) => {
  const runtimeServiceName = String(node?.runtimeServiceName || '').trim()
  const clusterIP = String(node?.clusterIP || '').trim()
  const loadBalancerIP = String(node?.loadBalancerIP || '').trim()
  return [
    runtimeServiceName,
    clusterIP ? `集群内 ${clusterIP}` : '',
    loadBalancerIP ? `负载均衡 ${loadBalancerIP}` : '',
  ].filter(Boolean).join(' · ')
}

export const explicitComponentDependencies = (comp: ComponentTopologyComponent) => {
  const config = parseComponentConfig(comp?.config)
  const bindings = Array.isArray(config?.bindings) ? config.bindings : []
  // Prefer specific targetKey over generic targetName to avoid ambiguous bare-name lookups
  const bindingTargets = bindings
    .map((item: any) => String((item?.targetKey || item?.targetName || '')).trim())
    .filter(Boolean)
  const raw = comp?.dependencies
    ?? comp?.dependsOn
    ?? comp?.dependencyNames
    ?? comp?.dependencyComponents
    ?? config?.dependencies
    ?? config?.dependsOn
    ?? config?.dependencyNames
    ?? config?.dependencyComponents
  if (Array.isArray(raw)) {
    const rawItems = raw
      .map((item) => String(typeof item === 'object' && item ? (item as any).name || (item as any).service || (item as any).component || (item as any).id : item))
      .map((item) => item.trim())
      .filter(Boolean)
    // Deduplicate: if a targetKey (e.g. capability:17) is already emitted from bindings,
    // skip the corresponding bare targetName (e.g. postgresql) from raw to avoid ambiguous resolution
    const nameToKey = new Map<string, string>()
    for (const b of bindings) {
      const n = String(b?.targetName || '').trim().toLowerCase()
      const k = String(b?.targetKey || '').trim()
      if (n && k) nameToKey.set(n, k)
    }
    const combined = [...bindingTargets, ...rawItems]
    const result: string[] = []
    const seen = new Set<string>()
    for (const item of combined) {
      if (seen.has(item)) continue
      if (!item.includes(':') && nameToKey.has(item.toLowerCase())) continue
      seen.add(item)
      result.push(item)
    }
    return result
  }
  if (typeof raw === 'string') {
    const rawItems = raw.split(',').map((item) => item.trim()).filter(Boolean)
    return [...new Set([...bindingTargets, ...rawItems])]
  }
  return [...new Set(bindingTargets)]
}

const parseComponentConfig = (config: unknown): any => {
  if (!config) return null
  if (typeof config === 'object') return config
  if (typeof config !== 'string') return null
  const trimmed = config.trim()
  if (!trimmed || trimmed === '{}') return null
  try {
    return JSON.parse(trimmed)
  } catch {
    return null
  }
}

export const buildComponentDependencyEdges = (components: ComponentTopologyComponent[]): ComponentTopologyEdge[] => {
  const byName = new Map<string, ComponentTopologyComponent>()
  const byId = new Map<string, ComponentTopologyComponent>()
  const byTopologyId = new Map<string, ComponentTopologyComponent>()
  const nameCount = new Map<string, number>()
  for (const comp of components) {
    const name = String(comp.name || '').trim()
    const id = String(comp.id || '').trim()
    const topologyId = String(comp.topologyId || '').trim()
    if (name) {
      const key = name.toLowerCase()
      byName.set(key, comp)
      nameCount.set(key, (nameCount.get(key) || 0) + 1)
    }
    if (id) byId.set(id, comp)
    if (topologyId) byTopologyId.set(topologyId.toLowerCase(), comp)
  }

  const edges: ComponentTopologyEdge[] = []
  const seen = new Set<string>()
  const addEdge = (from: ComponentTopologyComponent, to: ComponentTopologyComponent) => {
    const fromId = Number(from.id)
    const toId = Number(to.id)
    if (!Number.isFinite(fromId) || !Number.isFinite(toId) || fromId === toId) return
    const fromKey = String(from.topologyId || fromId)
    const toKey = String(to.topologyId || toId)
    const key = `${fromKey}:${toKey}`
    if (seen.has(key)) return
    seen.add(key)
    const edge: ComponentTopologyEdge = {
      from: String(from.name || fromId),
      to: String(to.name || toId),
      fromId,
      toId,
    }
    if (from.topologyId || to.topologyId) {
      edge.fromKey = fromKey
      edge.toKey = toKey
    }
    edges.push(edge)
  }

  // Potential targets include all nodes (services, capabilities, and other components)
  const allTargets = components

  for (const comp of components) {
    if (comp.topologyKind === 'service') continue
    for (const dependency of explicitComponentDependencies(comp)) {
      const depLower = dependency.toLowerCase()
      // For bare names (no ':'), skip if 2+ nodes share the same name (ambiguous).
      // The specific targetKey (e.g. "capability:17") will still resolve correctly
      // via byTopologyId in the loop body below.
      if (!depLower.includes(':') && (nameCount.get(depLower) || 0) > 1) continue
      const target = byTopologyId.get(depLower) || byName.get(depLower) || byId.get(dependency)
      if (target) addEdge(comp, target)
    }
    const config = parseComponentConfig(comp?.config)
    const env = Array.isArray(config?.env) ? config.env : []
    const envVars = env
      .filter((e: any) => e?.value)
      .map((e: any) => ({ name: String(e.name || '').trim(), value: String(e.value || '').trim() }))
      .filter((e: { name: string; value: string }) => e.name || e.value)

    // Also extract FQDN references from configMap data for heuristic matching
    const configMapRefs: Array<{ name: string; value: string }> = []
    if (Array.isArray(config?.configMaps)) {
      for (const cm of config.configMaps) {
        if (!cm?.data || typeof cm.data !== 'object') continue
        for (const val of Object.values(cm.data) as string[]) {
          if (typeof val !== 'string') continue
          // Match <service>.<namespace>.svc.cluster.local patterns
          const fqdnRe = /([a-z][a-z0-9-]*)\.([a-z][a-z0-9-]*)\.svc\.cluster\.local/gi
          let m: RegExpExecArray | null
          while ((m = fqdnRe.exec(val)) !== null) {
            const svcName = m[1].toLowerCase()
            configMapRefs.push({ name: svcName + '_SERVICE_HOST', value: m[0] })
          }
        }
      }
    }

    const allVars = [...envVars, ...configMapRefs]
    if (allVars.length) {
      const derived = deriveBindingsFromEnv(allVars, allTargets)
      for (const binding of derived) {
        const target = byName.get(binding.targetName.toLowerCase()) || byId.get(binding.targetKey)
        if (target) addEdge(comp, target)
      }
    }
  }

  return edges
}

const escapeRegex = (text: string): string => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

const connectionEnvPrefix = (serviceType: string): string => {
  const normalized = String(serviceType || 'SERVICE').toUpperCase().replace(/[^A-Z0-9]+/g, '_')
  return normalized === 'POSTGRESQL' ? 'POSTGRES' : normalized
}

export interface DerivedBinding {
  targetKey: string
  targetName: string
  targetType: string
  targetKind: string
  confidence: string
}

/**
 * Derive service bindings from environment variables using structured heuristics.
 * Only returns a binding when exactly ONE target matches (avoids ambiguity).
 */
export const deriveBindingsFromEnv = (
  env: Array<{ name: string; value: string }>,
  targets: Array<{ id?: string | number; name?: string; type?: string; namespace?: string; topologyKind?: string }>
): DerivedBinding[] => {
  const results: DerivedBinding[] = []
  const matchedKeys = new Set<string>()

  if (!Array.isArray(env) || !Array.isArray(targets)) return results

  for (const envVar of env) {
    const name = String(envVar.name || '').trim()
    const value = String(envVar.value || '').trim()
    if (!name && !value) continue
    const nameLower = name.toLowerCase()
    const valueLower = value.toLowerCase()

    const matchingTargets: Array<{ target: typeof targets[0]; confidence: string }> = []

    for (const target of targets) {
      const targetName = String(target.name || '').trim().toLowerCase()
      const targetType = String(target.type || '').trim().toLowerCase()
      const namespace = String(target.namespace || '').trim().toLowerCase()
      if (!targetName) continue

      // Heuristic 1: FQDN in value — e.g. postgresql.default.svc.cluster.local
      if (namespace) {
        const fqdn = `${targetName}.${namespace}.svc.cluster.local`
        const shortFqdn = `${targetName}.${namespace}`
        if (valueLower.includes(fqdn) || valueLower.includes(shortFqdn)) {
          matchingTargets.push({ target, confidence: 'high' })
          continue
        }
      }

      // Heuristic 2: env name matches {PREFIX}_HOST / _PORT / _URL convention
      // e.g. POSTGRES_HOST, REDIS_PORT, MONGODB_URL
      const prefix = connectionEnvPrefix(targetType).toLowerCase()
      if (nameLower === `${prefix}_host` || nameLower === `${prefix}_port` || nameLower === `${prefix}_url`) {
        matchingTargets.push({ target, confidence: 'medium' })
        continue
      }

      // Heuristic 3: env name matches {service_name}_SERVICE_HOST (K8s convention)
      // or {service_name}_HOST
      if (nameLower === `${targetName}_service_host` || nameLower === `${targetName}_host`) {
        matchingTargets.push({ target, confidence: 'medium' })
        continue
      }

      // Heuristic 4: URL hostname in value — ://serviceName: or ://serviceName/ or ://serviceName.
      // Catches jdbc:postgresql://postgresql:5432/db without matching jdbc:postgresql:// protocol scheme
      const urlPattern = new RegExp(`://${escapeRegex(targetName)}(:|/|\\.|$)`)
      if (urlPattern.test(valueLower)) {
        matchingTargets.push({ target, confidence: 'medium' })
        continue
      }

      // Heuristic 5: host:port pattern — serviceName:portnumber
      // Avoids matching protocol schemes like jdbc:postgresql: (colon followed by /, not digit)
      const hostPortPattern = new RegExp(`(^|[^a-z0-9.])${escapeRegex(targetName)}:(\\d+)($|[^a-z0-9/])`)
      if (hostPortPattern.test(valueLower)) {
        matchingTargets.push({ target, confidence: 'medium' })
        continue
      }
    }

    // Only create binding when exactly one target matches (avoids ambiguity)
    if (matchingTargets.length === 1) {
      const { target, confidence } = matchingTargets[0]
      const targetName = String(target.name || '').trim()
      const targetKey = String(target.id || targetName)
      if (!matchedKeys.has(targetKey)) {
        matchedKeys.add(targetKey)
        results.push({
          targetKey,
          targetName,
          targetType: String(target.type || '').trim(),
          targetKind: target.topologyKind === 'capability' ? 'capability' : 'service',
          confidence,
        })
      }
    }
  }

  return results
}

export const parseComponentTopologyPositions = (raw: string | null | undefined): ComponentTopologyPositions => {
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const positions: ComponentTopologyPositions = {}
    for (const [key, value] of Object.entries(parsed)) {
      if (!key || !value || typeof value !== 'object') continue
      const x = Number((value as any).x)
      const y = Number((value as any).y)
      if (!Number.isFinite(x) || !Number.isFinite(y)) continue
      const width = Number((value as any).width)
      const height = Number((value as any).height)
      positions[key] = {
        x,
        y,
        ...(Number.isFinite(width) && width > 0 ? { width } : {}),
        ...(Number.isFinite(height) && height > 0 ? { height } : {}),
      }
    }
    return positions
  } catch {
    return {}
  }
}

export const serializeComponentTopologyPositions = (positions: ComponentTopologyPositions): string => {
  const serializable: ComponentTopologyPositions = {}
  for (const [key, value] of Object.entries(positions || {})) {
    const x = Number(value?.x)
    const y = Number(value?.y)
    if (!key || !Number.isFinite(x) || !Number.isFinite(y)) continue
    const width = Number(value?.width)
    const height = Number(value?.height)
    serializable[key] = {
      x,
      y,
      ...(Number.isFinite(width) && width > 0 ? { width } : {}),
      ...(Number.isFinite(height) && height > 0 ? { height } : {}),
    }
  }
  return JSON.stringify(serializable)
}

export const parseComponentTopologyDisplayNames = (raw: string | null | undefined): ComponentTopologyDisplayNames => {
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const names: ComponentTopologyDisplayNames = {}
    for (const [key, value] of Object.entries(parsed)) {
      const trimmedKey = String(key || '').trim()
      const trimmedValue = String(value || '').trim()
      if (!trimmedKey || !trimmedValue) continue
      names[trimmedKey] = trimmedValue
    }
    return names
  } catch {
    return {}
  }
}

export const serializeComponentTopologyDisplayNames = (names: ComponentTopologyDisplayNames): string => {
  const serializable: ComponentTopologyDisplayNames = {}
  for (const [key, value] of Object.entries(names || {})) {
    const trimmedKey = String(key || '').trim()
    const trimmedValue = String(value || '').trim()
    if (!trimmedKey || !trimmedValue) continue
    serializable[trimmedKey] = trimmedValue
  }
  return JSON.stringify(serializable)
}

export const componentTopologyCanvasViewBox = (size: ComponentTopologyCanvasSize): string => {
  const width = Number.isFinite(Number(size?.width)) && Number(size.width) > 0 ? Number(size.width) : 0
  const height = Number.isFinite(Number(size?.height)) && Number(size.height) > 0 ? Number(size.height) : 0
  return `0 0 ${width} ${height}`
}

export const parseComponentTopologyManualEdges = (raw: string | null | undefined): ComponentTopologyManualEdge[] => {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    const seen = new Set<string>()
    const edges: ComponentTopologyManualEdge[] = []
    for (const item of parsed) {
      if (!item || typeof item !== 'object') continue
      const fromKey = String((item as any).fromKey || '').trim()
      const toKey = String((item as any).toKey || '').trim()
      if (!fromKey || !toKey || fromKey === toKey) continue
      const key = `${fromKey}->${toKey}`
      if (seen.has(key)) continue
      seen.add(key)
      edges.push({ fromKey, toKey })
    }
    return edges
  } catch {
    return []
  }
}

export const serializeComponentTopologyManualEdges = (edges: ComponentTopologyManualEdge[]): string => {
  const seen = new Set<string>()
  const serializable: ComponentTopologyManualEdge[] = []
  for (const item of edges || []) {
    const fromKey = String(item?.fromKey || '').trim()
    const toKey = String(item?.toKey || '').trim()
    if (!fromKey || !toKey || fromKey === toKey) continue
    const key = `${fromKey}->${toKey}`
    if (seen.has(key)) continue
    seen.add(key)
    serializable.push({ fromKey, toKey })
  }
  return JSON.stringify(serializable)
}

export const removeComponentTopologyManualEdge = (
  edges: ComponentTopologyManualEdge[],
  fromKey: string,
  toKey: string
): ComponentTopologyManualEdge[] => {
  const from = String(fromKey || '').trim()
  const to = String(toKey || '').trim()
  if (!from || !to) return [...(edges || [])]
  return (edges || []).filter((edge) => !(edge.fromKey === from && edge.toKey === to))
}

export const hasComponentTopologyDragMoved = (point: ComponentTopologyDragPoint, threshold = 4): boolean => {
  const dx = Math.abs(Number(point.currentX) - Number(point.startX))
  const dy = Math.abs(Number(point.currentY) - Number(point.startY))
  return dx > threshold || dy > threshold
}

export const shouldSuppressComponentTopologyClick = (key: string, state: ComponentTopologyClickSuppressState): boolean => {
  const normalizedKey = String(key || '')
  if (state.suppressNext && (!state.suppressKey || state.suppressKey === normalizedKey)) return true
  if (!state.recentDragKey || state.recentDragKey !== normalizedKey) return false
  const now = Number.isFinite(state.now) ? Number(state.now) : Date.now()
  const recentDragAt = Number(state.recentDragAt || 0)
  const windowMs = Number.isFinite(state.windowMs) ? Number(state.windowMs) : 350
  return recentDragAt > 0 && now - recentDragAt <= windowMs
}

export const nextComponentTopologyDragPosition = (input: ComponentTopologyDragPositionInput): ComponentTopologyPosition => {
  const minX = Number.isFinite(input.minX) ? Number(input.minX) : 12
  const minY = Number.isFinite(input.minY) ? Number(input.minY) : 46
  const maxX = Number.isFinite(input.maxX) ? Number(input.maxX) : Number.POSITIVE_INFINITY
  const maxY = Number.isFinite(input.maxY) ? Number(input.maxY) : Number.POSITIVE_INFINITY
  const zoom = Number.isFinite(input.zoom) && Number(input.zoom) > 0 ? Number(input.zoom) : 1
  const nextX = Number(input.originX) + (Number(input.currentX) - Number(input.startX)) / zoom
  const nextY = Number(input.originY) + (Number(input.currentY) - Number(input.startY)) / zoom
  return {
    x: Math.min(Math.max(minX, nextX), maxX),
    y: Math.min(Math.max(minY, nextY), maxY),
  }
}

export const componentTopologyContentBounds = (
  nodes: ComponentTopologyEdgeNode[],
  padding: ComponentTopologyZonePadding
): ComponentTopologyBounds | null => {
  if (!nodes.length) return null
  const paddingX = Number(padding.paddingX || 0)
  const paddingTop = Number(padding.paddingTop || 0)
  const paddingBottom = Number(padding.paddingBottom || 0)
  const minLeft = Number.isFinite(padding.minLeft) ? Number(padding.minLeft) : 0
  const minTop = Number.isFinite(padding.minTop) ? Number(padding.minTop) : 0
  const left = Math.max(minLeft, Math.min(...nodes.map((node) => Number(node.x || 0))) - paddingX)
  const top = Math.max(minTop, Math.min(...nodes.map((node) => Number(node.y || 0))) - paddingTop)
  const right = Math.max(...nodes.map((node) => Number(node.x || 0) + Number(node.width || 0))) + paddingX
  const bottom = Math.max(...nodes.map((node) => Number(node.y || 0) + Number(node.height || 0))) + paddingBottom
  const minWidth = Number.isFinite(padding.minWidth) ? Number(padding.minWidth) : 0
  const minHeight = Number.isFinite(padding.minHeight) ? Number(padding.minHeight) : 0
  return {
    left,
    top,
    width: Math.max(minWidth, right - left),
    height: Math.max(minHeight, bottom - top),
  }
}

export const expandComponentTopologyZoneBounds = (
  bounds: ComponentTopologyBounds,
  nodes: ComponentTopologyEdgeNode[],
  padding: ComponentTopologyZonePadding
): ComponentTopologyBounds => {
  const content = componentTopologyContentBounds(nodes, padding)
  if (!content) return bounds
  const minLeft = Number.isFinite(padding.minLeft) ? Number(padding.minLeft) : 0
  const minTop = Number.isFinite(padding.minTop) ? Number(padding.minTop) : 0
  const left = Math.max(minLeft, Math.min(Number(bounds.left || 0), content.left))
  const top = Math.max(minTop, Math.min(Number(bounds.top || 0), content.top))
  const right = Math.max(Number(bounds.left || 0) + Number(bounds.width || 0), content.left + content.width)
  const bottom = Math.max(Number(bounds.top || 0) + Number(bounds.height || 0), content.top + content.height)
  return {
    left,
    top,
    width: Math.max(Number(padding.minWidth || 0), right - left),
    height: Math.max(Number(padding.minHeight || 0), bottom - top),
  }
}

export const componentTopologyCanvasSizeWithSavedBounds = (
  base: ComponentTopologyCanvasSize,
  savedBounds: { right?: number; bottom?: number },
  edgePadding = 48
): ComponentTopologyCanvasSize => {
  const padding = Number.isFinite(edgePadding) ? Math.max(0, Number(edgePadding)) : 0
  const savedRight = Number(savedBounds.right || 0)
  const savedBottom = Number(savedBounds.bottom || 0)
  return {
    width: Math.max(Number(base.width || 0), savedRight > 0 ? savedRight + padding : 0),
    height: Math.max(Number(base.height || 0), savedBottom > 0 ? savedBottom + padding : 0),
  }
}

export const nextComponentTopologyZoneResizeBounds = (input: ComponentTopologyZoneResizeInput): ComponentTopologyBounds => {
  const origin = input.originBounds
  const edges = input.edges || []
  const content = input.contentBounds || null
  const minLeft = Number.isFinite(input.minLeft) ? Number(input.minLeft) : 16
  const minTop = Number.isFinite(input.minTop) ? Number(input.minTop) : 16
  const minWidth = content ? Number(content.right) - Number(content.left) : Number(input.minWidth || 0)
  const minHeight = content ? Number(content.bottom) - Number(content.top) : Number(input.minHeight || 0)
  let left = Number(origin.left || 0)
  let top = Number(origin.top || 0)
  let right = Number(origin.left || 0) + Number(origin.width || 0)
  let bottom = Number(origin.top || 0) + Number(origin.height || 0)
  if (edges.includes('left')) left = Math.min(Math.max(minLeft, left + Number(input.dx || 0)), content?.left ?? right - minWidth)
  if (edges.includes('right')) right = Math.max(right + Number(input.dx || 0), content?.right ?? left + minWidth)
  if (edges.includes('top')) top = Math.min(Math.max(minTop, top + Number(input.dy || 0)), content?.top ?? bottom - minHeight)
  if (edges.includes('bottom')) bottom = Math.max(bottom + Number(input.dy || 0), content?.bottom ?? top + minHeight)
  if (right - left < minWidth) {
    if (edges.includes('left')) left = right - minWidth
    else right = left + minWidth
  }
  if (bottom - top < minHeight) {
    if (edges.includes('top')) top = bottom - minHeight
    else bottom = top + minHeight
  }
  const normalizedLeft = Math.max(minLeft, left)
  const normalizedTop = Math.max(minTop, top)
  return {
    left: normalizedLeft,
    top: normalizedTop,
    width: Math.max(minWidth, right - normalizedLeft),
    height: Math.max(minHeight, bottom - normalizedTop),
  }
}

export const componentTopologyUnionBounds = (
  saved: ComponentTopologyBounds,
  content: ComponentTopologyBounds
): ComponentTopologyBounds => {
  const left = Math.min(Number(saved.left || 0), Number(content.left || 0))
  const top = Math.min(Number(saved.top || 0), Number(content.top || 0))
  const right = Math.max(Number(saved.left || 0) + Number(saved.width || 0), Number(content.left || 0) + Number(content.width || 0))
  const bottom = Math.max(Number(saved.top || 0) + Number(saved.height || 0), Number(content.top || 0) + Number(content.height || 0))
  return { left, top, width: right - left, height: bottom - top }
}

export const componentTopologyEdgePath = (edge: ComponentTopologyEdgePathInput): string => {
  const from = edge.fromNode
  const to = edge.toNode
  if (!from || !to) return ''

  const fromCenterX = Number(from.x) + Number(from.width) / 2
  const fromCenterY = Number(from.y) + Number(from.height) / 2
  const toCenterX = Number(to.x) + Number(to.width) / 2
  const toCenterY = Number(to.y) + Number(to.height) / 2
  const dx = toCenterX - fromCenterX
  const dy = toCenterY - fromCenterY

  if (Math.abs(dx) >= Math.abs(dy)) {
    const startX = dx >= 0 ? Number(from.x) + Number(from.width) : Number(from.x)
    const startY = fromCenterY
    const endX = dx >= 0 ? Number(to.x) : Number(to.x) + Number(to.width)
    const endY = toCenterY
    const elbowX = Math.round((startX + endX) / 2)
    return `M ${startX} ${startY} H ${elbowX} V ${endY} H ${endX}`
  }

  const startX = fromCenterX
  const startY = dy >= 0 ? Number(from.y) + Number(from.height) : Number(from.y)
  const endX = toCenterX
  const endY = dy >= 0 ? Number(to.y) : Number(to.y) + Number(to.height)
  const elbowY = Math.round((startY + endY) / 2)
  return `M ${startX} ${startY} V ${elbowY} H ${endX} V ${endY}`
}

export type ComponentTopologyMarqueeRect = {
  x: number
  y: number
  width: number
  height: number
}

export const isNodeInMarquee = (node: ComponentTopologyEdgeNode, rect: ComponentTopologyMarqueeRect): boolean => {
  return (
    node.x < rect.x + rect.width &&
    node.x + node.width > rect.x &&
    node.y < rect.y + rect.height &&
    node.y + node.height > rect.y
  )
}

export const nodeKey = (node: { topologyId?: string; id?: string | number; name?: string } | null | undefined): string => {
  return String(node?.topologyId || node?.id || node?.name || '')
}

export const findTopologyNodeAtPoint = <T extends ComponentTopologyEdgeNode>(
  nodes: T[],
  point: { x: number; y: number },
  excludeKey = ''
): T | null => {
  const x = Number(point?.x)
  const y = Number(point?.y)
  if (!Number.isFinite(x) || !Number.isFinite(y)) return null
  const excluded = String(excludeKey || '')
  for (let i = nodes.length - 1; i >= 0; i--) {
    const node = nodes[i]
    const key = nodeKey(node)
    if (excluded && key === excluded) continue
    if (
      x >= Number(node.x) &&
      x <= Number(node.x) + Number(node.width) &&
      y >= Number(node.y) &&
      y <= Number(node.y) + Number(node.height)
    ) {
      return node
    }
  }
  return null
}
