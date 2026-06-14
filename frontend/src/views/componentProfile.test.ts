import { describe, expect, it } from 'vitest'
import {
  buildComponentProfile,
  componentDrawerBlueprint,
  componentConfigPresets,
  componentConfigKeySuggestions,
  componentFrameworkLabel,
} from './componentProfile'

describe('componentProfile', () => {
  it('uses a web-entry profile for unknown frontend implementations', () => {
    const profile = buildComponentProfile({
      component: {
        name: 'web',
        type: 'frontend',
        image: 'registry.local/orders-web:v1',
        config: {
          env: [{ name: 'VITE_API_BASE_URL', value: 'http://orders-api' }],
        },
      },
    })

    expect(profile.webEntry).toBe(true)
    expect(profile.apiService).toBe(false)
    expect(profile.capabilityLabels).toContain('Web 入口')
    expect(componentConfigKeySuggestions(profile).map((item) => item.key)).toEqual(expect.arrayContaining([
      'BACKEND_URL',
      'API_BASE_URL',
      'VITE_API_BASE_URL',
      'NEXT_PUBLIC_API_URL',
    ]))
  })

  it('detects Spring Boot backends with database and Redis dependencies from generated config', () => {
    const profile = buildComponentProfile({
      component: {
        name: 'orders-api',
        type: 'custom',
        image: 'eclipse-temurin:21-jre',
      },
      form: {
        framework: 'auto',
        bindings: [
          { targetName: 'orders-postgresql', targetType: 'postgresql', role: 'database' },
          { targetName: 'orders-redis', targetType: 'redis', role: 'cache' },
        ],
        configMaps: [{
          name: 'orders-api-config',
          data: {
            'application-paap.yml': 'spring:\n  datasource:\n    url: jdbc:postgresql://orders-postgresql:5432/postgres\n',
          },
        }],
        files: [{
          name: 'spring-paap-config',
          configMapName: 'orders-api-config',
          key: 'application-paap.yml',
          mountPath: '/etc/paap/application-paap.yml',
        }],
      },
    })

    expect(profile.framework).toBe('springboot')
    expect(profile.apiService).toBe(true)
    expect(profile.hasRuntimeDependencies).toBe(true)
    expect(profile.capabilityLabels).toEqual(expect.arrayContaining(['API 服务', '数据库客户端', 'Redis 客户端', '配置文件/敏感配置']))
    expect(profile.configSourceSummary).toBe('1 普通配置 / 1 配置文件')
    expect(componentConfigKeySuggestions(profile).map((item) => item.key)).toEqual(expect.arrayContaining([
      'SERVER_PORT',
      'SPRING_PROFILES_ACTIVE',
      'POSTGRES_HOST',
      'POSTGRES_PASSWORD',
      'REDIS_HOST',
      'REDIS_PASSWORD',
    ]))
  })

  it('keeps an opaque custom workload generic until runtime or user config provides evidence', () => {
    const profile = buildComponentProfile({
      component: {
        name: 'worker',
        type: 'custom',
        image: 'registry.local/worker:v1',
      },
    })

    expect(profile.framework).toBe('unknown')
    expect(profile.webEntry).toBe(false)
    expect(profile.apiService).toBe(false)
    expect(profile.hasRuntimeDependencies).toBe(false)
    expect(componentConfigKeySuggestions(profile).map((item) => item.key)).toEqual(['PORT', 'LOG_LEVEL'])
  })

  it('keeps unknown components configurable through generic presets and discovered keys', () => {
    const profile = buildComponentProfile({
      component: {
        name: 'worker',
        type: 'custom',
        image: 'registry.local/worker:v1',
        runtimeConfig: {
          env: [
            { name: 'PORT', value: '8080' },
            { name: 'API_TOKEN', secretName: 'worker-secret', secretKey: 'API_TOKEN' },
          ],
        },
      },
    })

    expect(profile.framework).toBe('unknown')
    expect(profile.configSourceSummary).toBe('2 运行参数')
    expect(componentConfigPresets(profile).map((item) => item.key)).toContain('generic-runtime')
    expect(componentConfigKeySuggestions(profile).map((item) => item.key)).toEqual(expect.arrayContaining(['PORT', 'LOG_LEVEL']))
    expect(profile.discoveredConfigKeys).toEqual(expect.arrayContaining([
      expect.objectContaining({ name: 'PORT', source: '环境变量', sensitive: false }),
      expect.objectContaining({ name: 'API_TOKEN', source: '敏感配置', sensitive: true, refKind: 'secret' }),
    ]))

    const blueprint = componentDrawerBlueprint(profile)
    expect(blueprint.mode).toBe('generic-runtime')
    expect(blueprint.tabs.map((item) => item.key)).toEqual([
      'deploy',
      'variables',
      'runtime',
      'logs',
      'console',
      'settings',
    ])
  })

  it('discovers envFrom and mounted config files for unknown runtime components', () => {
    const profile = buildComponentProfile({
      component: {
        name: 'external-api',
        type: 'custom',
        image: 'registry.local/external-api:v7',
        runtimeConfig: {
          envFrom: [
            { kind: 'configMap', name: 'api-config' },
            { kind: 'secret', name: 'api-secret' },
          ],
          configMaps: [
            { name: 'api-config', keys: ['FEATURE_FLAG', 'application.yml'] },
          ],
          secrets: [
            { name: 'api-secret', keys: ['DATABASE_PASSWORD'] },
          ],
          files: [
            {
              kind: 'configMap',
              objectName: 'api-config',
              key: 'application.yml',
              mountPath: '/etc/app/application.yml',
            },
            {
              kind: 'secret',
              objectName: 'api-secret',
              key: 'tls.key',
              mountPath: '/etc/tls/tls.key',
            },
          ],
        },
      },
    })

    expect(profile.capabilityLabels).toContain('配置文件/敏感配置')
    expect(profile.configSourceSummary).toBe('1 普通配置 / 1 敏感配置 / 2 配置文件')
    expect(profile.discoveredConfigKeys).toEqual(expect.arrayContaining([
      expect.objectContaining({ name: 'FEATURE_FLAG', source: '普通配置', refKind: 'configMap' }),
      expect.objectContaining({ name: 'DATABASE_PASSWORD', source: '敏感配置', sensitive: true, refKind: 'secret' }),
      expect.objectContaining({ name: 'application.yml', source: '配置文件', refKind: 'configMap', asFile: true }),
      expect.objectContaining({ name: 'tls.key', source: '敏感配置文件', sensitive: true, refKind: 'secret', asFile: true }),
    ]))
  })

  it('infers runtime dependencies from configuration file content without bindings', () => {
    const profile = buildComponentProfile({
      component: {
        name: 'opaque-api',
        type: 'custom',
        image: 'registry.local/opaque-api:v2',
        config: {
          configMaps: [{
            name: 'opaque-api-config',
            data: {
              'application.yml': [
                'server:',
                '  port: 8080',
                'spring:',
                '  datasource:',
                '    url: jdbc:postgresql://orders-postgresql:5432/orders',
                '  data:',
                '    redis:',
                '      host: orders-redis',
                '  rabbitmq:',
                '    host: orders-rabbitmq',
                'storage:',
                '  endpoint: s3://orders-artifacts',
              ].join('\n'),
            },
          }],
          files: [{
            configMapName: 'opaque-api-config',
            key: 'application.yml',
            mountPath: '/workspace/config/application.yml',
          }],
        },
      },
    })

    expect(profile.framework).toBe('springboot')
    expect(profile.apiService).toBe(true)
    expect(profile.capabilityLabels).toEqual(expect.arrayContaining([
      '数据库客户端',
      'Redis 客户端',
      '消息队列客户端',
      '对象存储客户端',
      '配置文件/敏感配置',
    ]))
    expect(componentDrawerBlueprint(profile).tabs.map((item) => item.key)).toEqual(['deploy', 'variables', 'runtime', 'logs', 'console', 'settings'])
  })

  it('lets an explicit framework declaration override weak image hints', () => {
    const profile = buildComponentProfile({
      component: {
        name: 'api',
        type: 'backend',
        image: 'node:22-alpine',
      },
      form: {
        framework: 'go',
      },
    })

    expect(profile.framework).toBe('go')
    expect(componentFrameworkLabel(profile.framework)).toBe('Go')
  })

  it('returns framework and dependency presets for recognized component capabilities', () => {
    const profile = buildComponentProfile({
      component: {
        name: 'orders-api',
        type: 'backend',
      },
      form: {
        framework: 'springboot',
        bindings: [
          { targetName: 'orders-db', targetType: 'postgresql', role: 'database' },
          { targetName: 'orders-rabbitmq', targetType: 'rabbitmq', role: 'message-queue' },
        ],
      },
    })

    const presets = componentConfigPresets(profile)
    expect(presets.map((item) => item.key)).toEqual(expect.arrayContaining([
      'springboot-runtime',
      'database-client',
      'message-client',
    ]))
    expect(presets.find((item) => item.key === 'message-client')?.keys).toEqual(expect.arrayContaining([
      'RABBITMQ_URL',
      'KAFKA_BROKERS',
    ]))

    const blueprint = componentDrawerBlueprint(profile)
    expect(blueprint.mode).toBe('api-service')
    expect(blueprint.configStrategyLabel).toContain('运行依赖')
    expect(blueprint.tabs.map((item) => item.key)).toEqual(['deploy', 'variables', 'runtime', 'logs', 'console', 'settings'])
    expect(blueprint.tabs.map((item) => item.key)).not.toContain('api')
  })

  it('builds drawer blueprints from component evidence rather than component names', () => {
    const frontend = buildComponentProfile({
      component: {
        name: 'whatever',
        type: 'frontend',
        image: 'registry.local/opaque:v1',
      },
    })
    const middleware = buildComponentProfile({
      component: {
        name: 'opaque-runtime',
        type: 'middleware',
        image: 'registry.local/custom-daemon:v1',
      },
    })

    expect(componentDrawerBlueprint(frontend).tabs.map((item) => item.key)).toEqual(['deploy', 'variables', 'runtime', 'logs', 'console', 'settings'])
    expect(componentDrawerBlueprint(frontend).tabs.map((item) => item.key)).not.toContain('dependencies')
    expect(componentDrawerBlueprint(middleware).mode).toBe('middleware-workload')
    expect(componentDrawerBlueprint(middleware).tabs.map((item) => item.key)).toEqual(['deploy', 'variables', 'runtime', 'logs', 'console', 'settings'])
  })

  it('offers a real Nginx API proxy preset for web entry components', () => {
    const profile = buildComponentProfile({
      component: {
        name: 'web',
        type: 'frontend',
        image: 'nginx:alpine',
      },
      form: {
        framework: 'nginx',
      },
    })

    const preset = componentConfigPresets(profile).find((item) => item.key === 'nginx-api-proxy')
    expect(preset).toMatchObject({
      label: 'Nginx API 代理',
      framework: 'nginx',
      keys: ['default.conf', 'BACKEND_URL'],
    })
  })
})
