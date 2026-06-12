import { ref, onMounted, onUnmounted } from 'vue'

export interface StatusChange {
  type: string
  resource: string
  name: string
  namespace: string
  phase: string
}

export function useWebSocket() {
  const connected = ref(false)
  const lastMessage = ref<StatusChange | null>(null)
  let ws: WebSocket | null = null
  let reconnectTimer: number | null = null

  function connect() {
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${location.host}/ws`
    ws = new WebSocket(url)

    ws.onopen = () => {
      connected.value = true
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as StatusChange
        lastMessage.value = data
      } catch (e) {
        console.error('WebSocket message parse error:', e)
      }
    }

    ws.onclose = () => {
      connected.value = false
      reconnectTimer = window.setTimeout(connect, 3000)
    }

    ws.onerror = (err) => {
      console.error('WebSocket error:', err)
      ws?.close()
    }
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (ws) {
      ws.close()
      ws = null
    }
  }

  onMounted(connect)
  onUnmounted(disconnect)

  return { connected, lastMessage }
}
