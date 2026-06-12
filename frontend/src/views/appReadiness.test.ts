import { describe, expect, it } from 'vitest'
import { buildEnvironmentReadiness, serviceIsReady } from './appReadiness'

describe('appReadiness', () => {
  it('treats only running service instances as ready', () => {
    expect(serviceIsReady({ serviceType: 'deploy', status: 'running' })).toBe(true)
    expect(serviceIsReady({ serviceType: 'deploy', status: 'installing' })).toBe(false)
    expect(serviceIsReady({ serviceType: 'deploy', status: 'failed' })).toBe(false)
  })

  it('builds deploy, ci, and observability readiness from services and components', () => {
    const result = buildEnvironmentReadiness(
      { id: 1, name: '测试环境', identifier: 'staging', status: 'running' },
      [
        { serviceType: 'deploy', status: 'running' },
        { serviceType: 'git', status: 'running' },
        { serviceType: 'registry', status: 'running' },
        { serviceType: 'ci', status: 'installing' },
        { serviceType: 'monitor', status: 'failed' },
        { serviceType: 'log', status: 'running' },
      ],
      [{ id: 1 }],
    )

    expect(result.deploy.ready).toBe(true)
    expect(result.ci.ready).toBe(false)
    expect(result.ci.missing).toEqual(['ci'])
    expect(result.observability.ready).toBe(false)
    expect(result.observability.missing).toEqual(['monitor'])
    expect(result.componentCount).toBe(1)
  })

  it('requires the lightweight registry service for image registry readiness', () => {
    const result = buildEnvironmentReadiness(
      { id: 1, name: 'dev', identifier: 'dev', status: 'running' },
      [
        { serviceType: 'git', status: 'running' },
        { serviceType: 'registry', status: 'running' },
      ],
      [],
    )

    expect(result.ci.missing).toContain('ci')
    expect(result.ci.missing).not.toContain('registry')
  })
})
