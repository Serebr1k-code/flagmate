<template>
  <div class="flow-table-page" :class="{ compact: selectedFlow }">
    <div class="page-header">
      <div class="header-title-row">
        <h1>Flows</h1>
        <div class="ban-mode-switch">
          <div class="switch-track">
            <div class="switch-thumb" :style="{ transform: `translateX(${banMode * 100}%)` }">
              <span v-if="banMode === 0">Manual</span>
              <span v-else-if="banMode === 1">Auto-flag</span>
              <span v-else>Checker-only</span>
            </div>
            <div class="switch-labels">
              <span @click.stop="setBanMode(0)">Manual</span>
              <span @click.stop="setBanMode(1)">Auto-flag</span>
              <span @click.stop="setBanMode(2)">Checker-only</span>
            </div>
          </div>
          <div class="mode-tooltip">
            <div class="tooltip-row" :class="{ active: banMode === 0 }">
              <b>Manual</b><span>Manually ban flows via patterns. Banned flows get blocked/poisoned.</span>
            </div>
            <div class="tooltip-row" :class="{ active: banMode === 1 }">
              <b>Auto-flag</b><span>All traffic passes through. Flags in responses are replaced with fake ones.</span>
            </div>
            <div class="tooltip-row" :class="{ active: banMode === 2 }">
              <b>Checker-only</b><span>Only checker flows get responses. Everything else gets 503.</span>
            </div>
          </div>
        </div>
      </div>
      <div class="header-actions">
        <input
          v-model="searchQuery"
          class="input"
          placeholder="Search flows..."
          @input="debouncedFetch"
        />
        <select v-model="serviceFilter" class="select" @change="fetchFlows(true)">
          <option value="">All services</option>
          <option v-for="service in services" :key="service.id" :value="String(service.id)">
            {{ service.name }} :{{ service.port }}
          </option>
        </select>
        <label class="filter-check">
          <input v-model="showBanned" type="checkbox" @change="fetchFlows(true)" />
          Banned
        </label>
        <label class="filter-check">
          <input v-model="showChecker" type="checkbox" @change="fetchFlows(true)" />
          Checker
        </label>
        <button
          class="btn btn-sm"
          :class="collapseDuplicates ? 'btn-primary' : 'btn-outline'"
          @click="toggleCollapseDuplicates"
        >
          {{ collapseDuplicates ? 'Duplicates collapsed' : 'Collapse duplicates' }}
        </button>
        <button class="btn btn-sm btn-outline" @click="fetchFlows(true)">Refresh</button>
      </div>
    </div>

    <div ref="tableContainer" class="table-container" @scroll="onTableScroll">
      <table class="table">
        <thead>
          <tr>
            <th v-if="!selectedFlow">
              <input type="checkbox" class="checkbox" :checked="allSelected" @change="toggleAll" />
            </th>
            <th v-if="!selectedFlow">Time</th>
            <th>Direction</th>
            <th v-if="!selectedFlow">Status</th>
            <th v-if="!selectedFlow">Response</th>
            <th v-if="!selectedFlow">Actions</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="flow in flows" :key="flow.id">
            <tr
              class="flow-row"
              :class="rowClass(flow)"
              @click="$emit('open-flow', flow)"
            >
              <td v-if="!selectedFlow" @click.stop>
                <input type="checkbox" class="checkbox" :checked="selected.has(flow.id)" @change="toggleSelect(flow.id)" />
              </td>
              <td v-if="!selectedFlow" class="text-muted">{{ formatTime(flow.created_at) }}</td>
              <td>
                <div class="direction-cell">
                  <span class="direction-service">{{ serviceName(flow) }}</span>
                  <span class="direction-line">{{ displayDirection(flow) }}</span>
                </div>
              </td>
              <td v-if="!selectedFlow">
                <span class="badge" :class="flow.stability_pct >= 70 ? 'badge-success' : 'badge-warning'">{{ stabilityLabel(flow) }}</span>
                <span v-if="isWebSocketFlow(flow)" class="badge badge-ws">ws</span>
                <span v-if="flow.checker" class="badge badge-primary">Checker</span>
                <span v-else-if="isProbablyChecker(flow)" class="badge badge-success">Probably checker</span>
                <span v-if="flow.banned" class="badge badge-destructive">Banned</span>
                <span v-for="mark in flow.marks || []" :key="mark.id" class="badge mark-badge" :style="markStyle(mark.color)">{{ mark.name || mark.regex }}</span>
                <span v-if="flow.group_count > 1" class="badge badge-outline">{{ flow.group_count }}x</span>
              </td>
              <td v-if="!selectedFlow">
                <span class="badge" :class="isPositiveResponse(flow.response_code) ? 'badge-success' : 'badge-warning'">{{ flow.response_code }}</span>
              </td>
              <td v-if="!selectedFlow" class="flow-actions-cell" @click.stop>
                <div class="flow-actions">
                  <button v-if="!flow.banned" class="btn btn-sm btn-destructive" @click="$emit('open-word-picker', flow)">Ban</button>
                  <button v-else-if="flow.banned" class="btn btn-sm btn-outline" @click="unbanFlow(flow)">Unban</button>
                  <button class="btn btn-sm mirror-btn" :class="flow.mirrored ? 'btn-success' : 'btn-outline'" @click="toggleMirror(flow)">
                    {{ flow.mirrored ? 'Mirrored' : 'Mirror' }}
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="collapseDuplicates && flow.group_count > 1" class="expand-row" @click.stop="toggleExpanded(flow)">
              <td :colspan="selectedFlow ? 1 : 6">
                <span>{{ expandedHashes.has(flow.hash) ? '▴ collapse repeated streams' : `▾ ${flow.group_count - 1} repeated streams` }}</span>
              </td>
            </tr>
            <template v-if="collapseDuplicates && expandedHashes.has(flow.hash)">
              <tr
                v-for="item in expandedFlows[flow.hash] || []"
                :key="item.id"
                class="flow-row repeated-row"
                :class="rowClass(item)"
                @click="$emit('open-flow', item)"
              >
                <td v-if="!selectedFlow" @click.stop></td>
                <td v-if="!selectedFlow" class="text-muted">{{ formatTime(item.created_at) }}</td>
                <td>
                  <div class="direction-cell">
                    <span class="direction-service">{{ serviceName(item) }}</span>
                    <span class="direction-line">{{ displayDirection(item) }}</span>
                  </div>
                </td>
                <td v-if="!selectedFlow">
                  <span class="badge" :class="item.stability_pct >= 70 ? 'badge-success' : 'badge-warning'">{{ stabilityLabel(item) }}</span>
                  <span v-if="isWebSocketFlow(item)" class="badge badge-ws">ws</span>
                  <span v-if="item.checker" class="badge badge-primary">Checker</span>
                  <span v-else-if="isProbablyChecker(item)" class="badge badge-success">Probably checker</span>
                  <span v-for="mark in item.marks || []" :key="mark.id" class="badge mark-badge" :style="markStyle(mark.color)">{{ mark.name || mark.regex }}</span>
                </td>
                <td v-if="!selectedFlow"><span class="badge" :class="isPositiveResponse(item.response_code) ? 'badge-success' : 'badge-warning'">{{ item.response_code }}</span></td>
                <td v-if="!selectedFlow" class="flow-actions-cell" @click.stop>
                  <div class="flow-actions">
                    <button v-if="!item.banned" class="btn btn-sm btn-destructive" @click="$emit('open-word-picker', item)">Ban</button>
                    <button v-else-if="item.banned" class="btn btn-sm btn-outline" @click="unbanFlow(item)">Unban</button>
                    <button class="btn btn-sm mirror-btn" :class="item.mirrored ? 'btn-success' : 'btn-outline'" @click="toggleMirror(item)">
                      {{ item.mirrored ? 'Mirrored' : 'Mirror' }}
                    </button>
                  </div>
                </td>
              </tr>
            </template>
          </template>
          <tr v-if="flows.length === 0">
            <td :colspan="selectedFlow ? 1 : 6" class="empty-state">No flows captured yet</td>
          </tr>
          <tr v-else-if="!hasMore" class="end-row">
            <td :colspan="selectedFlow ? 1 : 6">End of flows</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="loadingMore" class="load-state">Loading more flows...</div>

    <div v-if="selected.size > 0" class="selection-bar">
      <span>{{ selected.size }} flow(s) selected</span>
      <button class="btn btn-sm btn-destructive" @click="banSelected">Ban Selected</button>
      <button class="btn btn-sm btn-ghost" @click="selected.clear()">Clear</button>
    </div>

    <div v-if="showUnbanConfirm" class="confirm-overlay" @click.self="cancelUnbanConfirm">
      <div class="confirm-dialog">
        <h2>Unban this flow?</h2>
        <p class="text-muted">These service ban rules match it and will be deleted:</p>
        <div class="confirm-list">
          <span v-for="pattern in pendingUnbanPatterns" :key="pattern.id" class="confirm-chip">
            {{ pattern.pattern }}
          </span>
        </div>
        <div class="confirm-actions">
          <button class="btn btn-outline" @click="cancelUnbanConfirm">Cancel</button>
          <button class="btn btn-destructive" @click="confirmUnbanFlow">Delete rules and unban</button>
        </div>
      </div>
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
const pageSize = 50
const searchQuery = ref('')
const serviceFilter = ref('')
const showBanned = ref(true)
const showChecker = ref(true)
const collapseDuplicates = ref(true)
const banMode = ref(0)
const services = ref<Service[]>([])
const selected = ref(new Set<string>())
const tableContainer = ref<HTMLElement | null>(null)
const loadingMore = ref(false)
const hasMore = ref(true)
const expandedHashes = ref(new Set<string>())
const expandedFlows = ref<Record<string, Flow[]>>({})
const showUnbanConfirm = ref(false)
const pendingUnbanFlow = ref<Flow | null>(null)
const pendingUnbanPatterns = ref<Array<{ id: number; pattern: string }>>([])
let debounceTimer: ReturnType<typeof setTimeout> | null = null
let liveSocket: WebSocket | null = null
let liveRefreshTimer: ReturnType<typeof setTimeout> | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

