import { describe, expect, it } from 'vitest'
import { argocdEdgePath, buildArgoCDResourceList, buildArgoCDTreeLayout, defaultArgoNodeMetrics, resourceKey } from './argocdTopology'
import type { WorkspaceResource } from '../../views/serviceWorkspace'

const resource = (type: string, name: string, key: string, children: WorkspaceResource[] = []): WorkspaceResource => ({
  type,
  name,
  status: 'Healthy',
  description: 'test-staging',
  annotations: {
    key,
    namespace: 'test-staging',
    health: 'Healthy',
  },
  children,
})

describe('argocdTopology', () => {
  it('keeps resource-tree terminal nodes and vertically centers parents over their child subtrees', () => {
    const app: WorkspaceResource = {
      type: 'Application',
      name: 'test-staging-source-smoke',
      status: 'OutOfSync/Healthy',
      description: 'ArgoCD Application',
      annotations: {
        key: 'app:test-staging-source-smoke',
        treeSource: 'argocd-resource-tree-api',
        treeNodes: [
          resource('Service', 'source-smoke', 'svc'),
          resource('Endpoints', 'source-smoke', 'ep'),
          resource('Deployment', 'source-smoke', 'deploy'),
          resource('ReplicaSet', 'source-smoke-68bd65bdf6', 'rs-current'),
          resource('Pod', 'source-smoke-68bd65bdf6-nc6xb', 'pod-current'),
          resource('ReplicaSet', 'source-smoke-75b7fdd8df', 'rs-old'),
          resource('ControllerRevision', 'source-smoke-75b7fdd8df-rev', 'rev-old'),
        ],
        treeEdges: [
          { from: 'app:test-staging-source-smoke', to: 'svc' },
          { from: 'svc', to: 'ep' },
          { from: 'app:test-staging-source-smoke', to: 'deploy' },
          { from: 'deploy', to: 'rs-current' },
          { from: 'rs-current', to: 'pod-current' },
          { from: 'deploy', to: 'rs-old' },
          { from: 'rs-old', to: 'rev-old' },
        ],
      },
    }

    const layout = buildArgoCDTreeLayout(app)
    const byKey = new Map(layout.nodes.map(node => [node.key, node]))

    expect(layout.nodes.map(node => node.resource.type)).toContain('Endpoints')
    expect(layout.nodes.map(node => node.resource.type)).toContain('Pod')
    expect(layout.nodes.map(node => node.resource.type)).toContain('ControllerRevision')
    expect(layout.edges.map(edge => edge.key)).toContain('svc->ep')
    expect(layout.edges.map(edge => edge.key)).toContain('rs-current->pod-current')

    const deploy = byKey.get('deploy')
    const current = byKey.get('rs-current')
    const old = byKey.get('rs-old')
    const service = byKey.get('svc')
    const endpoints = byKey.get('ep')
    expect(deploy).toBeTruthy()
    expect(current).toBeTruthy()
    expect(old).toBeTruthy()
    expect(service).toBeTruthy()
    expect(endpoints).toBeTruthy()

    const expectedDeployY = Math.round(((current!.subtreeTop + old!.subtreeBottom) - defaultArgoNodeMetrics.height) / 2)
    expect(deploy!.y).toBe(expectedDeployY)
    expect(service!.y).toBe(Math.round(((endpoints!.subtreeTop + endpoints!.subtreeBottom) - defaultArgoNodeMetrics.height) / 2))
    expect(deploy!.x).toBeLessThan(current!.x)
    expect(current!.x).toBeLessThan(byKey.get('pod-current')!.x)
  })

  it('falls back to nested children while preserving explicit resource keys', () => {
    const app = resource('Application', 'billing-dev-api', 'app', [
      resource('Deployment', 'api', 'deploy', [
        resource('ReplicaSet', 'api-abc', 'rs', [
          resource('Pod', 'api-abc-1', 'pod'),
        ]),
      ]),
    ])

    const layout = buildArgoCDTreeLayout(app)

    expect(layout.nodes.map(node => node.key)).toEqual(['app', 'deploy', 'rs', 'pod'])
    expect(layout.edges.map(edge => edge.key)).toEqual(['rs->pod', 'deploy->rs', 'app->deploy'])
    expect(resourceKey(app, 'application')).toBe('app')
  })

  it('centers a short resource tree vertically inside the minimum canvas height', () => {
    const app = resource('Application', 'billing-dev-api', 'app', [
      resource('Deployment', 'api', 'deploy'),
    ])

    const layout = buildArgoCDTreeLayout(app)
    const root = layout.nodes.find(node => node.key === 'app')
    const child = layout.nodes.find(node => node.key === 'deploy')

    expect(layout.height).toBeGreaterThan(400)
    expect(root?.y).toBeGreaterThan(defaultArgoNodeMetrics.paddingY)
    expect(child?.y).toBe(root?.y)
  })

  it('keeps ArgoCD resource-tree roots as application branches without inventing resource edges', () => {
    const app: WorkspaceResource = {
      type: 'Application',
      name: 'test-staging-backend-3',
      status: 'Synced/Healthy',
      description: 'ArgoCD Application',
      annotations: {
        key: 'app:test-staging-backend-3',
        treeNodes: [
          resource('Service', 'backend-3', 'svc'),
          resource('Endpoints', 'backend-3', 'ep'),
          resource('Deployment', 'backend-3', 'deploy'),
          resource('ReplicaSet', 'backend-3-86ddbf7d9d', 'rs-old'),
          resource('ReplicaSet', 'backend-3-9f6c7c4c8', 'rs-current'),
          resource('Pod', 'backend-3-9f6c7c4c8-lm9rz', 'pod-current'),
        ],
        treeEdges: [
          { from: 'svc', to: 'ep' },
          { from: 'deploy', to: 'rs-old' },
          { from: 'deploy', to: 'rs-current' },
          { from: 'rs-current', to: 'pod-current' },
        ],
      },
    }

    const layout = buildArgoCDTreeLayout(app)
    const edgeKeys = layout.edges.map(edge => edge.key)
    const byKey = new Map(layout.nodes.map(node => [node.key, node]))

    expect(edgeKeys).toContain('app:test-staging-backend-3->svc')
    expect(edgeKeys).toContain('app:test-staging-backend-3->deploy')
    expect(edgeKeys).toContain('svc->ep')
    expect(edgeKeys).toContain('deploy->rs-current')
    expect(edgeKeys).not.toContain('app:test-staging-backend-3->ep')
    expect(edgeKeys).not.toContain('app:test-staging-backend-3->rs-current')
    expect(edgeKeys).not.toContain('app:test-staging-backend-3->pod-current')
    expect(byKey.get('app:test-staging-backend-3')?.y).toBe(Math.round(((byKey.get('svc')!.subtreeTop + byKey.get('deploy')!.subtreeBottom) - defaultArgoNodeMetrics.height) / 2))
  })

  it('draws orthogonal links from node boundaries without crossing node interiors', () => {
    const edge = {
      key: 'app->deploy',
      from: { key: 'app', resource: resource('Application', 'app', 'app'), depth: 0, x: 100, y: 120, subtreeTop: 120, subtreeBottom: 182 },
      to: { key: 'deploy', resource: resource('Deployment', 'api', 'deploy'), depth: 1, x: 400, y: 260, subtreeTop: 260, subtreeBottom: 322 },
    }

    expect(argocdEdgePath(edge)).toBe('M 314 151 H 357 V 291 H 400')
  })

  it('lists Application resource-tree nodes in the all resources view', () => {
    const app: WorkspaceResource = {
      type: 'Application',
      name: 'test-staging-source-smoke',
      status: 'Synced/Healthy',
      description: 'ArgoCD Application',
      annotations: {
        key: 'app:test-staging-source-smoke',
        treeNodes: [
          resource('Service', 'source-smoke', 'svc'),
          resource('Endpoints', 'source-smoke', 'ep'),
          resource('Deployment', 'source-smoke', 'deploy'),
          resource('ReplicaSet', 'source-smoke-68bd65bdf6', 'rs-current'),
          resource('Pod', 'source-smoke-68bd65bdf6-nc6xb', 'pod-current'),
        ],
      },
    }

    const resources = buildArgoCDResourceList([app])

    expect(resources.map(item => `${item.type}/${item.name}`)).toEqual([
      'Application/test-staging-source-smoke',
      'Service/source-smoke',
      'Endpoints/source-smoke',
      'Deployment/source-smoke',
      'ReplicaSet/source-smoke-68bd65bdf6',
      'Pod/source-smoke-68bd65bdf6-nc6xb',
    ])
  })
})
