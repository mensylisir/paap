import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from './client'

describe('api client', () => {
  beforeEach(() => {
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key: string) => store.get(key) ?? null),
      setItem: vi.fn((key: string, value: string) => { store.set(key, value) }),
      removeItem: vi.fn((key: string) => { store.delete(key) }),
      clear: vi.fn(() => { store.clear() }),
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
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

  it('sends the stored auth token as a bearer header', async () => {
    localStorage.setItem('paap_token', 'signed.jwt.token')
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({ data: { username: 'admin' } }),
    } as Response)

    await api.me()

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/auth/me', expect.objectContaining({
      method: 'GET',
      headers: expect.objectContaining({
        Authorization: 'Bearer signed.jwt.token',
      }),
    }))
  })

  it('does not reuse completed GET responses across auth token changes', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { id: 1, owner: 'first-token' } }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { id: 1, owner: 'second-token' } }),
      } as Response)

    localStorage.setItem('paap_token', 'first.jwt.token')
    const first = await api.getApp(99)
    localStorage.setItem('paap_token', 'second.jwt.token')
    const second = await api.getApp(99)

    expect(first).toEqual({ data: { id: 1, owner: 'first-token' } })
    expect(second).toEqual({ data: { id: 1, owner: 'second-token' } })
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/applications/99', expect.objectContaining({
      headers: expect.objectContaining({ Authorization: 'Bearer first.jwt.token' }),
    }))
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/applications/99', expect.objectContaining({
      headers: expect.objectContaining({ Authorization: 'Bearer second.jwt.token' }),
    }))
  })
})
