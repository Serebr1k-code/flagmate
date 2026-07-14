<template>
  <div class="flow-groups-page">
    <div class="page-header">
      <div>
        <h1>Flow Groups</h1>
        <p class="text-muted">Groups are equal stream fingerprints. Rows show the latest real stream from each group.</p>
      </div>
      <div class="header-actions">
        <label class="text-muted">Top</label>
        <input v-model.number="topN" type="number" class="input w-20" @change="fetchGroups" />
        <select v-model="sortBy" class="select" @change="sortGroups">
          <option value="count">By amount</option>
          <option value="threat">By threat</option>
          <option value="stability">By stability</option>
          <option value="first_seen">First seen</option>
          <option value="last_seen">Last seen</option>
        </select>
      </div>
    </div>

    <div class="group-list">
      <div v-for="group in sortedGroups" :key="group.hash" class="group-card">
        <div class="group-main">
          <div class="group-title">
            <input
              :value="draftNames[group.hash] ?? group.name"
              class="input name-input"
              placeholder="Group name"
              @input="draftNames[group.hash] = ($event.target as HTMLInputElement).value"
              @change="renameGroup(group)"
            />
            <span class="badge badge-primary">{{ group.count }}x</span>
            <span v-if="group.stability_pct > 0" class="badge" :class="group.stability_pct >= 70 ? 'badge-success' : 'badge-warning'">{{ group.stability_pct }}%</span>
            <span v-if="group.checker" class="badge badge-success">Checker</span>
            <span v-if="group.mirrored" class="badge badge-outline">Mirrored</span>
          </div>
          <div class="destination mono">{{ displayGroup(group) }}</div>
          <div class="meta text-muted">
            {{ formatTime(group.first_seen) }} → {{ formatTime(group.last_seen) }} ·
            <span :class="group.stability_pct >= 70 ? 'text-success' : group.stability_pct > 0 ? 'text-warning' : ''">
              {{ group.stability_pct }}% · {{ formatInterval(group.avg_interval) }}
            </span>
            · {{ group.hash.substring(0, 12) }}…
          </div>
        </div>
        <div class="actions">
          <button class="btn btn-sm btn-outline" @click="toggleChecker(group)">{{ group.checker ? 'Unchecker' : 'Checker' }}</button>
          <button class="btn btn-sm btn-ghost" @click="viewExampleFlow(group.example_flow_id)">Open latest</button>
        </div>
      </div>
      <div v-if="groups.length === 0" class="empty-state">No flow groups detected yet</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/utils/api'
import type { FlowGroup, Service } from '@/types'

const groups = ref<FlowGroup[]>([])
const sortedGroups = ref<FlowGroup[]>([])
const draftNames = ref<Record<string, string>>({})
const services = ref<Service[]>([])
const topN = ref(50)
const sortBy = ref('count')
const emit = defineEmits<{ 'open-flow-id': [flowId: string] }>()

async function fetchGroups() {
  try {
    const [{ data: groupsData }, { data: servicesData }] = await Promise.all([
      api.get('/flow-groups', { params: { top: topN.value } }),
      api.get('/services')
    ])
    services.value = servicesData
    groups.value = groupsData
    for (const group of groups.value) draftNames.value[group.hash] = group.name || ''
    sortGroups()
  } catch (e) { console.error('Failed to fetch groups:', e) }
}

function sortGroups() {
  const s = groups.value.slice()
  switch (sortBy.value) {
    case 'count':
      s.sort((a, b) => b.count - a.count)
      break
    case 'threat':
      s.sort((a, b) => {
        const aScore = (a.checker ? 0 : 1) + (a.mirrored ? 0 : 1) + Math.min(a.count, 50) / 50
        const bScore = (b.checker ? 0 : 1) + (b.mirrored ? 0 : 1) + Math.min(b.count, 50) / 50
        return bScore - aScore
      })
      break
    case 'stability':
      s.sort((a, b) => {
        const aStab = a.latest_flow?.stability_pct ?? 0
        const bStab = b.latest_flow?.stability_pct ?? 0
        return (bStab - aStab) || (b.count - a.count)
      })
      break
    case 'first_seen':
      s.sort((a, b) => String(a.first_seen).localeCompare(String(b.first_seen)) || b.count - a.count)
      break
    case 'last_seen':
      s.sort((a, b) => String(b.last_seen).localeCompare(String(a.last_seen)) || b.count - a.count)
      break
  }
  sortedGroups.value = s
}

async function renameGroup(group: FlowGroup) {
  const name = draftNames.value[group.hash] || ''
  try {
    await api.post(`/flow-groups/${group.hash}/name`, { name })
    group.name = name
  } catch (e) { console.error('Failed to rename group:', e) }
}

async function toggleChecker(group: FlowGroup) {
  try {
    await api.post(`/flow-groups/${group.hash}/checker`, { checker: !group.checker })
    group.checker = !group.checker
  } catch (e) { console.error('Failed to toggle checker group:', e) }
}

function displayGroup(group: FlowGroup) {
  const uri = group.uri || group.latest_flow?.raw_request?.uri || group.latest_flow?.raw_request?.url || ''
  const dstPort = group.latest_flow?.dst_port || group.destination?.match(/:(\d+)/)?.[1] || ''
  const method = group.method || group.latest_flow?.raw_request?.method || 'HTTP'
  const target = `${dstPort}${String(uri).startsWith('/') ? uri : `/${uri}`}`
  const svc = services.value.find(s => s.id === group.service_id)
  const svcName = svc?.name ? svc.name : `:${dstPort}`
  return `${svcName} ${method} ${target} -> ${group.response_code}`
}

function formatTime(ts: string) { return ts ? new Date(ts).toLocaleString() : '—' }
function formatInterval(sec: number) {
  if (!sec || sec <= 0) return '—'
  if (sec < 60) return `${Math.round(sec)}s`
  if (sec < 3600) return `${Math.round(sec / 60)}m ${Math.round(sec % 60)}s`
  return `${(sec / 3600).toFixed(1)}h`
}
function viewExampleFlow(flowId: string) { emit('open-flow-id', flowId) }

onMounted(fetchGroups)
</script>

<style scoped>
.flow-groups-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.page-header p { margin: 4px 0 0; }
.header-actions { display: flex; align-items: center; gap: 8px; }
.group-list { display: flex; flex-direction: column; gap: 10px; }
.group-card { display: flex; justify-content: space-between; gap: 16px; padding: 14px; border: 1px solid var(--border); border-radius: 12px; background: var(--card); }
.group-main { min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.group-title { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.name-input { width: 240px; }
.destination { font-size: 15px; font-weight: 700; }
.meta { font-size: 12px; }
.actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.mono { font-family: 'JetBrains Mono', monospace; }
.text-muted { color: var(--text-muted); }
.w-20 { width: 80px; }
</style>
