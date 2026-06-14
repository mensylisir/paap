import { describe, expect, it } from 'vitest'
import { buildRecentEvents, countServiceIssues, effectiveEnvironmentStatus, environmentResourceSummary, environmentStatusLabel, hasErrorEnvironment, sumApplicationResources } from './appSummary'

describe('app summary helpers', () => {
  it('detects environments with explicit errors', () => {
    expect(hasErrorEnvironment([{ name: 'dev', status: 'running' }])).toBe(false)
    expect(hasErrorEnvironment([{ name: 'dev', status: 'error' }])).toBe(true)
    expect(hasErrorEnvironment([{ name: 'dev', status: 'running', errorMessage: 'ImagePullBackOff' }])).toBe(true)
  })

  it('builds recent events from real environment state', () => {
    const events = buildRecentEvents([
      { name: 'dev', status: 'running', toolCount: 2, componentCount: 1 },
      { name: 'staging', status: 'creating' },
      { name: 'prod', status: 'running', errorMessage: 'helm install failed' },
    ])

    expect(events).toEqual([
      'dev 已运行，2 个工具，1 个组件',
      'staging 正在创建中',
      'prod：helm install failed',
    ])
  })

  it('surfaces service-level problems before generic running events', () => {
    const events = buildRecentEvents([
      {
        name: 'staging',
        status: 'running',
        toolCount: 2,
        componentCount: 1,
        services: [
          { serviceType: 'deploy', status: 'running' },
          { serviceType: 'registry', status: 'failed', errorMessage: 'ImagePullBackOff' },
        ],
      },
    ])

    expect(events[0]).toContain('staging registry 异常')
    expect(events[0]).toContain('ImagePullBackOff')
  })

  it('counts installing and failed services as issues', () => {
    expect(countServiceIssues([
      { name: 'dev', services: [{ serviceType: 'deploy', status: 'running' }] },
      { name: 'staging', services: [{ serviceType: 'registry', status: 'failed' }, { serviceType: 'git', status: 'installing' }] },
    ])).toBe(2)
  })

  it('splits platform tools from middleware and data services', () => {
    const env = {
      name: 'staging',
      status: 'running',
      toolCount: 8,
      componentCount: 2,
      services: [
        { serviceType: 'ci', status: 'running' },
        { serviceType: 'deploy', status: 'running' },
        { serviceType: 'git', status: 'running' },
        { serviceType: 'log', status: 'running' },
        { serviceType: 'monitor', status: 'running' },
        { serviceType: 'registry', status: 'running' },
        { serviceType: 'postgresql', status: 'running' },
        { serviceType: 'redis', status: 'running' },
      ],
    }

    expect(environmentResourceSummary(env)).toEqual({ toolCount: 6, middlewareCount: 2, componentCount: 2 })
    expect(sumApplicationResources({ environments: [env] })).toEqual({ toolCount: 6, middlewareCount: 2, componentCount: 2 })
  })

  it('does not present a populated environment as empty when the stored status is stale', () => {
    const env = {
      name: 'dev',
      status: 'empty',
      componentCount: 2,
      services: [
        { serviceType: 'git', status: 'running' },
        { serviceType: 'postgresql', status: 'running' },
        { serviceType: 'redis', status: 'running' },
      ],
    }

    expect(effectiveEnvironmentStatus(env)).toBe('running')
    expect(environmentStatusLabel(effectiveEnvironmentStatus(env))).toBe('运行中')
    expect(buildRecentEvents([env])[0]).toBe('dev 已运行，1 个工具，2 个中间件，2 个组件')
  })
})
