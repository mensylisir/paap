import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './client'

describe('api client', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('deduplicates concurrent GET requests for the same resource', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({ data: { id: 4 } }),
    } as Response)

    const [first, second] = await Promise.all([
      api.getEnv(4),
      api.getEnv(4),
    ])

    expect(first).toEqual({ data: { id: 4 } })
    expect(second).toEqual({ data: { id: 4 } })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/environments/4', expect.objectContaining({ method: 'GET' }))
  })

  it('reuses a short-lived completed GET response during page boot', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({ data: { id: 1 } }),
    } as Response)

    await api.getApp(1)
    await api.getApp(1)

    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('does not deduplicate mutating requests', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({ ok: true }),
    } as Response)

    await Promise.all([
      api.createApp({ name: 'a' }),
      api.createApp({ name: 'a' }),
    ])

    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('calls environment adoptable resource endpoints', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({ data: [] }),
    } as Response)

    await api.listAdoptableResources(3)
    await api.adoptResource(3, { key: 'billing-dev/deployment/api' })

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/environments/3/adoptable-resources', expect.objectContaining({ method: 'GET' }))
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/environments/3/adoptable-resources', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ key: 'billing-dev/deployment/api' }),
    }))
  })
})
