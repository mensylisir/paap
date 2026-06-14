import { describe, expect, it } from 'vitest'
import {
  connectionBindingPreview,
  serviceConfigFieldVisible,
  serviceConfigFields,
  serviceConfigProfile,
  serviceConfigType,
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
    expect(serviceInternalEndpoint(service)).toBe('redis-master.billing-dev-redis.svc.cluster.local:6379')
    expect(rows.map((row) => row.label)).toEqual([
      'Redis 架构',
      'Redis 副本',
      '主节点存储',
    ])
    expect(rows.map((row) => row.label).join(',')).not.toMatch(/Namespace|StatefulSet|Container|命名空间|容器/)
    expect(rows.map((row) => row.label).join(',')).not.toMatch(/访问|NodePort|连接地址/)
  })

  it('uses Helm runtime service names for connection endpoints instead of display names', () => {
    const postgres = {
      serviceType: 'postgresql',
      serviceName: 'dev-postgresql',
      releaseName: 'billing-dev-postgresql',
      namespace: 'billing-dev-postgresql',
      values: JSON.stringify({ fullnameOverride: 'billing-dev-postgresql' }),
    }
    const redis = {
      serviceType: 'redis',
      serviceName: 'dev-redis',
      releaseName: 'billing-dev-redis',
      namespace: 'billing-dev-redis',
      values: JSON.stringify({ fullnameOverride: 'billing-dev-redis' }),
    }

    expect(serviceInternalEndpoint(postgres)).toBe('billing-dev-postgresql.billing-dev-postgresql.svc.cluster.local:5432')
    expect(serviceInternalEndpoint(redis)).toBe('billing-dev-redis-master.billing-dev-redis.svc.cluster.local:6379')
    expect(connectionBindingPreview({}, redis).bindings[0]).toMatchObject({
      name: 'REDIS_HOST',
      value: 'billing-dev-redis-master.billing-dev-redis.svc.cluster.local',
    })
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
      'sentinel.enabled': 'false',
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

  it('maps Redis Cluster mode to the real redis-cluster chart values and startup nodes', () => {
    const values = serviceConfigValuesFromForm('redis', {
      architecture: 'cluster',
      'cluster.nodes': 4,
      'cluster.replicas': 1,
      'persistence.enabled': true,
      'persistence.size': '16Gi',
      'master.persistence.enabled': true,
      'replica.persistence.enabled': true,
      'sentinel.enabled': true,
    })
    const preview = connectionBindingPreview({}, {
      serviceType: 'redis',
      serviceName: 'billing-dev-redis-master',
      namespace: 'billing-dev-redis',
      values: JSON.stringify(values),
    })

    expect(values).toMatchObject({
      architecture: 'cluster',
      'cluster.init': 'true',
      'cluster.nodes': '6',
      'cluster.replicas': '1',
      'persistence.enabled': 'true',
      'persistence.size': '16Gi',
      usePassword: 'true',
      'sentinel.enabled': 'false',
    })
    expect(values).not.toHaveProperty('master.persistence.enabled')
    expect(values).not.toHaveProperty('replica.persistence.enabled')
    expect(preview.bindings).toEqual(expect.arrayContaining([
      { name: 'REDIS_HOST', value: 'billing-dev-redis.billing-dev-redis.svc.cluster.local', source: 'billing-dev-redis' },
      { name: 'REDIS_CLUSTER_HOST', value: 'billing-dev-redis.billing-dev-redis.svc.cluster.local', source: 'billing-dev-redis' },
      { name: 'REDIS_CLUSTER_PORT', value: '6379', source: 'billing-dev-redis' },
    ]))
    expect(preview.bindings.find((item) => item.name === 'REDIS_CLUSTER_NODES')?.value).toContain('billing-dev-redis-5.billing-dev-redis-headless.billing-dev-redis.svc.cluster.local:6379')
    expect(serviceConfigPrimaryRows({ serviceType: 'redis' }, {
      architecture: 'cluster',
      'cluster.nodes': 6,
      'cluster.replicas': 1,
      'persistence.enabled': true,
      'persistence.size': '16Gi',
    })).toEqual([
      { label: 'Redis 架构', value: 'Redis Cluster' },
      { label: '集群节点数', value: '6' },
      { label: '每主节点副本', value: '1' },
      { label: '集群节点存储', value: '16Gi' },
    ])
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

  it('treats Redis Sentinel as a first-class deployable mode backed by the bundled chart', () => {
    const form = serviceConfigFormFromInstallation({
      serviceType: 'redis',
      namespace: 'billing-dev-redis',
      values: JSON.stringify({
        architecture: 'replication',
        'sentinel.enabled': 'true',
        'sentinel.masterSet': 'cache-main',
        'replica.replicaCount': '3',
      }),
    })
    const values = serviceConfigValuesFromForm('redis', {
      architecture: 'sentinel',
      'replica.replicaCount': 3,
      'sentinel.masterSet': 'cache-main',
      'master.persistence.enabled': true,
      'master.persistence.size': '10Gi',
      'replica.persistence.enabled': true,
      'replica.persistence.size': '10Gi',
    })
    const preview = connectionBindingPreview({}, {
      serviceType: 'redis',
      serviceName: 'redis',
      releaseName: 'billing-dev-redis',
      namespace: 'billing-dev-redis',
      values: JSON.stringify({ 'sentinel.enabled': 'true', 'sentinel.masterSet': 'cache-main' }),
    })

    expect(form.architecture).toBe('sentinel')
    expect(serviceConfigFields({ serviceType: 'redis' }).find((field) => field.key === 'architecture')?.options?.map((option) => option.label)).toEqual([
      'Redis 单节点',
      'Redis 主从复制',
      'Redis Sentinel',
      'Redis Cluster',
    ])
    expect(serviceConfigFields({ serviceType: 'redis' }).find((field) => field.key === 'sentinel.masterSet')).toMatchObject({
      label: '哨兵监控名称',
      showWhen: { key: 'architecture', equals: 'sentinel' },
    })
    expect(values).toMatchObject({
      architecture: 'replication',
      'sentinel.enabled': 'true',
      'sentinel.masterSet': 'cache-main',
      'replica.replicaCount': '3',
      'master.persistence.enabled': 'true',
      'master.persistence.size': '10Gi',
      'replica.persistence.enabled': 'true',
      'replica.persistence.size': '10Gi',
    })
    expect(preview.bindings).toEqual(expect.arrayContaining([
      { name: 'REDIS_HOST', value: 'billing-dev-redis.billing-dev-redis.svc.cluster.local', source: 'billing-dev-redis' },
      { name: 'REDIS_PORT', value: '6379', source: 'billing-dev-redis' },
      { name: 'REDIS_SENTINEL_HOST', value: 'billing-dev-redis.billing-dev-redis.svc.cluster.local', source: 'billing-dev-redis' },
      { name: 'REDIS_SENTINEL_PORT', value: '26379', source: 'billing-dev-redis' },
      { name: 'REDIS_SENTINEL_MASTER_NAME', value: 'cache-main', source: 'billing-dev-redis' },
    ]))
  })

  it('previews Redis Sentinel service names without the Bitnami master suffix', () => {
    const preview = connectionBindingPreview({}, {
      serviceType: 'redis',
      serviceName: 'billing-dev-redis-master',
      namespace: 'billing-dev-redis',
      values: JSON.stringify({ 'sentinel.enabled': 'true', 'sentinel.masterSet': 'mymaster' }),
    })

    expect(preview.bindings).toEqual(expect.arrayContaining([
      { name: 'REDIS_HOST', value: 'billing-dev-redis.billing-dev-redis.svc.cluster.local', source: 'billing-dev-redis' },
      { name: 'REDIS_SENTINEL_HOST', value: 'billing-dev-redis.billing-dev-redis.svc.cluster.local', source: 'billing-dev-redis' },
    ]))
  })

  it('detects connection binding environment variable conflicts before saving component parameters', () => {
    const preview = connectionBindingPreview(
      { name: 'api', env: [{ name: 'REDIS_HOST', value: 'old-redis' }] },
      { serviceType: 'redis', serviceName: 'redis', namespace: 'billing-dev-redis' },
    )

    expect(preview.bindings.map((binding) => binding.name)).toEqual(['REDIS_HOST', 'REDIS_PORT'])
    expect(preview.conflicts).toEqual(['REDIS_HOST'])
  })

  it('keeps PV size fields discoverable even before persistence is enabled', () => {
    const registryFields = serviceConfigFields({ serviceType: 'registry' })
    expect(serviceConfigFieldVisible(registryFields.find((field) => field.key === 'persistence.size')!, {
      'persistence.enabled': false,
    })).toBe(true)

    const redisFields = serviceConfigFields({ serviceType: 'redis' })
    expect(serviceConfigFieldVisible(redisFields.find((field) => field.key === 'master.persistence.size')!, {
      architecture: 'replication',
      'master.persistence.enabled': false,
    })).toBe(true)
    expect(serviceConfigFieldVisible(redisFields.find((field) => field.key === 'replica.persistence.size')!, {
      architecture: 'replication',
      'replica.persistence.enabled': false,
    })).toBe(true)
    expect(serviceConfigFieldVisible(redisFields.find((field) => field.key === 'persistence.size')!, {
      architecture: 'cluster',
      'persistence.enabled': false,
    })).toBe(true)

    const postgresFields = serviceConfigFields({ serviceType: 'postgresql' })
    expect(serviceConfigFieldVisible(postgresFields.find((field) => field.key === 'primary.persistence.size')!, {
      architecture: 'standalone',
      'primary.persistence.enabled': false,
    })).toBe(true)
    expect(serviceConfigFieldVisible(postgresFields.find((field) => field.key === 'persistence.size')!, {
      architecture: 'ha-cluster',
      'persistence.enabled': false,
    })).toBe(true)
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
      'replica.replicaCount',
      'sentinel.masterSet',
      'cluster.nodes',
      'cluster.replicas',
      'persistence.enabled',
      'persistence.size',
      'master.persistence.enabled',
      'master.persistence.size',
      'replica.persistence.enabled',
      'replica.persistence.size',
    ])
    expect(serviceConfigFields({ serviceType: 'redis' }).find((field) => field.key === 'architecture')?.options?.map((option) => option.label)).toEqual(['Redis 单节点', 'Redis 主从复制', 'Redis Sentinel', 'Redis Cluster'])

    expect(mysql.kind).toBe('database')
    expect(serviceConfigFields({ serviceType: 'mysql' }).map((field) => field.key)).toEqual([
      'architecture',
      'secondary.replicaCount',
      'replicaCount',
      'primary.persistence.enabled',
      'primary.persistence.size',
      'secondary.persistence.enabled',
      'secondary.persistence.size',
      'persistence.enabled',
      'persistence.size',
    ])
    expect(serviceConfigFields({ serviceType: 'mysql' }).map((field) => field.key)).not.toContain('sentinel.enabled')
    expect(mysql.showTopology).toBe(true)
    expect(serviceConfigFields({ serviceType: 'postgresql' }).find((field) => field.key === 'architecture')?.options?.map((option) => option.label)).toEqual(['单主库', '主库 + 只读副本', 'PostgreSQL HA 集群 (repmgr + Pgpool)'])

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
    expect(git.showDeploymentConfig).toBe(true)
    expect(git.showConnectionBindings).toBe(false)
    expect(git.showTopology).toBe(true)
    expect(git.showWorkspaceSummary).toBe(true)
    expect(git.workspaceTitle).toBe('Gitea 仓库配置')
    expect(serviceConfigFields({ serviceType: 'git' }).map((field) => field.key)).toEqual(['replicaCount', 'persistence.enabled', 'persistence.size'])
  })

  it('exposes real product-specific Helm values for tools instead of a generic tool drawer', () => {
    expect(serviceConfigFields({ serviceType: 'gitea' }).map((field) => field.key)).toEqual(['replicaCount', 'persistence.enabled', 'persistence.size'])
    expect(serviceConfigFields({ serviceType: 'jenkins' }).map((field) => field.key)).toEqual(['controller.numExecutors', 'controller.javaOpts', 'persistence.enabled', 'persistence.size'])
    expect(serviceConfigFields({ serviceType: 'argocd' }).map((field) => field.key)).toEqual(['controller.replicas', 'server.replicas', 'repoServer.replicas', 'applicationSet.replicas'])
    expect(serviceConfigFields({ serviceType: 'loki' }).map((field) => field.key)).toEqual(['loki.replicas', 'loki.persistence.enabled', 'loki.persistence.size'])
    expect(serviceConfigFields({ serviceType: 'docker-registry' }).map((field) => field.key)).toEqual(['replicaCount', 'persistence.enabled', 'persistence.size', 'deleteEnabled'])
    expect(serviceConfigFields({ serviceType: 'harbor' }).map((field) => field.key)).toEqual([
      'core.replicas',
      'portal.replicas',
      'registry.replicas',
      'jobservice.replicas',
      'trivy.enabled',
      'trivy.replicas',
      'persistence.enabled',
      'persistence.persistentVolumeClaim.registry.size',
      'persistence.persistentVolumeClaim.database.size',
      'persistence.persistentVolumeClaim.redis.size',
      'persistence.persistentVolumeClaim.jobservice.jobLog.size',
      'persistence.persistentVolumeClaim.trivy.size',
    ])
    expect(serviceConfigFields({ serviceType: 'prometheus-grafana' }).map((field) => field.key)).toEqual([
      'prometheus.prometheusSpec.replicas',
      'prometheus.prometheusSpec.shards',
      'grafana.persistence.enabled',
      'grafana.persistence.size',
    ])

    const harborValues = serviceConfigValuesFromForm('harbor', {
      'core.replicas': 2,
      'portal.replicas': 2,
      'registry.replicas': 2,
      'jobservice.replicas': 1,
      'trivy.enabled': true,
      'trivy.replicas': 1,
      'persistence.enabled': true,
      'persistence.persistentVolumeClaim.registry.size': '20Gi',
      'persistence.persistentVolumeClaim.database.size': '5Gi',
      'persistence.persistentVolumeClaim.redis.size': '2Gi',
      'persistence.persistentVolumeClaim.jobservice.jobLog.size': '2Gi',
      'persistence.persistentVolumeClaim.trivy.size': '10Gi',
      'fake.mode': 'cluster',
    })

    expect(harborValues).toMatchObject({
      'core.replicas': '2',
      'portal.replicas': '2',
      'registry.replicas': '2',
      'jobservice.replicas': '1',
      'trivy.enabled': 'true',
      'trivy.replicas': '1',
      'persistence.enabled': 'true',
      'persistence.persistentVolumeClaim.registry.size': '20Gi',
      'persistence.persistentVolumeClaim.database.size': '5Gi',
      'persistence.persistentVolumeClaim.redis.size': '2Gi',
      'persistence.persistentVolumeClaim.jobservice.jobLog.size': '2Gi',
      'persistence.persistentVolumeClaim.trivy.size': '10Gi',
    })
    expect(harborValues).not.toHaveProperty('fake.mode')
    expect(harborValues).not.toHaveProperty('cluster.enabled')
  })

  it('chooses product-specific profiles from template evidence instead of only serviceType', () => {
    const harborService = {
      serviceType: 'registry',
      templateName: 'Harbor (官方)',
      chartName: 'harbor',
      template: { s3Key: 'charts/harbor.tar.gz' },
    }
    const registryService = {
      serviceType: 'registry',
      templateName: 'Docker Registry v2',
      template: { s3Key: 'charts/registry.tar.gz' },
    }

    expect(serviceConfigType(harborService)).toBe('harbor')
    expect(serviceConfigProfile(harborService).workspaceTitle).toBe('Harbor 配置')
    expect(serviceConfigFields(harborService).map((field) => field.key)).toContain('persistence.persistentVolumeClaim.registry.size')
    expect(serviceConfigFields(harborService).map((field) => field.key)).not.toContain('deleteEnabled')

    expect(serviceConfigType(registryService)).toBe('docker-registry')
    expect(serviceConfigProfile(registryService).workspaceTitle).toBe('Docker Registry 配置')
    expect(serviceConfigFields(registryService).map((field) => field.key)).toContain('deleteEnabled')
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

  it('maps advanced database architectures to values backed by bundled HA charts', () => {
    const mysql = serviceConfigValuesFromForm('mysql', {
      architecture: 'dual-master',
      replicaCount: 2,
      'primary.persistence.enabled': true,
      'primary.persistence.size': '20Gi',
      'secondary.persistence.enabled': true,
      'secondary.persistence.size': '20Gi',
      'persistence.enabled': true,
      'persistence.size': '30Gi',
    })
    const postgres = serviceConfigValuesFromForm('postgresql', {
      architecture: 'ha-cluster',
      'readReplicas.replicaCount': 3,
      'primary.persistence.enabled': true,
      'primary.persistence.size': '20Gi',
      'readReplicas.persistence.enabled': true,
      'readReplicas.persistence.size': '20Gi',
      'postgresql.replicaCount': 3,
      'pgpool.replicaCount': 2,
      'persistence.enabled': true,
      'persistence.size': '40Gi',
    })

    expect(serviceConfigFormFromInstallation({ serviceType: 'mysql', values: JSON.stringify({ 'paap.architecture': 'galera', replicaCount: 3 }) }).architecture).toBe('galera')
    expect(serviceConfigFormFromInstallation({ serviceType: 'postgresql', values: JSON.stringify({ 'paap.architecture': 'ha-cluster', 'postgresql.replicaCount': 3 }) }).architecture).toBe('ha-cluster')
    expect(mysql).toEqual({
      'paap.architecture': 'dual-master',
      replicaCount: '2',
      'persistence.enabled': 'true',
      'persistence.size': '30Gi',
    })
    expect(postgres).toEqual({
      'paap.architecture': 'ha-cluster',
      'postgresql.replicaCount': '3',
      'pgpool.replicaCount': '2',
      'persistence.enabled': 'true',
      'persistence.size': '40Gi',
    })
    expect(serviceConfigFields({ serviceType: 'mysql' }).find((field) => field.key === 'architecture')?.options?.map((option) => option.value)).toEqual(['standalone', 'replication', 'dual-master', 'galera'])
    expect(serviceConfigFields({ serviceType: 'postgresql' }).find((field) => field.key === 'architecture')?.options?.map((option) => option.value)).toEqual(['standalone', 'replication', 'ha-cluster'])
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

  it('uses runtime workload topology before workspace business objects', () => {
    const topology = serviceTopologyFromWorkspace(
      {
        serviceType: 'postgresql',
        status: 'running',
        runtimeConfig: {
          namespace: 'app-dev-postgresql',
          workloadName: 'app-dev-postgresql',
          workloadKind: 'StatefulSet',
          container: 'postgresql',
          image: 'bitnamilegacy/postgresql:17',
          replicas: 1,
        },
      },
      [
        { name: 'database-connection', type: 'Connection', status: 'Ready', description: 'Database connection is healthy.' },
        { name: 'postgres', type: 'Database', status: 'Ready', description: 'Database catalog' },
      ],
    )

    expect(topology.summaryRows).toEqual([
      { label: '实例', value: '1' },
      { label: '类型', value: 'StatefulSet' },
    ])
    expect(topology.nodes).toHaveLength(1)
    expect(topology.nodes[0].name).toBe('app-dev-postgresql')
    expect(topology.nodes[0].detail).toContain('Container postgresql')
    expect(JSON.stringify(topology)).not.toContain('Database catalog')
  })
})