const allSelected = computed(() => flows.value.length > 0 && selected.value.size === flows.value.length)

async function fetchFlows(reset = true) {
  if (reset) {
    page.value = 1
    hasMore.value = true
  } else {
    if (loadingMore.value || !hasMore.value) return
    loadingMore.value = true
    page.value += 1
  }
  try {
    const params: Record<string, string> = {
      page: String(page.value),
      size: String(pageSize),
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
    if (collapseDuplicates.value) {
      params.collapse = 'true'
    }
    const { data } = await api.get('/flows', { params })
    const rows = data.flows || []
    flows.value = reset ? rows : [...flows.value, ...rows]
    hasMore.value = rows.length === pageSize
  } catch (e) {
    console.error('Failed to fetch flows:', e)
  } finally {
    loadingMore.value = false
  }
}

function scheduleLiveRefresh() {
  if (liveRefreshTimer) return
  liveRefreshTimer = setTimeout(() => {
    liveRefreshTimer = null
    fetchFlows(true)
  }, 250)
}

function connectLiveSocket() {
  if (liveSocket && (liveSocket.readyState === WebSocket.OPEN || liveSocket.readyState === WebSocket.CONNECTING)) return
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  liveSocket = new WebSocket(`${proto}://${window.location.host}/ws`)
  liveSocket.onmessage = () => scheduleLiveRefresh()
  liveSocket.onclose = () => {
    liveSocket = null
    if (!reconnectTimer) {
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        connectLiveSocket()
      }, 1500)
    }
  }
  liveSocket.onerror = () => liveSocket?.close()
}

