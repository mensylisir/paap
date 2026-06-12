export type ComponentTopologyComponent = {
  id?: string | number
  name?: string
  type?: string
  serviceName?: string
  serviceType?: string
  status?: string
  config?: unknown
  dependencies?: unknown
  dependsOn?: unknown
  dependencyNames?: unknown
  dependencyComponents?: unknown
  topologyId?: string
  topologyKind?: 'component' | 'service'
  serviceId?: string | number
  componentId?: string | number
}

export type ComponentTopologyEdge = {
  from: string
  to: string
  fromId: number
  toId: number
  fromKey?: string
  toKey?: string
}

export type ComponentTopologyLane = {
  key: string
  label: string
  nodes: ComponentTopologyComponent[]
}

export type ComponentTopologyPosition = {
  x: number
  y: number
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

const laneLabels: Record<string, string> = {
  frontend: '入口层',
  backend: '服务层',
  data: '数据/中间件',
  tools: '平台工具',
  other: '其他组件',
}

const laneOrder = ['frontend', 'backend', 'data', 'tools', 'other']

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
  services: ComponentTopologyComponent[] = []
): ComponentTopologyComponent[] => {
  const componentNodes = components.map((comp) => ({
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
    return {
      ...normalized,
      topologyKind: 'service' as const,
      topologyId: stableNodeId('service', normalized),
      serviceId: svc.id,
      status: svc.status || 'running',
    }
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

export const explicitComponentDependencies = (comp: ComponentTopologyComponent) => {
  const config = parseComponentConfig(comp?.config)
  const raw = comp?.dependencies
    ?? comp?.dependsOn
    ?? comp?.dependencyNames
    ?? comp?.dependencyComponents
    ?? config?.dependencies
    ?? config?.dependsOn
    ?? config?.dependencyNames
    ?? config?.dependencyComponents
  if (Array.isArray(raw)) {
    return raw
      .map((item) => String(typeof item === 'object' && item ? (item as any).name || (item as any).service || (item as any).component || (item as any).id : item))
      .map((item) => item.trim())
      .filter(Boolean)
  }
  if (typeof raw === 'string') return raw.split(',').map((item) => item.trim()).filter(Boolean)
  return []
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
  for (const comp of components) {
    const name = String(comp.name || '').trim()
    const id = String(comp.id || '').trim()
    const topologyId = String(comp.topologyId || '').trim()
    if (name) byName.set(name.toLowerCase(), comp)
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

  for (const comp of components) {
    if (comp.topologyKind === 'service') continue
    for (const dependency of explicitComponentDependencies(comp)) {
      const target = byTopologyId.get(dependency.toLowerCase()) || byName.get(dependency.toLowerCase()) || byId.get(dependency)
      if (target) addEdge(comp, target)
    }
  }

  return edges
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
      positions[key] = { x, y }
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
    serializable[key] = { x, y }
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
  const zoom = Number.isFinite(input.zoom) && Number(input.zoom) > 0 ? Number(input.zoom) : 1
  return {
    x: Math.max(minX, Number(input.originX) + (Number(input.currentX) - Number(input.startX)) / zoom),
    y: Math.max(minY, Number(input.originY) + (Number(input.currentY) - Number(input.startY)) / zoom),
  }
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
