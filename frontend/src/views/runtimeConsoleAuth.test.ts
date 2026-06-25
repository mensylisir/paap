import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { RUNTIME_CONSOLE_PROTOCOL, runtimeConsoleWebSocketProtocols } from './runtimeConsoleAuth'

describe('runtime console websocket auth', () => {
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

  it('passes the stored auth token through websocket subprotocols', () => {
    localStorage.setItem('paap_token', 'signed.jwt.token')

    expect(runtimeConsoleWebSocketProtocols()).toEqual([RUNTIME_CONSOLE_PROTOCOL, 'signed.jwt.token'])
  })

  it('omits websocket subprotocols when no token is stored', () => {
    expect(runtimeConsoleWebSocketProtocols()).toBeUndefined()
  })
})
