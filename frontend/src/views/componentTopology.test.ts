import { describe, expect, it } from 'vitest'
import {
  buildComponentDependencyEdges,
  buildComponentTopologyNodes,
  buildComponentTopologyLanes,
  buildComponentTopologyZones,
  componentTopologyCanvasViewBox,
  componentTopologyCanvasSizeWithSavedBounds,
  componentTopologyEdgePath,
  componentTopologyUnionBounds,
  componentTopologyZoneKey,
  componentTopologyContentBounds,
  expandComponentTopologyZoneBounds,
  findTopologyNodeAtPoint,
  hasComponentTopologyDragMoved,
  nextComponentTopologyDragPosition,
  nextComponentTopologyZoneResizeBounds,
  nodeKey,
  parseComponentTopologyDisplayNames,
  parseComponentTopologyManualEdges,
  parseComponentTopologyPositions,
  removeComponentTopologyManualEdge,
  serializeComponentTopologyDisplayNames,
  serializeComponentTopologyManualEdges,
  serializeComponentTopologyPositions,
  serviceNetworkSummary,
  shouldSuppressComponentTopologyClick,
} from './componentTopology'

describe('componentTopology', () => {
  it('does not invent dependencies when components do not declare them', () => {
    const components = [
      { id: 1, name: 'web', type: 'frontend' },
      { id: 2, name: 'api', type: 'backend' },
      { id: 3, name: 'redis', type: 'middleware' },
    ]

    expect(buildComponentDependencyEdges(components)).toEqual([])
  })

  it('builds declared component dependencies by id or name', () => {
    const components = [
      { id: 1, name: 'web', type: 'frontend', dependencies: ['api'] },
      { id: 2, name: 'api', type: 'backend', dependencies: [3] },
      { id: 3, name: 'redis', type: 'middleware' },
    ]

    expect(buildComponentDependencyEdges(components)).toEqual([
      { from: 'web', to: 'api', fromId: 1, toId: 2 },
      { from: 'api', to: 'redis', fromId: 2, toId: 3 },
    ])
  })

  it('limits topology lanes to visible components without dropping lane labels', () => {
    const lanes = buildComponentTopologyLanes([
      { id: 1, name: 'web', type: 'frontend' },
      { id: 2, name: 'api', type: 'backend' },
      { id: 3, name: 'queue', type: 'middleware' },
    ])

    expect(lanes.map((lane) => lane.label)).toEqual(['入口层', '服务层', '数据/中间件'])
    expect(lanes.flatMap((lane) => lane.nodes.map((node) => node.name))).toEqual(['web', 'api', 'queue'])
  })

  it('adds installed middleware as real topology nodes without inventing relationships', () => {
    const nodes = buildComponentTopologyNodes(
      [
        { id: 1, name: 'web', type: 'frontend' },
        { id: 2, name: 'api', type: 'backend' },
      ],
      [{ id: 10, name: 'redis-cache', type: 'redis', status: 'running' }]
    )

    expect(nodes.map((node) => ({ topologyId: node.topologyId, topologyKind: node.topologyKind, name: node.name }))).toEqual([
      { topologyId: 'component:1', topologyKind: 'component', name: 'web' },
      { topologyId: 'component:2', topologyKind: 'component', name: 'api' },
      { topologyId: 'service:10', topologyKind: 'service', name: 'redis-cache' },
    ])
    expect(buildComponentDependencyEdges(nodes)).toEqual([])
  })

  it('formats service network addresses for canvas cards', () => {
    expect(serviceNetworkSummary({
      topologyKind: 'service',
      runtimeServiceName: 'redis-master',
      clusterIP: '10.96.0.12',
      loadBalancerIP: '172.18.0.240',
    })).toBe('redis-master · 集群内 10.96.0.12 · 负载均衡 172.18.0.240')
  })

  it('keeps installed tools as canvas nodes in the platform tools lane', () => {
    const lanes = buildComponentTopologyLanes(buildComponentTopologyNodes(
      [{ id: 1, name: 'api', type: 'backend' }],
      [
        { id: 20, serviceName: 'dev-git', serviceType: 'git', status: 'running' } as any,
        { id: 21, serviceName: 'dev-deploy', serviceType: 'deploy', status: 'running' } as any,
        { id: 22, serviceName: 'dev-monitor', serviceType: 'monitor', status: 'running' } as any,
      ],
    ))

    expect(lanes.find((lane) => lane.key === 'tools')?.nodes.map((node) => node.name)).toEqual([
      'dev-git',
      'dev-deploy',
      'dev-monitor',
    ])
  })

  it('groups topology nodes by environment shared and external zones', () => {
    const nodes = [
      { id: 1, name: 'api', topologyKind: 'component' as const },
      { id: 10, name: 'redis', topologyKind: 'service' as const },
      { id: 20, name: 'shared-harbor', topologyKind: 'capability' as any, source: 'shared' },
      { id: 21, name: 'external-postgres', topologyKind: 'capability' as any, source: 'external' },
    ]

    expect(nodes.map(componentTopologyZoneKey)).toEqual(['environment', 'environment', 'shared', 'external'])
    expect(buildComponentTopologyZones(nodes).map((zone) => ({
      key: zone.key,
      label: zone.label,
      names: zone.nodes.map((node) => node.name),
    }))).toEqual([
      { key: 'environment', label: '本环境', names: ['api', 'redis'] },
      { key: 'shared', label: '平台公共', names: ['shared-harbor'] },
      { key: 'external', label: '集群外部', names: ['external-postgres'] },
    ])
  })

  it('builds explicit component dependencies from JSON config to installed services', () => {
    const nodes = buildComponentTopologyNodes(
      [
        {
          id: 1,
          name: 'api',
          type: 'backend',
          config: JSON.stringify({ dependencies: ['redis-cache'] }),
        },
      ],
      [{ id: 10, name: 'redis-cache', type: 'redis', status: 'running' }]
    )

    expect(buildComponentDependencyEdges(nodes)).toEqual([
      {
        from: 'api',
        to: 'redis-cache',
        fromId: 1,
        toId: 10,
        fromKey: 'component:1',
        toKey: 'service:10',
      },
    ])
  })

  it('infers component dependencies from generated environment variables', () => {
    const nodes = buildComponentTopologyNodes([
      {
        id: 1,
        name: 'web',
        type: 'frontend',
        config: JSON.stringify({ env: [{ name: 'BACKEND_URL', value: 'http://backend-1' }] }),
      },
      { id: 2, name: 'backend-1', type: 'backend' },
    ])

    expect(buildComponentDependencyEdges(nodes)).toEqual([
      {
        from: 'web',
        to: 'backend-1',
        fromId: 1,
        toId: 2,
        fromKey: 'component:1',
        toKey: 'component:2',
      },
    ])
  })

  it('infers middleware dependencies from ConfigMap file content and generated bindings', () => {
    const nodes = buildComponentTopologyNodes(
      [
        {
          id: 1,
          name: 'api',
          type: 'backend',
          config: JSON.stringify({
            configMaps: [{
              name: 'api-config',
              data: {
                'application-paap.yml': [
                  'spring:',
                  '  datasource:',
                  '    url: jdbc:postgresql://orders-postgresql.orders-dev-postgresql.svc.cluster.local:5432/postgres',
                  '  data:',
                  '    redis:',
                  '      host: orders-redis-master.orders-dev-redis.svc.cluster.local',
                ].join('\n'),
              },
            }],
            bindings: [{
              targetName: 'orders-postgresql',
              targetType: 'postgresql',
              role: 'database',
            }],
          }),
        },
      ],
      [
        { id: 10, serviceName: 'orders-postgresql', serviceType: 'postgresql', namespace: 'orders-dev-postgresql', status: 'running' } as any,
        { id: 11, serviceName: 'orders-redis-master', serviceType: 'redis', namespace: 'orders-dev-redis', status: 'running' } as any,
      ]
    )

    expect(buildComponentDependencyEdges(nodes)).toEqual([
      {
        from: 'api',
        to: 'orders-postgresql',
        fromId: 1,
        toId: 10,
        fromKey: 'component:1',
        toKey: 'service:10',
      },
      {
        from: 'api',
        to: 'orders-redis-master',
        fromId: 1,
        toId: 11,
        fromKey: 'component:1',
        toKey: 'service:11',
      },
    ])
  })

  it('does not infer an edge from an ambiguous alias shared by multiple services', () => {
    const nodes = buildComponentTopologyNodes(
      [
        {
          id: 1,
          name: 'api',
          type: 'backend',
          config: JSON.stringify({ env: [{ name: 'CACHE_TYPE', value: 'redis' }] }),
        },
      ],
      [
        { id: 10, serviceName: 'redis-a', serviceType: 'redis', status: 'running' } as any,
        { id: 11, serviceName: 'redis-b', serviceType: 'redis', status: 'running' } as any,
      ]
    )

    expect(buildComponentDependencyEdges(nodes)).toEqual([])
  })

  it('normalizes installed service installations before resolving dependencies', () => {
    const nodes = buildComponentTopologyNodes(
      [
        {
          id: 1,
          name: 'api',
          type: 'backend',
          config: JSON.stringify({ dependencies: ['redis'] }),
        },
      ],
      [{ id: 10, serviceName: '', serviceType: 'redis', status: 'running' } as any]
    )

    expect(nodes.find((node) => node.topologyId === 'service:10')).toMatchObject({
      name: 'redis',
      type: 'redis',
    })
    expect(buildComponentDependencyEdges(nodes)).toEqual([
      {
        from: 'api',
        to: 'redis',
        fromId: 1,
        toId: 10,
        fromKey: 'component:1',
        toKey: 'service:10',
      },
    ])
  })

  it('parses only valid saved canvas positions', () => {
    expect(parseComponentTopologyPositions('{"component:1":{"x":120,"y":88},"bad":{"x":"no","y":12},"service:2":{"x":"240","y":"144"},"zone:shared":{"x":300,"y":120,"width":420,"height":260}}')).toEqual({
      'component:1': { x: 120, y: 88 },
      'service:2': { x: 240, y: 144 },
      'zone:shared': { x: 300, y: 120, width: 420, height: 260 },
    })
    expect(parseComponentTopologyPositions('not-json')).toEqual({})
  })

  it('serializes only finite canvas positions', () => {
    expect(serializeComponentTopologyPositions({
      'component:1': { x: 120, y: 88 },
      'zone:shared': { x: 300, y: 120, width: 420, height: 260 },
      bad: { x: Number.NaN, y: 12 },
    })).toBe('{"component:1":{"x":120,"y":88},"zone:shared":{"x":300,"y":120,"width":420,"height":260}}')
  })

  it('keeps the SVG viewBox in canvas coordinates so CSS zoom does not break links', () => {
    expect(componentTopologyCanvasViewBox({ width: 1280, height: 720 })).toBe('0 0 1280 720')
    expect(componentTopologyCanvasViewBox({ width: 0, height: -1 })).toBe('0 0 0 0')
  })

  it('persists only explicit canvas links without duplicates or self links', () => {
    expect(parseComponentTopologyManualEdges(JSON.stringify([
      { fromKey: 'component:1', toKey: 'component:2' },
      { fromKey: 'component:1', toKey: 'component:2' },
      { fromKey: 'component:1', toKey: 'component:1' },
      { fromKey: '', toKey: 'service:1' },
      { fromKey: 'component:2', toKey: 'service:10' },
    ]))).toEqual([
      { fromKey: 'component:1', toKey: 'component:2' },
      { fromKey: 'component:2', toKey: 'service:10' },
    ])
    expect(parseComponentTopologyManualEdges('not-json')).toEqual([])

    expect(serializeComponentTopologyManualEdges([
      { fromKey: 'component:1', toKey: 'component:2' },
      { fromKey: 'component:1', toKey: 'component:2' },
      { fromKey: 'component:2', toKey: 'component:2' },
    ])).toBe('[{"fromKey":"component:1","toKey":"component:2"}]')
  })

  it('removes only the selected manual canvas link', () => {
    const edges = [
      { fromKey: 'component:1', toKey: 'component:2' },
      { fromKey: 'component:2', toKey: 'service:10' },
      { fromKey: 'service:10', toKey: 'component:1' },
    ]

    expect(removeComponentTopologyManualEdge(edges, 'component:2', 'service:10')).toEqual([
      { fromKey: 'component:1', toKey: 'component:2' },
      { fromKey: 'service:10', toKey: 'component:1' },
    ])
    expect(removeComponentTopologyManualEdge(edges, 'component:9', 'service:10')).toEqual(edges)
  })

  it('finds connection targets from canvas coordinates instead of browser hit testing', () => {
    const nodes = [
      { topologyId: 'component:1', x: 120, y: 80, width: 196, height: 70 },
      { topologyId: 'component:2', x: 420, y: 210, width: 196, height: 70 },
    ]

    expect(nodeKey(nodes[0])).toBe('component:1')
    expect(findTopologyNodeAtPoint(nodes, { x: 500, y: 245 }, 'component:1')?.topologyId).toBe('component:2')
    expect(findTopologyNodeAtPoint(nodes, { x: 130, y: 90 }, 'component:1')).toBeNull()
    expect(findTopologyNodeAtPoint(nodes, { x: 20, y: 20 })).toBeNull()
  })

  it('treats pointerup displacement as a drag so node clicks do not open the side drawer', () => {
    expect(hasComponentTopologyDragMoved({ startX: 120, startY: 90, currentX: 124, currentY: 90 })).toBe(false)
    expect(hasComponentTopologyDragMoved({ startX: 120, startY: 90, currentX: 126, currentY: 91 })).toBe(true)
    expect(hasComponentTopologyDragMoved({ startX: 120, startY: 90, currentX: 120, currentY: 96 })).toBe(true)
  })

  it('suppresses the click fired after dragging a topology node', () => {
    expect(shouldSuppressComponentTopologyClick('component:1', {
      suppressNext: true,
      suppressKey: 'component:1',
    })).toBe(true)

    expect(shouldSuppressComponentTopologyClick('component:1', {
      suppressNext: false,
      recentDragKey: 'component:1',
      recentDragAt: 1000,
      now: 1200,
      windowMs: 350,
    })).toBe(true)

    expect(shouldSuppressComponentTopologyClick('component:1', {
      suppressNext: false,
      recentDragKey: 'component:1',
      recentDragAt: 1000,
      now: 1500,
      windowMs: 350,
    })).toBe(false)
  })

  it('clamps dragged component positions inside the visible canvas area', () => {
    expect(nextComponentTopologyDragPosition({
      originX: 24,
      originY: 36,
      startX: 100,
      startY: 100,
      currentX: 40,
      currentY: 42,
    })).toEqual({ x: 12, y: 46 })

    expect(nextComponentTopologyDragPosition({
      originX: 24,
      originY: 36,
      startX: 100,
      startY: 100,
      currentX: 140,
      currentY: 160,
    })).toEqual({ x: 64, y: 96 })

    expect(nextComponentTopologyDragPosition({
      originX: 240,
      originY: 180,
      startX: 100,
      startY: 100,
      currentX: 600,
      currentY: 460,
      minX: 40,
      minY: 56,
      maxX: 320,
      maxY: 240,
    })).toEqual({ x: 320, y: 240 })
  })

  it('expands topology zone bounds when dragged cards move past the frame', () => {
    expect(expandComponentTopologyZoneBounds(
      { left: 100, top: 80, width: 420, height: 260 },
      [{ x: 470, y: 290, width: 196, height: 70 }],
      { paddingX: 12, paddingTop: 24, paddingBottom: 12, minLeft: 16, minTop: 16 }
    )).toEqual({ left: 100, top: 80, width: 578, height: 292 })

    expect(expandComponentTopologyZoneBounds(
      { left: 100, top: 80, width: 420, height: 260 },
      [{ x: 36, y: 44, width: 196, height: 70 }],
      { paddingX: 12, paddingTop: 24, paddingBottom: 12, minLeft: 16, minTop: 16 }
    )).toEqual({ left: 24, top: 20, width: 496, height: 320 })
  })

  it('calculates topology zone minimum bounds from contained cards', () => {
    expect(componentTopologyContentBounds(
      [{ x: 470, y: 290, width: 196, height: 70 }],
      { paddingX: 12, paddingTop: 24, paddingBottom: 12, minLeft: 16, minTop: 16 }
    )).toEqual({ left: 458, top: 266, width: 220, height: 106 })
  })

  it('keeps environment canvas stable until saved bounds are near the edge', () => {
    expect(componentTopologyCanvasSizeWithSavedBounds(
      { width: 1280, height: 680 },
      { right: 1180, bottom: 590 },
      16
    )).toEqual({ width: 1280, height: 680 })

    expect(componentTopologyCanvasSizeWithSavedBounds(
      { width: 1280, height: 680 },
      { right: 1270, bottom: 670 },
      16
    )).toEqual({ width: 1286, height: 686 })
  })

  it('resizes topology zones on all four edges without covering contained cards', () => {
    const base = {
      originBounds: { left: 100, top: 80, width: 420, height: 260 },
      contentBounds: { left: 128, top: 112, right: 410, bottom: 260 },
      minLeft: 16,
      minTop: 16,
    }

    expect(nextComponentTopologyZoneResizeBounds({ ...base, edges: ['right'], dx: 80, dy: 0 }))
      .toEqual({ left: 100, top: 80, width: 500, height: 260 })
    expect(nextComponentTopologyZoneResizeBounds({ ...base, edges: ['bottom'], dx: 0, dy: 90 }))
      .toEqual({ left: 100, top: 80, width: 420, height: 350 })
    expect(nextComponentTopologyZoneResizeBounds({ ...base, edges: ['left'], dx: 220, dy: 0 }))
      .toEqual({ left: 128, top: 80, width: 392, height: 260 })
    expect(nextComponentTopologyZoneResizeBounds({ ...base, edges: ['top'], dx: 0, dy: 220 }))
      .toEqual({ left: 100, top: 112, width: 420, height: 228 })
  })

  it('expands saved zone bounds left and up when content sits outside the frame', () => {
    expect(componentTopologyUnionBounds(
      { left: 1012, top: 46, width: 321, height: 293 },
      { left: 889, top: 63, width: 220, height: 106 }
    )).toEqual({ left: 889, top: 46, width: 444, height: 293 })
  })

  it('draws canvas links from the nearest node boundaries instead of through cards', () => {
    expect(componentTopologyEdgePath({
      fromNode: { x: 120, y: 80, width: 196, height: 70 },
      toNode: { x: 520, y: 210, width: 196, height: 70 },
    })).toBe('M 316 115 H 418 V 245 H 520')

    expect(componentTopologyEdgePath({
      fromNode: { x: 520, y: 210, width: 196, height: 70 },
      toNode: { x: 120, y: 80, width: 196, height: 70 },
    })).toBe('M 520 245 H 418 V 115 H 316')
  })

  it('parses only non-empty canvas display names', () => {
    expect(parseComponentTopologyDisplayNames('{"component:1":"My App","service:10":"主数据库","bad":""}')).toEqual({
      'component:1': 'My App',
      'service:10': '主数据库',
    })
    expect(parseComponentTopologyDisplayNames(null)).toEqual({})
    expect(parseComponentTopologyDisplayNames('not-json')).toEqual({})
  })

  it('serializes canvas display names trimming whitespace keys', () => {
    expect(serializeComponentTopologyDisplayNames({
      'component:1': 'My App',
      'service:10': '主数据库',
      '  ': 'ignored',
      'component:2': '',
    })).toBe('{"component:1":"My App","service:10":"主数据库"}')
  })

  it('applies display name overrides from canvas state to topology nodes', () => {
    const nodes = buildComponentTopologyNodes(
      [
        { id: 1, name: 'web', type: 'frontend' },
        { id: 2, name: 'api', type: 'backend' },
      ],
      [{ id: 10, name: 'redis-cache', type: 'redis', status: 'running' }],
      { 'component:1': '用户前端', 'service:10': '主缓存' }
    )

    expect(nodes.map((n) => ({ topologyId: n.topologyId, name: n.name }))).toEqual([
      { topologyId: 'component:1', name: '用户前端' },
      { topologyId: 'component:2', name: 'api' },
      { topologyId: 'service:10', name: '主缓存' },
    ])
  })

  it('falls back to real name when display name is empty or absent', () => {
    const nodes = buildComponentTopologyNodes(
      [{ id: 1, name: 'web', type: 'frontend' }],
      [{ id: 10, name: 'redis', type: 'redis', status: 'running' }],
      { 'component:1': '', 'service:10': '  ' }
    )

    expect(nodes.find((n) => n.topologyId === 'component:1')?.name).toBe('web')
    expect(nodes.find((n) => n.topologyId === 'service:10')?.name).toBe('redis')
  })

  it('ignores display name overrides for nodes that do not exist', () => {
    const nodes = buildComponentTopologyNodes(
      [{ id: 1, name: 'web', type: 'frontend' }],
      [],
      { 'component:999': 'Ghost', 'service:888': 'Phantom' }
    )

    expect(nodes).toHaveLength(1)
    expect(nodes[0].name).toBe('web')
  })
})
