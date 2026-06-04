<template>
  <div class="flow-table-page" :class="{ compact: selectedFlow }">
    <div class="page-header">
      <h1>Flows</h1>
      <div class="header-actions">
        <input
          v-model="searchQuery"
          class="input"
          placeholder="Search flows..."
          @input="debouncedFetch"
        />
        <select v-model="serviceFilter" class="select" @change="fetchFlows">
          <option value="">All services</option>
          <option v-for="service in services" :key="service.id" :value="String(service.id)">
            {{ service.name }} :{{ service.port }}
          </option>
        </select>
        <label class="filter-check">
          <input v-model="showBanned" type="checkbox" @change="fetchFlows" />
          Banned
        </label>
        <label class="filter-check">
          <input v-model="showChecker" type="checkbox" @change="fetchFlows" />
          Checker
        </label>
        <select
          v-model="pageSize"
          class="select"
          @change="fetchFlows"
        >
          <option :value="25">25 per page</option>
          <option :value="50">50 per page</option>
          <option :value="100">100 per page</option>
        </select>
      </div>
    </div>

    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th v-if="!selectedFlow">
              <input type="checkbox" class="checkbox" :checked="allSelected" @change="toggleAll" />
            </th>
            <th v-if="!selectedFlow">Time</th>
            <th>Direction</th>
            <th v-if="!selectedFlow">Proto</th>
            <th v-if="!selectedFlow">Status</th>
            <th v-if="!selectedFlow">Response</th>
            <th v-if="!selectedFlow">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="flow in flows"
            :key="flow.id"
            class="flow-row"
            :class="{
              stable: flow.stability_pct >= 70,
              banned: flow.banned,
              selected: selectedFlow?.id === flow.id,
              'negative-response': !flow.banned && !isPositiveResponse(flow.response_code)
            }"
            @click="$emit('open-flow', flow)"
          >
            <td v-if="!selectedFlow" @click.stop>
              <input type="checkbox" class="checkbox" :checked="selected.has(flow.id)" @change="toggleSelect(flow.id)" />
            </td>
            <td v-if="!selectedFlow" class="text-muted">{{ formatTime(flow.created_at) }}</td>
            <td>{{ displayDirection(flow) }}</td>
            <td v-if="!selectedFlow">
              <span class="badge badge-outline">{{ flow.proto }}</span>
            </td>
            <td v-if="!selectedFlow">
              <span class="badge" :class="flow.stability_pct >= 70 ? 'badge-success' : 'badge-warning'">{{ stabilityLabel(flow) }}</span>
              <span v-if="flow.checker" class="badge badge-primary">Checker</span>
              <span v-if="flow.banned" class="badge badge-destructive">Banned</span>
            </td>
            <td v-if="!selectedFlow">
              <span class="badge" :class="isPositiveResponse(flow.response_code) ? 'badge-success' : 'badge-warning'">
                {{ flow.response_code }}
              </span>
            </td>
            <td v-if="!selectedFlow" class="flow-actions" @click.stop>
              <button
                v-if="flow.response_code === 200 && !flow.banned"
                class="btn btn-sm btn-destructive"
                @click="$emit('open-word-picker', flow)"
              >
                Ban
              </button>
              <button
                v-else-if="flow.banned"
                class="btn btn-sm btn-outline"
                @click="unbanFlow(flow)"
              >
                Unban
              </button>
              <button
                class="btn btn-sm mirror-btn"
                :class="flow.mirrored ? 'btn-success' : 'btn-outline'"
                @click="toggleMirror(flow)"
              >
                {{ flow.mirrored ? 'Mirrored' : 'Mirror' }}
              </button>
            </td>
          </tr>
          <tr v-if="flows.length === 0">
            <td :colspan="selectedFlow ? 1 : 7" class="empty-state">No flows captured yet</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pagination">
      <button class="btn btn-sm btn-outline" :disabled="page <= 1" @click="page--; fetchFlows()">Previous</button>
      <span class="text-muted">Page {{ page }}</span>
      <button class="btn btn-sm btn-outline" :disabled="flows.length < pageSize" @click="page++; fetchFlows()">Next</button>
    </div>

    <div v-if="selected.size > 0" class="selection-bar">
      <span>{{ selected.size }} flow(s) selected</span>
      <button class="btn btn-sm btn-destructive" @click="banSelected">Ban Selected</button>
      <button class="btn btn-sm btn-ghost" @click="selected.clear()">Clear</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import api from '@/utils/api'
import type { Flow, Service } from '@/types'

const emit = defineEmits<{
  'open-flow': [flow: Flow]
  'open-word-picker': [flow: Flow]
}>()

defineProps<{ selectedFlow?: Flow | null }>()

const flows = ref<Flow[]>([])
const page = ref(1)
const pageSize = ref(50)
const searchQuery = ref('')
const serviceFilter = ref('')
const showBanned = ref(true)
const showChecker = ref(true)
const services = ref<Service[]>([])
const selected = ref(new Set<string>())
let debounceTimer: ReturnType<typeof setTimeout> | null = null
let ws: WebSocket | null = null

