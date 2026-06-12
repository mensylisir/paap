import { describe, expect, it } from 'vitest'
import {
  connectionBindingPreview,
  serviceConfigFields,
  serviceConfigProfile,
  serviceConfigFormFromInstallation,
  serviceConfigPrimaryRows,
  serviceInternalEndpoint,
  serviceTopologyFromWorkspace,
  serviceConfigValuesFromForm,
  redisTopologyFromWorkspace,
} from './serviceAssetConfig'
import type { WorkspaceResource } from './serviceWorkspace'

describe('serviceAssetConfig', () => {
  it('builds a Redis configuration form from saved Helm values without exposing Kubernetes internals as primary rows', () => {
    const service = {
      serviceType: 'redis',
      namespace: 'billing-dev-redis',
      values: JSON.stringify({
        architecture: 'replication',
        'replica.replicaCount': '2',
        'master.persistence.enabled': 'true',
        'master.persistence.size': '12Gi',
        'service.type': 'ClusterIP',
      }),
      runtimeConfig: {
        namespace: 'billing-dev-redis',
        workloadKind: 'StatefulSet',
        workloadName: 'redis-master',
        container: 'redis',
      },
    }

    const form = serviceConfigFormFromInstallation(service)
    const rows = serviceConfigPrimaryRows(service, form)

    expect(form.architecture).toBe('replication')
    expect(form['replica.replicaCount']).toBe(2)
    expect(form['master.persistence.enabled']).toBe(true)
    expect(form['master.persistence.size']).toBe('12Gi')
    expect(form).not.toHaveProperty('master.service.type')
    expect(form).not.toHaveProperty('service.type')
    expect(serviceInternalEndpoint(service)).toBe('redis.billing-dev-redis.svc.cluster.local:6379')
    expect(rows.map((row) => row.label)).toEqual([
      'Redis 架构',
      'Redis 副本',
      'Sentinel 高可用',
      'Master 存储',
    ])
    expect(rows.map((row) => row.label).join(',')).not.toMatch(/Namespace|StatefulSet|Container|命名空间|容器/)
    expect(rows.map((row) => row.label).join(',')).not.toMatch(/访问|NodePort|连接地址/)
  })

  it('maps Redis form values only to deployable Helm values supported by the bundled chart', () => {
    const values = serviceConfigValuesFromForm('redis', {
      architecture: 'replication',
      'sentinel.enabled': true,
      'replica.replicaCount': 2,
      'master.persistence.enabled': true,
      'master.persistence.size': '20Gi',
      'replica.persistence.enabled': true,
      'replica.persistence.size': '20Gi',
      'master.service.type': 'NodePort',
      'master.service.nodePorts.redis': 30379,
      'sentinel.service.type': 'NodePort',
      'sentinel.service.nodePorts.redis': 30379,
      'sentinel.service.nodePorts.sentinel': 30380,
    })

    expect(values).toMatchObject({
      architecture: 'replication',
      'sentinel.enabled': 'true',
      'replica.replicaCount': '2',
      'master.persistence.enabled': 'true',
      'master.persistence.size': '20Gi',
      'replica.persistence.enabled': 'true',
      'replica.persistence.size': '20Gi',
    })
    expect(values).not.toHaveProperty('cluster.enabled')
    expect(values).not.toHaveProperty('cluster.shards')
    expect(values).not.toHaveProperty('master.service.type')
    expect(values).not.toHaveProperty('master.service.nodePorts.redis')
    expect(values).not.toHaveProperty('sentinel.service.type')
    expect(values).not.toHaveProperty('sentinel.service.nodePorts.redis')
    expect(values).not.toHaveProperty('sentinel.service.nodePorts.sentinel')
  })

  it('summarizes Redis cluster topology from live workspace resources without inventing nodes', () => {
    const resources: WorkspaceResource[] = [
      {
        name: 'redis-0',
        type: 'Redis Node',
        status: 'Ready',
        description: 'master',
        annotations: { role: 'master', slots: '0-5460', address: '10.0.0.1:6379' },
      },
      {
        name: 'redis-1',
        type: 'Redis Node',
        status: 'Ready',
        description: 'replica',
        annotations: { role: 'replica', master: 'redis-0', address: '10.0.0.2:6379' },
      },
    ]

    const topology = redisTopologyFromWorkspace(resources)

    expect(topology.summary).toEqual({ masters: 1, replicas: 1, slots: '0-5460' })
    expect(topology.nodes.map((node) => `${node.name}:${node.role}:${node.parent || '-'}`)).toEqual([
      'redis-0:master:-',
      'redis-1:replica:redis-0',
    ])
  })

  it('detects connection binding environment variable conflicts before saving component parameters', () => {
    const preview = connectionBindingPreview(
      { name: 'api', env: [{ name: 'REDIS_HOST', value: 'old-redis' }] },
      { serviceType: 'redis', serviceName: 'redis', namespace: 'billing-dev-redis' },
    )

    expect(preview.bindings.map((binding) => binding.name)).toEqual(['REDIS_HOST', 'REDIS_PORT'])
    expect(preview.conflicts).toEqual(['REDIS_HOST'])
  })

  it('uses service-specific middleware configuration profiles instead of sharing Redis options', () => {
    const redis = serviceConfigProfile({ serviceType: 'redis' })
    const mysql = serviceConfigProfile({ serviceType: 'mysql' })
    const rabbitmq = serviceConfigProfile({ serviceType: 'rabbitmq' })
    const kafka = serviceConfigProfile({ serviceType: 'kafka' })
    const minio = serviceConfigProfile({ serviceType: 'minio' })
    const git = serviceConfigProfile({ serviceType: 'git' })

    expect(redis.kind).toBe('redis')
    expect(redis.showDeploymentConfig).toBe(true)
    expect(redis.showConnectionBindings).toBe(true)
    expect(redis.showTopology).toBe(true)
    expect(redis.showWorkspaceSummary).toBe(false)
    expect(serviceConfigFields({ serviceType: 'redis' }).map((field) => field.key)).toEqual([
      'architecture',
      'sentinel.enabled',
      'replica.replicaCount',
      'master.persistence.enabled',
      'master.persistence.size',
      'replica.persistence.enabled',
      'replica.persistence.size',
    ])
    expect(serviceConfigFields({ serviceType: 'redis' }).flatMap((field) => field.options?.map((option) => option.label) || [])).not.toContain('分片集群')
    expect(serviceConfigFields({ serviceType: 'redis' }).find((field) => field.key === 'architecture')?.options?.map((option) => option.label)).toEqual(['Redis 单节点', 'Redis 主从复制'])

    expect(mysql.kind).toBe('database')
    expect(serviceConfigFields({ serviceType: 'mysql' }).map((field) => field.key)).toEqual([
      'architecture',
      'secondary.replicaCount',
      'primary.persistence.enabled',
      'primary.persistence.size',
      'secondary.persistence.enabled',
      'secondary.persistence.size',
    ])
    expect(serviceConfigFields({ serviceType: 'mysql' }).map((field) => field.key)).not.toContain('sentinel.enabled')
    expect(mysql.showTopology).toBe(true)
    expect(serviceConfigFields({ serviceType: 'postgresql' }).find((field) => field.key === 'architecture')?.options?.map((option) => option.label)).toEqual(['单主库', '主库 + 只读副本'])

    expect(rabbitmq.kind).toBe('message-queue')
    expect(serviceConfigFields({ serviceType: 'rabbitmq' }).map((field) => field.key)).toEqual(['replicaCount', 'persistence.enabled', 'persistence.size'])

    expect(kafka.kind).toBe('message-queue')
    expect(serviceConfigFields({ serviceType: 'kafka' }).map((field) => field.key)).toEqual([
      'controller.replicaCount',
      'broker.replicaCount',
      'controller.persistence.enabled',
      'controller.persistence.size',
      'broker.persistence.enabled',
      'broker.persistence.size',
    ])

    expect(minio.kind).toBe('object-storage')
    expect(serviceConfigFields({ serviceType: 'minio' }).map((field) => field.key)).toEqual(['mode', 'statefulset.replicaCount', 'persistence.enabled', 'persistence.size'])

    expect(git.kind).toBe('tool')
    expect(git.showDeploymentConfig).toBe(false)
    expect(git.showConnectionBindings).toBe(false)
    expect(git.showTopology).toBe(false)
    expect(git.showWorkspaceSummary).toBe(true)
    expect(git.workspaceTitle).toBe('工作台入口')
  })

  it('does not generate Redis-only Helm values for other middleware', () => {
    const values = serviceConfigValuesFromForm('mysql', {
      architecture: 'replication',
      'secondary.replicaCount': 1,
      'primary.persistence.enabled': true,
      'primary.persistence.size': '20Gi',
      'secondary.persistence.enabled': true,
      'secondary.persistence.size': '20Gi',
      'primary.service.type': 'NodePort',
      'primary.service.nodePorts.mysql': 30306,
    })

    expect(values).toMatchObject({
      architecture: 'replication',
      'primary.persistence.enabled': 'true',
      'primary.persistence.size': '20Gi',
      'secondary.replicaCount': '1',
    })
    expect(values).not.toHaveProperty('sentinel.enabled')
    expect(values).not.toHaveProperty('master.service.nodePorts.redis')
    expect(values).not.toHaveProperty('cluster.enabled')
    expect(values).not.toHaveProperty('primary.service.type')
    expect(values).not.toHaveProperty('primary.service.nodePorts.mysql')
  })

  it('keeps non-Redis middleware topology generic and does not render Redis slot language', () => {
    const topology = serviceTopologyFromWorkspace(
      { serviceType: 'rabbitmq' },
      [
        { name: 'rabbitmq-0', type: 'Pod', status: 'Ready', description: 'amqp broker', annotations: { address: '10.0.0.10' } },
      ],
    )

    expect(topology.summaryRows.map((row) => row.label)).toEqual(['实例'])
    expect(topology.nodes).toHaveLength(1)
    expect(topology.nodes[0].detail).toBe('amqp broker')
    expect(JSON.stringify(topology)).not.toMatch(/Slots|slot|master|replica/)
  })
})
