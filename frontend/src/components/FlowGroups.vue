<template>
  <div class="flow-groups-page">
    <div class="page-header">
      <h1>Flow Groups</h1>
      <div class="header-actions">
        <label class="text-muted">Top:</label>
        <input v-model.number="topN" type="number" class="input w-20" @change="fetchGroups" />
      </div>
    </div>

    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Hash</th><th>Count</th><th>Example Flow</th><th>First Seen</th><th>Last Seen</th><th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="group in groups" :key="group.hash">
            <td class="mono">{{ group.hash.substring(0, 16) }}...</td>
            <td><span class="badge badge-primary">{{ group.count }}</span></td>
            <td class="mono text-muted">{{ group.example_flow_id }}</td>
            <td class="text-muted">{{ formatTime(group.first_seen) }}</td>
            <td class="text-muted">{{ formatTime(group.last_seen) }}</td>
            <td><button class="btn btn-sm btn-ghost" @click="viewExampleFlow(group.example_flow_id)">View Flow</button></td>
          </tr>
          <tr v-if="groups.length === 0"><td colspan="6" class="empty-state">No flow groups detected yet</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/utils/api'
import type { FlowGroup } from '@/types'

const groups = ref<FlowGroup[]>([])
const topN = ref(20)

async function fetchGroups() {
  try {
    const { data } = await api.get('/flow-groups', { params: { top: topN.value } })
    groups.value = data
  } catch (e) { console.error('Failed to fetch groups:', e) }
}

function formatTime(ts: string) { return new Date(ts).toLocaleString() }
function viewExampleFlow(flowId: string) { window.open(`/api/flows/${flowId}`, '_blank') }

onMounted(fetchGroups)
</script>

<style scoped>
.flow-groups-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.header-actions { display: flex; align-items: center; gap: 8px; }
.table-container { width: 100%; overflow-x: auto; border: 1px solid var(--border); border-radius: 8px; }
.table { width: 100%; border-collapse: collapse; }
.table th { padding: 12px 16px; text-align: left; font-size: 13px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; border-bottom: 1px solid var(--border); background-color: var(--surface); color: var(--text-muted); }
.table td { padding: 12px 16px; font-size: 14px; border-bottom: 1px solid var(--border); }
.table tbody tr:hover { filter: brightness(1.05); }
.mono { font-family: 'JetBrains Mono', monospace; }
.text-muted { color: var(--text-muted); }
.w-20 { width: 80px; }
</style>