function disconnectLiveSocket() {
  if (liveRefreshTimer) clearTimeout(liveRefreshTimer)
  if (reconnectTimer) clearTimeout(reconnectTimer)
  liveRefreshTimer = null
  reconnectTimer = null
  liveSocket?.close()
  liveSocket = null
}

function toggleCollapseDuplicates() {
  collapseDuplicates.value = !collapseDuplicates.value
  expandedHashes.value.clear()
  expandedHashes.value = new Set(expandedHashes.value)
  expandedFlows.value = {}
  fetchFlows(true)
}

function rowClass(flow: Flow) {
  return {
    stable: flow.stability_pct >= 70,
    banned: flow.banned,
    checker: flow.checker,
    selected: false,
    'negative-response': !flow.banned && !flow.checker && !isPositiveResponse(flow.response_code),
  }
}

async function toggleExpanded(flow: Flow) {
  if (expandedHashes.value.has(flow.hash)) {
    expandedHashes.value.delete(flow.hash)
    expandedHashes.value = new Set(expandedHashes.value)
    return
  }
  expandedHashes.value.add(flow.hash)
  expandedHashes.value = new Set(expandedHashes.value)
  if (!expandedFlows.value[flow.hash]) {
    try {
      const { data } = await api.get('/flows/history', { params: { hash: flow.hash, limit: 100, offset: 1 } })
      expandedFlows.value = { ...expandedFlows.value, [flow.hash]: data || [] }
    } catch (e) {
      console.error('Failed to fetch repeated streams:', e)
    }
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

async function fetchBanMode() {
  try {
    const { data } = await api.get('/settings')
    banMode.value = parseInt(String(data.ban_mode || '0'), 10) || 0
  } catch (e) {
    console.error('Failed to fetch ban mode:', e)
  }
}

async function setBanMode(mode: number) {
  banMode.value = mode
  try {
    await api.post('/settings', { ban_mode: String(mode) })
  } catch (e) {
    console.error('Failed to set ban mode:', e)
  }
}

function debouncedFetch() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    page.value = 1
    fetchFlows(true)
  }, 300)
}

