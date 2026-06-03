import { ref, onMounted, onUnmounted } from 'vue'
import type { Flow } from '@/types'

export function useWebSocket() {
  const flows = ref<Flow[]>([])
  const ws = ref<WebSocket | null>(null)
  const isConnected = ref(false)

  function connect() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws`

    ws.value = new WebSocket(wsUrl)

    ws.value.onopen = () => {
      isConnected.value = true
    }

    ws.value.onmessage = (event) => {
      try {
        const flow: Flow = JSON.parse(event.data)
        flows.value.unshift(flow)
      } catch (e) {
        console.error('Failed to parse flow:', e)
      }
    }

    ws.value.onclose = () => {
      isConnected.value = false
      setTimeout(connect, 3000)
    }

    ws.value.onerror = () => {
      ws.value?.close()
    }
  }

  function disconnect() {
    ws.value?.close()
  }

  onMounted(connect)
  onUnmounted(disconnect)

  return {
    flows,
    isConnected
  }
}
