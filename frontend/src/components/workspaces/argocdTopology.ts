import type { WorkspaceResource } from '../../views/serviceWorkspace'

export type ArgoTreeRow = {
  key: string
  resource: WorkspaceResource
  depth: number
  x: number
  y: number
  subtreeTop: number
  subtreeBottom: number
}

export type ArgoTreeEdge = {
  key: string
  from: ArgoTreeRow
  to: ArgoTreeRow
}

export type ArgoNodeMetrics = {
  width: number
  height: number
  gapX: number
  gapY: number
  paddingX: number
  paddingY: number
}

export type ArgoTreeLayout = {
  nodes: ArgoTreeRow[]
  edges: ArgoTreeEdge[]
  width: number
  height: number
}

export const defaultArgoNodeMetrics: ArgoNodeMetrics = {
  width: 214,
  height: 62,
  gapX: 86,
  gapY: 24,
  paddingX: 64,
  paddingY: 48,
}

export const resourceKey = (resource: WorkspaceResource, fallback: string) => {
  const explicit = String(resource.annotations?.key || '').trim()
  if (explicit) return explicit
  const uid = String(resource.annotations?.uid || '').trim()
  if (uid) return `uid:${uid}`
  return `${resource.type}:${resource.annotations?.namespace || resource.description || ''}:${resource.name}:${fallback}`
}

const kindOrder: Record<string, number> = {
  Application: 0,
  Service: 10,
  Endpoints: 20,
  EndpointSlice: 21,
  Ingress: 22,
  Deployment: 30,
  StatefulSet: 31,
  DaemonSet: 32,
  ReplicaSet: 40,
  ControllerRevision: 41,
  Pod: 50,
  ConfigMap: 60,
  Secret: 61,
}

const sortResources = (items: WorkspaceResource[]) =>
  [...items].sort((a, b) => {
    const orderA = kindOrder[a.type || ''] ?? 100
    const orderB = kindOrder[b.type || ''] ?? 100
    if (orderA !== orderB) return orderA - orderB
    return String(a.name || '').localeCompare(String(b.name || ''))
  })

const flattenResourceTree = (resources: WorkspaceResource[]) => {
  const nodes: WorkspaceResource[] = []
  const edges: Array<{ from: string; to: string }> = []
  const walk = (resource: WorkspaceResource, fallback: string, parentKey = '') => {
    const key = resourceKey(resource, fallback)
    nodes.push(resource)
    if (parentKey) edges.push({ from: parentKey, to: key })
    for (const [index, child] of (resource.children || []).entries()) {
      walk(child, `${fallback}.${index}`, key)
    }
  }
  for (const [index, resource] of resources.entries()) {
    walk(resource, `child.${index}`)
  }
  return { nodes, edges }
}

export const buildArgoCDResourceList = (resources: WorkspaceResource[]): WorkspaceResource[] => {
  const out: WorkspaceResource[] = []
  const seen = new Set<string>()
  const add = (resource: WorkspaceResource, fallback: string) => {
    const key = resourceKey(resource, fallback)
    if (seen.has(key)) return
    seen.add(key)
    out.push(resource)
  }

  for (const [index, resource] of resources.entries()) {
    add(resource, `resource.${index}`)
    if (resource.type !== 'Application') continue
    const annotationNodes = Array.isArray(resource.annotations?.treeNodes)
      ? resource.annotations.treeNodes as WorkspaceResource[]
      : []
    const treeNodes = annotationNodes.length ? annotationNodes : flattenResourceTree(resource.children || []).nodes
    for (const [nodeIndex, node] of treeNodes.entries()) {
      add(node, `${resource.name || index}.tree.${nodeIndex}`)
    }
  }
  return out
}

