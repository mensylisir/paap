import { describe, expect, it } from 'vitest'
import { mergeCreatedCanvasResource, selectCreatedCanvasResource } from './envDetailCanvasResources'

describe('envDetailCanvasResources', () => {
  it('keeps a service draft returned by the create API when refresh does not include it yet', () => {
    const refreshed = [{ id: 1, serviceType: 'redis', status: 'running' }]
    const created = { id: 2, serviceType: 'rabbitmq', status: 'draft' }

    expect(mergeCreatedCanvasResource(refreshed, created)).toEqual([
      { id: 1, serviceType: 'redis', status: 'running' },
      { id: 2, serviceType: 'rabbitmq', status: 'draft' },
    ])
    expect(selectCreatedCanvasResource(refreshed, created, 'rabbitmq')).toEqual(created)
  })

  it('uses the refreshed resource when the created resource is already present', () => {
    const created = { id: 2, serviceType: 'rabbitmq', status: 'draft' }
    const refreshed = [{ id: 2, serviceType: 'rabbitmq', status: 'running', namespace: 'dev-rabbitmq' }]

    expect(mergeCreatedCanvasResource(refreshed, created)).toEqual(refreshed)
    expect(selectCreatedCanvasResource(refreshed, created, 'rabbitmq')).toEqual(refreshed[0])
  })
})
