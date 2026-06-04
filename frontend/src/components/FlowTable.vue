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
            <td v-if="!selectedFlow" class="flow-actions-cell" @click.stop>
              <div class="flow-actions">
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
              </div>
            </td>
          </tr>
          <tr v-if="flows.length === 0">
            <td :colspan="selectedFlow ? 1 : 7" class="empty-state">No flows captured yet</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="loadingMore" class="load-state">Loading more flows...</div>
    <div v-else-if="!hasMore && flows.length > 0" class="load-state">End of flows</div>

    <div v-if="selected.size > 0" class="selection-bar">
      <span>{{ selected.size }} flow(s) selected</span>
      <button class="btn btn-sm btn-destructive" @click="banSelected">Ban Selected</button>
      <button class="btn btn-sm btn-ghost" @click="selected.clear()">Clear</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import api from '@/utils/api'
import type { Flow, Service } from '@/types'

const emit = defineEmits<{
  'open-flow': [flow: Flow]
  'open-word-picker': [flow: Flow]
}>()

defineProps<{ selectedFlow?: Flow | null }>()

const flows = ref<Flow[]>([])
const page = ref(1)
const pageSize = 100
const searchQuery = ref('')
const serviceFilter = ref('')
const showBanned = ref(true)
const showChecker = ref(true)
const services = ref<Service[]>([])
const selected = ref(new Set<string>())
const tableContainer = ref<HTMLElement | null>(null)
const loadingMore = ref(false)
const hasMore = ref(true)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

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
    const { data } = await api.get(`/flows/${flow.id}/matching-patterns`)
    const patterns = Array.isArray(data) ? data : []
    if (patterns.length > 0) {
      const list = patterns.map((p: { pattern: string }) => `- ${p.pattern}`).join('\n')
      const ok = window.confirm(`Unban this flow? These service ban rules match it and will be deleted:\n\n${list}`)
      if (!ok) return
      await api.post(`/flows/${flow.id}/remove-matching-patterns`)
    } else {
      await api.post(`/flows/${flow.id}/unban`)
    }
    flow.banned = false
    await fetchFlows(true)
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
  fetchFlows(true)
})
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
.page-header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.header-actions { display: flex; gap: 8px; align-items: center; }
.header-actions .input { width: 250px; }
.table-container { flex: 1; min-height: 0; overflow: auto; }
.filter-check { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--text-muted); white-space: nowrap; }
.flow-row { cursor: pointer; }
.flow-row.negative-response td { background-color: rgba(245, 158, 11, 0.12); }
.flow-row.negative-response:hover td { background-color: rgba(245, 158, 11, 0.18); }
.flow-row.banned td,
.flow-row.banned.negative-response td { background-color: color-mix(in srgb, var(--destructive) 14%, transparent); }
.flow-row.banned:hover td,
.flow-row.banned.negative-response:hover td { background-color: color-mix(in srgb, var(--destructive) 20%, transparent); }
.flow-actions-cell { min-width: 170px; }
.flow-actions { display: flex; align-items: center; gap: 10px; }
.mirror-btn { min-width: 76px; justify-content: center; }
.flow-row:hover td { filter: brightness(1.05); }
.load-state { text-align: center; color: var(--text-muted); font-size: 12px; padding: 6px; }
.selection-bar { position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%); padding: 12px 24px; border-radius: 12px; display: flex; align-items: center; gap: 12px; box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3); z-index: 100; background-color: var(--primary); color: var(--primary-foreground); }
.text-muted { color: var(--text-muted); }
.text-success { color: var(--success); }
</style>
