import { describe, expect, it } from 'vitest'
import { buildCapabilityTabs, buildEnvironmentCapabilityTabs, capabilityServiceInstanceLabel, requiredEnvironmentCapabilities, serviceCapability } from './envCapabilities'

describe('environment capabilities', () => {
  it('names infrastructure capability tabs by concrete middleware category', () => {
    expect(serviceCapability({ serviceType: 'redis' }).label).toBe('缓存')
    expect(serviceCapability({ serviceType: 'redis' }).key).toBe('cache')
    expect(serviceCapability({ serviceType: 'rabbitmq' }).label).toBe('消息队列')
    expect(serviceCapability({ serviceType: 'kafka' }).key).toBe('message-queue')

    const tabs = buildCapabilityTabs([
      { serviceType: 'redis' },
      { serviceType: 'rabbitmq' },
      { serviceType: 'kafka' },
      { serviceType: 'postgresql' },
    ])

    expect(tabs.map((tab) => `${tab.key}:${tab.label}:${tab.count}`)).toEqual([
      'databases:数据库:1',
      'cache:缓存:1',
      'message-queue:消息队列:2',
    ])
  })

  it('labels multiple database instances by concrete engine and installation state', () => {
    const templates = [
      { type: 'postgresql', name: 'PostgreSQL' },
      { type: 'mysql', name: 'MySQL' },
    ]

    expect(capabilityServiceInstanceLabel({
      serviceType: 'postgresql',
      serviceName: 'postgres',
      status: 'running',
    }, templates)).toBe('PostgreSQL · postgres · 运行中')

    expect(capabilityServiceInstanceLabel({
      serviceType: 'mysql',
      serviceName: 'mysql',
      status: 'installing',
    }, templates)).toBe('MySQL · mysql · 安装中')
  })

  it('keeps the five required environment foundation capabilities visible when missing', () => {
    expect(requiredEnvironmentCapabilities.map((item) => item.key)).toEqual([
      'code-repository',
      'image-registry',
      'continuous-deployment',
      'monitoring-center',
      'logging-center',
    ])

    const tabs = buildEnvironmentCapabilityTabs([
      { serviceType: 'git' },
      { serviceType: 'postgresql' },
    ])

    expect(tabs.map((tab) => `${tab.key}:${tab.count}`)).toEqual([
      'code-repository:1',
      'image-registry:0',
      'continuous-deployment:0',
      'monitoring-center:0',
      'logging-center:0',
      'databases:1',
    ])
  })
})
