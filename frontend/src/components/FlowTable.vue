<template>
  <div class="flow-table-page">
    <div class="page-header">
      <h1>Flows</h1>
      <div class="header-actions">
        <input
          v-model="searchQuery"
          class="input"
          placeholder="Search flows..."
          @input="debouncedFetch"
        />
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
            <th>
              <input type="checkbox" class="checkbox" :checked="allSelected" @change="toggleAll" />
            </th>
            <th>Time</th>
            <th>Direction</th>
            <th>Proto</th>
            <th>Status</th>
            <th>Response</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="flow in flows"
            :key="flow.id"
            class="flow-row"
            :class="{
              stable: flow.stable,
              banned: flow.banned
            }"
            @click="$emit('open-flow', flow)"
          >
            <td @click.stop>
              <input type="checkbox" class="checkbox" :checked="selected.has(flow.id)" @change="toggleSelect(flow.id)" />
            </td>
            <td class="text-muted">{{ formatTime(flow.created_at) }}</td>
            <td>{{ flow.direction }}</td>
            <td>
              <span class="badge badge-outline">{{ flow.proto }}</span>
            </td>
            <td>
              <span v-if="flow.stable" class="badge badge-success">Stable</span>
              <span v-if="flow.checker" class="badge badge-primary">Checker</span>
              <span v-if="flow.banned" class="badge badge-destructive">Banned</span>
            </td>
            <td>
              <span class="badge" :class="flow.response_code === 200 ? 'badge-success' : 'badge-warning'">
                {{ flow.response_code }}
              </span>
            </td>
            <td @click.stop>
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
            </td>
          </tr>
          <tr v-if="flows.length === 0">
            <td colspan="7" class="empty-state">No flows captured yet</td>
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
import { ref, computed, onMounted } from 'vue'
import api from '@/utils/api'
import type { Flow } from '@/types'

const emit = defineEmits<{
  'open-flow': [flow: Flow]
  'open-word-picker': [flow: Flow]
}>()

const flows = ref<Flow[]>([])
const page = ref(1)
const pageSize = ref(50)
const searchQuery = ref('')
const selected = ref(new Set<string>())
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const allSelected = computed(() => flows.value.length > 0 && selected.value.size === flows.value.length)

async function fetchFlows() {
  try {
    const params: Record<string, string> = {
      page: String(page.value),
      size: String(pageSize.value),
    }
    if (searchQuery.value) {
      params.search = searchQuery.value
    }
    const { data } = await api.get('/flows', { params })
    flows.value = data.flows
  } catch (e) {
    console.error('Failed to fetch flows:', e)
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

onMounted(fetchFlows)
</script>

<style scoped>
.flow-table-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.header-actions { display: flex; gap: 8px; align-items: center; }
.header-actions .input { width: 250px; }
.flow-row { cursor: pointer; }
.flow-row:hover td { filter: brightness(1.05); }
.pagination { display: flex; align-items: center; justify-content: center; gap: 16px; padding: 12px; border-top: 1px solid var(--border); }
.selection-bar { position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%); padding: 12px 24px; border-radius: 12px; display: flex; align-items: center; gap: 12px; box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3); z-index: 100; background-color: var(--primary); color: var(--primary-foreground); }
.text-muted { color: var(--text-muted); }
.text-success { color: var(--success); }
</style>