function onTableScroll() {
  const el = tableContainer.value
  if (!el) return
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 900) {
    fetchFlows(false)
  }
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

function isProbablyChecker(flow: Flow) {
  return !flow.banned && !flow.checker && (flow.group_count || 0) >= 5 && (flow.stability_pct || 0) >= 70 && (flow.marks || []).length === 0
}

function isWebSocketFlow(flow: Flow) {
  const reqHeaders = flow.raw_request?.headers || {}
  const respHeaders = flow.raw_response?.headers || {}
  const reqUpgrade = String(reqHeaders.Upgrade || reqHeaders.upgrade || '').toLowerCase()
  const respUpgrade = String(respHeaders.Upgrade || respHeaders.upgrade || '').toLowerCase()
  return String(flow.proto || '').toLowerCase() === 'ws' || Number(flow.response_code || flow.raw_response?.status || 0) === 101 || reqUpgrade.includes('websocket') || respUpgrade.includes('websocket') || String(flow.raw_response?.body || '').includes('websocket upgrade')
}

function markStyle(color: string) {
  return { borderColor: color, backgroundColor: `${color}33`, color }
}

function displayDirection(flow: Flow) {
  const uri = String(flow.raw_request?.uri || flow.raw_request?.url || '')
  if (uri) return `${flow.dst_port}${uri.startsWith('/') ? uri : `/${uri}`}`
  if (flow.destination) return flow.destination.replace(/^.*?:(\d+)/, '$1')
  return `${flow.dst_port}`
}

function serviceName(flow: Flow) {
  const service = services.value.find(item => item.id === flow.service_id)
  if (service) return service.name
  return flow.service_id ? `service ${flow.service_id}` : 'unknown service'
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
    const { data } = await api.get(`/flows/${flow.id}/matching-patterns`)
    const patterns = Array.isArray(data) ? data : []
    if (patterns.length > 0) {
      pendingUnbanFlow.value = flow
      pendingUnbanPatterns.value = patterns
      showUnbanConfirm.value = true
      return
    } else {
      await api.post(`/flows/${flow.id}/unban`)
    }
    flow.banned = false
    await fetchFlows(true)
  } catch (e) {
    console.error('Failed to unban flow:', e)
  }
}

function cancelUnbanConfirm() {
  showUnbanConfirm.value = false
  pendingUnbanFlow.value = null
  pendingUnbanPatterns.value = []
}

async function confirmUnbanFlow() {
  const flow = pendingUnbanFlow.value
  if (!flow) return
  try {
    await api.post(`/flows/${flow.id}/remove-matching-patterns`)
    flow.banned = false
    cancelUnbanConfirm()
    await fetchFlows(true)
  } catch (e) {
    console.error('Failed to confirm unban flow:', e)
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
  fetchBanMode()
  fetchFlows(true)
  connectLiveSocket()
})

onUnmounted(disconnectLiveSocket)
</script>

