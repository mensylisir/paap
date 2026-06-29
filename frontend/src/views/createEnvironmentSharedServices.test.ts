import { describe, expect, it } from 'vitest'
import { buildSharedCapabilityPayload, emptyEnvironmentForm } from './createEnvironmentSharedServices'

describe('createEnvironmentSharedServices', () => {
  it('builds an empty environment form with shared resources unselected', () => {
    expect(emptyEnvironmentForm(7)).toMatchObject({
      mode: 'empty',
      templateId: '7',
      sharedResourceIds: [],
    })
  })

  it('converts selected shared resources to environment capabilities', () => {
    const payload = buildSharedCapabilityPayload(['11'], [
      { id: 10, capability: 'cache', provider: 'redis', serviceType: 'redis', serviceName: 'shared-redis' },
      { id: 11, capability: 'database', provider: 'postgresql', serviceType: 'postgresql', serviceName: 'shared-postgres' },
    ])

    expect(payload).toEqual([{
      source: 'shared',
      capability: 'database',
      capabilityKey: 'shared-database-11',
      provider: 'postgresql',
      serviceType: 'postgresql',
      refServiceId: 11,
    }])
  })
})