const allSelected = computed(() => flows.value.length > 0 && selected.value.size === flows.value.length)

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws`
  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    console.log('WebSocket connected')
  }

  ws.onmessage = (event) => {
    try {
      const flow: Flow = JSON.parse(event.data)
      // Add new flow to the beginning of the list
      flows.value.unshift(flow)
      // Keep the list within page size limits
      if (flows.value.length > pageSize.value) {
        flows.value = flows.value.slice(0, pageSize.value)
      }
    } catch (e) {
      console.error('Failed to parse flow:', e)
    }
  }

  ws.onclose = () => {
    console.log('WebSocket disconnected, reconnecting in 3s...')
    setTimeout(connectWebSocket, 3000)
  }

  ws.onerror = () => {
    ws?.close()
  }
}

async function fetchFlows() {
  try {
    const params: Record<string, string> = {
      page: String(page.value),
      size: String(pageSize.value),
    }
    if (searchQuery.value) {
      params.search = searchQuery.value
    }
    if (serviceFilter.value) {
      params.service_id = serviceFilter.value
    }
    if (!showBanned.value) {
      params.banned = 'false'
    }
    if (!showChecker.value) {
      params.checker = 'false'
    }
    const { data } = await api.get('/flows', { params })
    flows.value = data.flows
  } catch (e) {
    console.error('Failed to fetch flows:', e)
  }
}

async function fetchServices() {
  try {
    const { data } = await api.get('/services')
    services.value = data
  } catch (e) {
    console.error('Failed to fetch services:', e)
  }
}

function debouncedFetch() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    page.value = 1
    fetchFlows()
  }, 300)
}

function formatTime(ts: string) {
  return new Date(ts).toLocaleString()
}

function stabilityLabel(flow: Flow) {
  const pct = Math.round(flow.stability_pct || 0)
  const avg = Number(flow.avg_interval || 0)
  return `${pct}%/${avg > 0 ? avg.toFixed(1) : '—'}s`
}

function isPositiveResponse(code: number) {
  return code === 101 || (code >= 200 && code < 400)
}

function displayDirection(flow: Flow) {
  const uri = String(flow.raw_request?.uri || flow.raw_request?.url || '')
  if (uri) return `${flow.dst_port}${uri.startsWith('/') ? uri : `/${uri}`}`
  if (flow.destination) return flow.destination.replace(/^.*?:(\d+)/, '$1')
  return `${flow.dst_port}`
}

function toggleSelect(id: string) {
  if (selected.value.has(id)) {
    selected.value.delete(id)
  } else {
    selected.value.add(id)
  }
  selected.value = new Set(selected.value)
}

function toggleAll() {
  if (allSelected.value) {
    selected.value.clear()
  } else {
    flows.value.forEach(f => selected.value.add(f.id))
  }
  selected.value = new Set(selected.value)
}

async function banSelected() {
  for (const id of selected.value) {
    try {
      emit('open-word-picker', flows.value.find(f => f.id === id)!)
    } catch (e) {
      console.error(`Failed to open word picker for flow ${id}:`, e)
    }
  }
  selected.value.clear()
  selected.value = new Set(selected.value)
}

async function unbanFlow(flow: Flow) {
  try {
    await api.post(`/flows/${flow.id}/unban`)
    flow.banned = false
    fetchFlows()
  } catch (e) {
    console.error('Failed to unban flow:', e)
  }
}

async function toggleMirror(flow: Flow) {
  try {
    await api.post(`/flows/${flow.id}/mirror`, { enabled: !flow.mirrored })
    const mirrored = !flow.mirrored
    for (const f of flows.value) {
      if (f.hash === flow.hash) f.mirrored = mirrored
    }
  } catch (e) {
    console.error('Failed to toggle mirror:', e)
  }
}

onMounted(() => {
  fetchServices()
  fetchFlows()
  connectWebSocket()
})

onUnmounted(() => {
  ws?.close()
})
</script>

<style scoped>
.flow-table-page { display: flex; flex-direction: column; gap: 16px; }
.flow-table-page.compact .page-header { align-items: flex-start; flex-direction: column; }
.flow-table-page.compact .page-header h1 { font-size: 18px; }
.flow-table-page.compact .header-actions { width: 100%; flex-direction: column; align-items: stretch; }
.flow-table-page.compact .header-actions .input { width: 100%; }
.flow-table-page.compact .select { width: 100%; }
.flow-table-page.compact .table-container { overflow-x: hidden; }
.flow-table-page.compact .table th,
.flow-table-page.compact .table td { padding: 10px; }
.flow-table-page.compact .flow-row.selected td { background-color: var(--surface-hover); color: var(--primary); font-weight: 600; }
.flow-table-page.compact .pagination { gap: 8px; font-size: 12px; }
.page-header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.header-actions { display: flex; gap: 8px; align-items: center; }
.header-actions .input { width: 250px; }
.filter-check { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--text-muted); white-space: nowrap; }
.flow-row { cursor: pointer; }
.flow-row.negative-response td { background-color: rgba(245, 158, 11, 0.12); }
.flow-row.negative-response:hover td { background-color: rgba(245, 158, 11, 0.18); }
.flow-row.banned td,
.flow-row.banned.negative-response td { background-color: color-mix(in srgb, var(--destructive) 14%, transparent); }
.flow-row.banned:hover td,
.flow-row.banned.negative-response:hover td { background-color: color-mix(in srgb, var(--destructive) 20%, transparent); }
.flow-actions { display: flex; align-items: center; gap: 10px; }
.mirror-btn { min-width: 76px; justify-content: center; }
.flow-row:hover td { filter: brightness(1.05); }
.pagination { display: flex; align-items: center; justify-content: center; gap: 16px; padding: 12px; border-top: 1px solid var(--border); }
.selection-bar { position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%); padding: 12px 24px; border-radius: 12px; display: flex; align-items: center; gap: 12px; box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3); z-index: 100; background-color: var(--primary); color: var(--primary-foreground); }
.text-muted { color: var(--text-muted); }
.text-success { color: var(--success); }
</style>