<style scoped>
.flow-table-page { display: flex; flex-direction: column; gap: 16px; height: calc(100vh - 48px); min-height: 0; }
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
.page-header { display: flex; flex-direction: column; align-items: stretch; gap: 10px; }
.header-title-row { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.ban-mode-switch { display: flex; align-items: center; position: relative; }
.ban-mode-switch .switch-track { position: relative; width: 280px; height: 30px; border-radius: 999px; background: var(--surface); border: 1px solid var(--border); cursor: pointer; overflow: visible; }
.ban-mode-switch .switch-labels { position: absolute; inset: 0; display: flex; }
.ban-mode-switch .switch-labels span { flex: 1; display: flex; align-items: center; justify-content: center; font-size: 11px; font-weight: 500; color: var(--text-muted); letter-spacing: .02em; z-index: 1; cursor: pointer; }
.ban-mode-switch .switch-thumb { position: absolute; top: 0; left: 0; width: 33.333%; height: 100%; background: var(--primary); border-radius: 999px; transition: transform .15s; display: flex; align-items: center; justify-content: center; z-index: 2; }
.ban-mode-switch .switch-thumb span { font-size: 11px; font-weight: 600; color: #fff; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 90%; }
.ban-mode-switch .mode-tooltip { display: none; position: absolute; top: calc(100% + 10px); left: 0; width: 100%; background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 6px 8px; box-shadow: 0 8px 24px rgba(0,0,0,.24); z-index: 50; flex-direction: column; gap: 4px; box-sizing: border-box; }
.ban-mode-switch:hover .mode-tooltip { display: flex; }
.ban-mode-switch .tooltip-row { display: flex; gap: 8px; align-items: baseline; padding: 4px 6px; border-radius: 6px; font-size: 13px; line-height: 1.4; }
.ban-mode-switch .tooltip-row b { white-space: nowrap; color: var(--primary); min-width: 76px; flex-shrink: 0; }
.ban-mode-switch .tooltip-row span { color: var(--text-muted); }
.header-actions { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
.header-actions .input { width: 250px; }
.table-container { flex: 1; min-height: 0; overflow: auto; }
.filter-check { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--text-muted); white-space: nowrap; }
.flow-row { cursor: pointer; }
.repeated-row td { opacity: 0.84; padding-left: 24px; }
.expand-row td { padding: 4px 0; text-align: center; color: var(--text-muted); font-size: 12px; border-bottom: 1px solid var(--border); cursor: pointer; background: color-mix(in srgb, var(--surface) 70%, transparent); }
.expand-row:hover td { color: var(--primary); background: var(--surface-hover); }
.end-row td { text-align: center; color: var(--text-muted); font-size: 12px; padding: 12px; background: color-mix(in srgb, var(--surface) 70%, transparent); }
.flow-row.checker td { background-color: rgba(104, 157, 106, 0.30); }
.flow-row.checker:hover td { background-color: rgba(104, 157, 106, 0.38); }
.flow-row.negative-response td { background-color: rgba(250, 189, 47, 0.20); }
.flow-row.negative-response:hover td { background-color: rgba(250, 189, 47, 0.28); }
.flow-row.banned td,
.flow-row.banned.negative-response td { background-color: rgba(251, 73, 52, 0.22); }
.flow-row.banned:hover td,
.flow-row.banned.negative-response:hover td { background-color: rgba(251, 73, 52, 0.30); }
.flow-actions-cell { min-width: 170px; }
.flow-actions { display: flex; align-items: center; gap: 10px; }
.table td .badge + .badge { margin-left: 4px; }
.table td .badge { margin-bottom: 4px; }
.mark-badge { border: 1px solid; }
.badge-ws { border: 1px solid #38bdf8; background: rgba(56, 189, 248, 0.18); color: #7dd3fc; text-transform: uppercase; }
.direction-cell { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.direction-service { font-weight: 700; color: var(--text); }
.direction-line { color: var(--text-muted); font-family: 'JetBrains Mono', monospace; font-size: 12px; overflow-wrap: anywhere; }
.mirror-btn { min-width: 76px; justify-content: center; }
.flow-row:hover td { filter: brightness(1.05); }
.load-state { text-align: center; color: var(--text-muted); font-size: 12px; padding: 6px; }
.selection-bar { position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%); padding: 12px 24px; border-radius: 12px; display: flex; align-items: center; gap: 12px; box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3); z-index: 100; background-color: var(--primary); color: var(--primary-foreground); }
.confirm-overlay { position: fixed; inset: 0; z-index: 1100; display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,0.65); backdrop-filter: blur(4px); }
.confirm-dialog { width: min(560px, 94vw); background: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 22px; box-shadow: 0 20px 60px rgba(0,0,0,0.45); }
.confirm-dialog h2 { margin: 0 0 8px; font-size: 20px; }
.confirm-list { display: flex; flex-wrap: wrap; gap: 8px; margin: 16px 0; max-height: 260px; overflow-y: auto; }
.confirm-chip { padding: 6px 10px; border-radius: 6px; border: 1px solid var(--destructive); background: rgba(239, 68, 68, 0.16); color: var(--text); font-family: 'JetBrains Mono', monospace; font-size: 13px; }
.confirm-actions { display: flex; justify-content: flex-end; gap: 8px; }
.text-muted { color: var(--text-muted); }
.text-success { color: var(--success); }
</style>
