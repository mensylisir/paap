export const RUNTIME_CONSOLE_PROTOCOL = 'paap-runtime-console'

export function runtimeConsoleWebSocketProtocols() {
  if (typeof localStorage === 'undefined') return undefined
  const token = localStorage.getItem('paap_token') || ''
  return token ? [RUNTIME_CONSOLE_PROTOCOL, token] : undefined
}