export const buildArgoCDTreeLayout = (
  app: WorkspaceResource | null | undefined,
  metrics: ArgoNodeMetrics = defaultArgoNodeMetrics
): ArgoTreeLayout => {
  if (!app) return { nodes: [], edges: [], width: 760, height: 420 }

  const appKey = resourceKey(app, 'application')
  const annotationNodes = Array.isArray(app.annotations?.treeNodes) ? app.annotations.treeNodes as WorkspaceResource[] : []
  const annotationEdges = Array.isArray(app.annotations?.treeEdges) ? app.annotations.treeEdges as Array<Record<string, string>> : []
  const childTree = flattenResourceTree(app.children || [])
  const rawNodes = annotationNodes.length ? annotationNodes : childTree.nodes
  const rawEdges = annotationNodes.length ? annotationEdges : childTree.edges

  const graphResources: WorkspaceResource[] = [app, ...rawNodes]
  const byKey = new Map<string, WorkspaceResource>()
  const childrenByKey = new Map<string, string[]>()
  const parentByKey = new Map<string, string>()
  for (const resource of graphResources) {
    const key = resourceKey(resource, resource === app ? 'application' : `${resource.type}:${resource.name}`)
    if (!byKey.has(key)) byKey.set(key, resource)
  }
  for (const key of byKey.keys()) childrenByKey.set(key, [])

  const addEdge = (fromRaw: string, toRaw: string) => {
    let from = String(fromRaw || '').trim()
    const to = String(toRaw || '').trim()
    if (!to || !byKey.has(to)) return
    if (!from || !byKey.has(from)) from = appKey
    if (from === to) return
    const siblings = childrenByKey.get(from) || []
    if (!siblings.includes(to)) siblings.push(to)
    childrenByKey.set(from, siblings)
    if (!parentByKey.has(to)) parentByKey.set(to, from)
  }
  for (const edge of rawEdges) addEdge(String(edge.from || ''), String(edge.to || ''))
  for (const resource of rawNodes) {
    const key = resourceKey(resource, `${resource.type}:${resource.name}`)
    if (!parentByKey.has(key)) addEdge(appKey, key)
  }

  for (const [parent, childKeys] of childrenByKey.entries()) {
    childrenByKey.set(parent, sortResources(childKeys.map(key => byKey.get(key)).filter(Boolean) as WorkspaceResource[])
      .map(resource => resourceKey(resource, `${resource.type}:${resource.name}`)))
  }

  const rows: ArgoTreeRow[] = []
  const rowsByKey = new Map<string, ArgoTreeRow>()
  const edgePairs: Array<{ from: string; to: string }> = []
  let cursorY = metrics.paddingY
  let maxDepth = 0
  const visiting = new Set<string>()
  const visited = new Set<string>()

  const layout = (key: string, depth: number): ArgoTreeRow | null => {
    const resource = byKey.get(key)
    if (!resource) return null
    if (visiting.has(key)) return null
    if (visited.has(key)) return rowsByKey.get(key) || null
    visiting.add(key)
    maxDepth = Math.max(maxDepth, depth)

    const childRows = (childrenByKey.get(key) || [])
      .map(childKey => layout(childKey, depth + 1))
      .filter(Boolean) as ArgoTreeRow[]
    for (const child of childRows) edgePairs.push({ from: key, to: child.key })

    let y = cursorY
    let subtreeTop = cursorY
    let subtreeBottom = cursorY + metrics.height
    if (childRows.length) {
      subtreeTop = Math.min(...childRows.map(row => row.subtreeTop))
      subtreeBottom = Math.max(...childRows.map(row => row.subtreeBottom))
      y = Math.round((subtreeTop + subtreeBottom - metrics.height) / 2)
    } else {
      cursorY += metrics.height + metrics.gapY
    }

    const row: ArgoTreeRow = {
      key,
      resource,
      depth,
      x: metrics.paddingX + depth * (metrics.width + metrics.gapX),
      y,
      subtreeTop,
      subtreeBottom,
    }
    rows.push(row)
    rowsByKey.set(key, row)
    visited.add(key)
    visiting.delete(key)
    return row
  }

  layout(appKey, 0)
  for (const key of byKey.keys()) {
    if (!visited.has(key)) layout(key, 1)
  }

  const edges: ArgoTreeEdge[] = []
  const seenEdges = new Set<string>()
  for (const edge of edgePairs) {
    const from = rowsByKey.get(edge.from)
    const to = rowsByKey.get(edge.to)
    const key = `${edge.from}->${edge.to}`
    if (!from || !to || seenEdges.has(key)) continue
    seenEdges.add(key)
    edges.push({ key, from, to })
  }

  const contentTop = rows.length ? Math.min(...rows.map(row => row.subtreeTop)) : metrics.paddingY
  const contentBottom = rows.length ? Math.max(...rows.map(row => row.subtreeBottom)) : metrics.paddingY + metrics.height
  const contentHeight = Math.max(metrics.height, contentBottom - contentTop)
  const canvasHeight = Math.max(520, cursorY + metrics.paddingY)
  const offsetY = Math.max(0, Math.round((canvasHeight - contentHeight) / 2) - contentTop)
  if (offsetY > 0) {
    for (const row of rows) {
      row.y += offsetY
      row.subtreeTop += offsetY
      row.subtreeBottom += offsetY
    }
  }

  return {
    nodes: rows.sort((a, b) => a.depth === b.depth ? a.y - b.y : a.depth - b.depth),
    edges,
    width: Math.max(980, metrics.paddingX * 2 + (maxDepth + 1) * metrics.width + maxDepth * metrics.gapX),
    height: canvasHeight,
  }
}

export const argocdEdgePath = (edge: ArgoTreeEdge, metrics: ArgoNodeMetrics = defaultArgoNodeMetrics) => {
  const startX = edge.from.x + metrics.width
  const startY = edge.from.y + metrics.height / 2
  const endX = edge.to.x
  const endY = edge.to.y + metrics.height / 2
  const elbowX = startX + Math.max(30, (endX - startX) / 2)
  return `M ${startX} ${startY} H ${elbowX} V ${endY} H ${endX}`
}
