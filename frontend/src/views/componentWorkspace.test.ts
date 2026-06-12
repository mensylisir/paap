import { describe, expect, it } from 'vitest'
import { buildComponentDeliveryChain, buildComponentEvents, buildComponentResourceTopology, buildComponentSummary } from './componentWorkspace'

describe('componentWorkspace', () => {
  it('builds business-oriented summary items for a component', () => {
    const items = buildComponentSummary({
      name: 'orders-api',
      type: 'backend',
      image: 'registry.local/orders:v1.2.3',
      version: 'v1.2.3',
      replicas: 3,
      cpu: '1核',
      memory: '1Gi',
      status: 'running',
      gitRepoUrl: 'http://gitea/app/orders.git',
      sourceMirrorRepoUrl: 'http://gitea/app/orders-source.git',
      gitPath: 'components/orders-api',
      argocdApp: 'orders-dev-api',
    })

    expect(items.map((item) => item.label)).toEqual(['镜像', '版本', '实例规模', '资源规格', '代码路径', '源码镜像仓', '发布应用'])
    expect(items.find((item) => item.label === '源码镜像仓')?.value).toBe('http://gitea/app/orders-source.git')
    expect(items.map((item) => item.value).join(' ')).not.toMatch(/Pod|Deployment|ReplicaSet/i)
  })

  it('does not invent lifecycle events from component status', () => {
    const events = buildComponentEvents({
      name: 'orders-api',
      status: 'running',
      version: 'v1.2.3',
      argocdApp: 'orders-dev-api',
    })

    expect(events).toEqual([])
  })

  it('uses only lifecycle events returned by the backend', () => {
    const events = buildComponentEvents({
      name: 'orders-api',
      status: 'running',
      events: [{ time: '2026-06-08T11:00:00+08:00', type: 'Normal', reason: 'Synced', message: 'ArgoCD synced revision 42' }],
    })

    expect(events).toEqual([{ time: '2026-06-08T11:00:00+08:00', type: 'Normal', reason: 'Synced', message: 'ArgoCD synced revision 42' }])
  })

  it('builds source to cluster delivery chain for source components', () => {
    const chain = buildComponentDeliveryChain({
      name: 'orders-api',
      deliveryMode: 'source',
      sourceRepoUrl: 'http://git/source/orders.git',
      sourceMirrorRepoUrl: 'http://gitea/paap/orders-source.git',
      sourceBranch: 'main',
      gitRepoUrl: 'http://gitea/paap/orders.git',
      gitPath: 'components/orders-api',
      jenkinsJob: 'orders-dev-api-build',
      registryImage: 'registry.orders-dev.example.com/orders/api:v1.2.3',
      pipelineStatus: 'running',
      argocdApp: 'orders-dev-api',
      status: 'running',
    })

    expect(chain.map((step) => step.label)).toEqual(['Source', 'Gitea Source Mirror', 'Jenkins/kpack', 'Registry', 'Gitea Deploy YAML', 'ArgoCD', 'Cluster'])
    expect(chain.find((step) => step.key === 'source')?.value).toContain('http://git/source/orders.git')
    expect(chain.find((step) => step.key === 'gitea-source')?.value).toContain('http://gitea/paap/orders-source.git')
    expect(chain.find((step) => step.key === 'gitea-deploy')?.value).toContain('components/orders-api')
    expect(chain.find((step) => step.key === 'ci')?.status).toBe('running')
    expect(chain.find((step) => step.key === 'registry')?.value).toContain('registry.orders-dev.example.com/orders/api:v1.2.3')
    expect(chain.find((step) => step.key === 'argocd')?.status).toBe('ready')
    expect(chain.find((step) => step.key === 'cluster')?.status).toBe('ready')
  })

  it('builds image to cluster delivery chain without source build steps', () => {
    const chain = buildComponentDeliveryChain({
      name: 'web',
      deliveryMode: 'image',
      image: 'registry.test-staging.paap.local:5000/test-staging/web:v1.0.0',
      gitRepoUrl: 'http://gitea/paap/test-staging-components.git',
      gitPath: 'components/web',
      argocdApp: 'test-staging-web',
      status: 'running',
    })

    expect(chain.map((step) => step.label)).toEqual(['Image', 'Gitea Deploy YAML', 'ArgoCD', 'Cluster'])
    expect(chain.map((step) => step.key)).not.toContain('ci')
    expect(chain.map((step) => step.key)).not.toContain('gitea-source')
    expect(chain.find((step) => step.key === 'image')?.detail).toContain('不经过源码构建')
  })

  it('builds declared component resource topology without inventing pods or replicasets', () => {
    const nodes = buildComponentResourceTopology({
      id: 7,
      name: 'orders-api',
      type: 'backend',
      image: 'registry.local/orders:v1',
      gitRepoUrl: 'http://gitea/paap/test-staging-components.git',
      gitPath: 'components/orders-api',
      argocdApp: 'test-staging-orders-api',
      config: JSON.stringify({
        env: [
          { name: 'DB_PASSWORD', secretName: 'orders-db', secretKey: 'password' },
          { name: 'REDIS_HOST', configMapName: 'redis-config', configMapKey: 'host' },
        ],
        dependencies: ['redis-cache'],
      }),
    })

    expect(nodes.map((node) => node.kind)).toEqual([
      'Component',
      'GitOps Path',
      'Deployment Manifest',
      'Service Manifest',
      'ArgoCD Application',
      'Cluster',
      'SecretKeyRef',
      'ConfigMapKeyRef',
      'Dependency',
    ])
    expect(nodes.map((node) => node.name)).toContain('orders-db/password')
    expect(nodes.map((node) => node.name)).toContain('redis-config/host')
    expect(nodes.map((node) => node.name)).toContain('redis-cache')
    expect(nodes.map((node) => node.kind)).not.toContain('Pod')
    expect(nodes.map((node) => node.kind)).not.toContain('ReplicaSet')
  })
})
