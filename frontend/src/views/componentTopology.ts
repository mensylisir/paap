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
    return { ...node, name: override }
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
  const bindingTargets = Array.isArray(config?.bindings)
    ? config.bindings
      .flatMap((item: any) => [item?.targetKey, item?.targetName])
      .map((item: any) => String(item || '').trim())
      .filter(Boolean)
    : []
  const raw = comp?.dependencies
    ?? comp?.dependsOn
    ?? comp?.dependencyNames
    ?? comp?.dependencyComponents
    ?? config?.dependencies
    ?? config?.dependsOn
    ?? config?.dependencyNames
    ?? config?.dependencyComponents
  if (Array.isArray(raw)) {
    return [...bindingTargets, ...raw
      .map((item) => String(typeof item === 'object' && item ? (item as any).name || (item as any).service || (item as any).component || (item as any).id : item))
      .map((item) => item.trim())
      .filter(Boolean)]
  }
  if (typeof raw === 'string') return [...bindingTargets, ...raw.split(',').map((item) => item.trim()).filter(Boolean)]
  return bindingTargets
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
  const aliasTargets = new Map<string, ComponentTopologyComponent[]>()
  for (const comp of components) {
    const name = String(comp.name || '').trim()
    const id = String(comp.id || '').trim()
    const topologyId = String(comp.topologyId || '').trim()
    if (name) byName.set(name.toLowerCase(), comp)
    if (id) byId.set(id, comp)
    if (topologyId) byTopologyId.set(topologyId.toLowerCase(), comp)
    for (const alias of dependencyAliases(comp)) {
      const list = aliasTargets.get(alias) || []
      list.push(comp)
      aliasTargets.set(alias, list)
    }
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

  for (const comp of components) {
    if (comp.topologyKind === 'service') continue
    for (const dependency of explicitComponentDependencies(comp)) {
      const target = byTopologyId.get(dependency.toLowerCase()) || byName.get(dependency.toLowerCase()) || byId.get(dependency)
      if (target) addEdge(comp, target)
    }
    for (const target of inferredConfigDependencies(comp, aliasTargets)) {
      addEdge(comp, target)
    }
  }

  return edges
}

const dependencyAliases = (node: ComponentTopologyComponent): string[] => {
  const values = new Set<string>()
  const add = (value: unknown) => {
    const text = String(value || '').trim().toLowerCase()
    if (text.length >= 3) values.add(text)
  }
  add(node.topologyId)
  add(node.name)
  add(node.serviceName)
  add(node.serviceType)
  const serviceName = String(node.serviceName || node.name || node.serviceType || '').trim()
  const namespace = String(node.namespace || '').trim()
  if (serviceName && namespace) {
    add(`${serviceName}.${namespace}`)
    add(`${serviceName}.${namespace}.svc`)
    add(`${serviceName}.${namespace}.svc.cluster.local`)
  }
  return Array.from(values)
}

const inferredConfigDependencies = (
  comp: ComponentTopologyComponent,
  aliasTargets: Map<string, ComponentTopologyComponent[]>
): ComponentTopologyComponent[] => {
  const haystack = componentConfigSearchText(comp)
  if (!haystack) return []
  const matched = new Map<string, ComponentTopologyComponent>()
  for (const [alias, targets] of aliasTargets.entries()) {
    if (!haystack.includes(alias)) continue
    const uniqueTargets = targets.filter((target) => String(target.topologyId || target.id) !== String(comp.topologyId || comp.id))
    if (uniqueTargets.length !== 1) continue
    const target = uniqueTargets[0]
    if (isPlatformToolServiceNode(target)) continue
    matched.set(String(target.topologyId || target.id), target)
  }
  return Array.from(matched.values())
}

const isPlatformToolServiceNode = (node: ComponentTopologyComponent): boolean => {
  if (node?.topologyKind !== 'service') return false
  const type = String(node?.type || node?.serviceType || '').trim().toLowerCase()
  return toolServiceTypes.has(type)
}

const componentConfigSearchText = (comp: ComponentTopologyComponent): string => {
  const cfg = parseComponentConfig(comp?.config)
  if (!cfg) return ''
  const values: string[] = []
  const add = (value: unknown) => {
    if (value === undefined || value === null) return
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
      values.push(String(value))
      return
    }
    if (Array.isArray(value)) {
      value.forEach(add)
      return
    }
    if (typeof value === 'object') {
      Object.entries(value as Record<string, unknown>).forEach(([key, item]) => {
        values.push(key)
        add(item)
      })
    }
  }
  add(cfg.env)
  add(cfg.command)
  add(cfg.args)
  add(cfg.configMaps)
  add(cfg.secrets)
  add(cfg.files)
  add(cfg.bindings)
  return values.join('\n').toLowerCase()
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
