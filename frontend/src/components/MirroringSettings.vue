<template>
  <div class="mirroring-page">
    <div class="page-header">
      <div>
        <h1>Mirroring</h1>
        <p class="text-muted">Global switch, per-service intervals, and persistent mirrored group names.</p>
      </div>
      <div class="toggle-container">
        <span class="text-muted">Enabled</span>
        <div class="toggle" :class="{ active: config.enabled }" @click="toggleMirroring"></div>
      </div>
    </div>

    <div class="service-grid">
      <div v-for="service in services" :key="service.id" class="card service-card">
        <div class="service-head">
          <div>
            <h3>{{ service.name }}</h3>
            <p class="mono text-muted">{{ service.protocol }} :{{ service.port }}</p>
          </div>
          <div class="interval-row">
            <label>Every</label>
            <input v-model.number="serviceConfig(service.id).interval_seconds" class="input interval-input" type="number" min="1" @change="saveConfig" />
            <span>seconds</span>
          </div>
        </div>

        <div class="mirrored-list">
          <div v-for="group in groupsForService(service.id)" :key="group.hash" class="mirror-group">
            <div class="group-main">
              <input
                :value="draftNames[group.hash] ?? group.name"
                class="input name-input"
                placeholder="Group name"
                @input="draftNames[group.hash] = ($event.target as HTMLInputElement).value"
                @change="renameGroup(group)"
              />
              <div class="mono target-line">{{ displayGroup(group) }}</div>
              <div class="text-muted small">{{ group.count }} streams · latest {{ formatTime(group.last_seen) }}</div>
            </div>
            <button class="btn btn-sm btn-destructive" @click="unmirror(group)">Remove</button>
          </div>
          <div v-if="groupsForService(service.id).length === 0" class="empty-state">No mirrored groups for this service</div>
        </div>
      </div>
    </div>

    <div class="card target-card">
      <div class="card-header">
        <h3 class="card-title">Mirror teams</h3>
        <p class="text-muted">Only IPs are stored. Flagmate sends every service to the same port number as that service.</p>
      </div>
      <div class="targets-list">
        <div v-for="(target, index) in config.targets" :key="index" class="target-item">
          <span class="mono">{{ target.ip }}:&lt;service port&gt;</span>
          <button class="btn btn-sm btn-destructive" @click="removeTarget(index)">Remove</button>
        </div>
        <div v-if="config.targets.length === 0" class="empty-state">No mirror targets</div>
      </div>
      <div class="add-target-form">
        <input v-model="newTargetIp" class="input" placeholder="Team IP" />
        <button class="btn btn-primary" @click="addTarget">Add IP</button>
        <button class="btn btn-outline" @click="saveConfig" :disabled="saving">{{ saving ? 'Saving...' : 'Save' }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/utils/api'
import type { FlowGroup, MirroringConfig, Service, ServiceMirrorConfig } from '@/types'

const config = ref<MirroringConfig>({ enabled: false, targets: [], services: [] })
const services = ref<Service[]>([])
const mirroredGroups = ref<FlowGroup[]>([])
const draftNames = ref<Record<string, string>>({})
const newTargetIp = ref('')
const saving = ref(false)

async function fetchConfig() {
  try {
    const [{ data: mirrorData }, { data: serviceData }, { data: groupData }] = await Promise.all([
      api.get('/mirroring'),
      api.get('/services'),
      api.get('/mirroring/groups'),
    ])
    config.value = { enabled: false, targets: [], services: [], ...mirrorData }
    services.value = serviceData
    mirroredGroups.value = groupData
    for (const service of services.value) serviceConfig(service.id)
    for (const group of mirroredGroups.value) draftNames.value[group.hash] = group.name || ''
  } catch (e) { console.error('Failed to fetch mirroring config:', e) }
}

function serviceConfig(serviceId: number): ServiceMirrorConfig {
  let cfg = config.value.services.find(s => s.service_id === serviceId)
  if (!cfg) {
    cfg = { service_id: serviceId, enabled: true, interval_seconds: 60, targets: [] }
    config.value.services.push(cfg)
  }
  cfg.enabled = true
  cfg.targets = []
  if (!cfg.interval_seconds || cfg.interval_seconds < 1) cfg.interval_seconds = 60
  return cfg
}

function groupsForService(serviceId: number) {
  return mirroredGroups.value.filter(group => group.service_id === serviceId)
}

async function toggleMirroring() {
  config.value.enabled = !config.value.enabled
  await saveConfig()
}

function addTarget() {
  const ip = newTargetIp.value.trim()
  if (!ip) return
  config.value.targets.push({ ip, port: 0 })
  newTargetIp.value = ''
  saveConfig()
}

function removeTarget(index: number) {
  config.value.targets.splice(index, 1)
  saveConfig()
}

async function renameGroup(group: FlowGroup) {
  const name = draftNames.value[group.hash] || ''
  try {
    await api.post(`/flow-groups/${group.hash}/name`, { name })
    group.name = name
  } catch (e) { console.error('Failed to rename mirrored group:', e) }
}

async function unmirror(group: FlowGroup) {
  const flowId = group.latest_flow?.id || group.example_flow_id
  if (!flowId) return
  try {
    await api.post(`/flows/${flowId}/mirror`, { enabled: false })
    mirroredGroups.value = mirroredGroups.value.filter(g => g.hash !== group.hash)
  } catch (e) { console.error('Failed to remove mirrored group:', e) }
}

async function saveConfig() {
  saving.value = true
  try {
    config.value.services = config.value.services.map(s => ({ ...s, enabled: true, targets: [] }))
    await api.post('/mirroring', config.value)
  } catch (e) { console.error('Failed to save mirroring config:', e) }
  finally { saving.value = false }
}

function displayGroup(group: FlowGroup) {
  const flow = group.latest_flow
  const uri = group.uri || flow?.raw_request?.uri || flow?.raw_request?.url || ''
  const port = flow?.dst_port || group.destination?.match(/:(\d+)/)?.[1] || ''
  const method = group.method || flow?.raw_request?.method || 'HTTP'
  return `${method} ${port}${String(uri).startsWith('/') ? uri : `/${uri}`} -> ${group.response_code}`
}

function formatTime(ts: string) { return ts ? new Date(ts).toLocaleString() : '—' }

onMounted(fetchConfig)
</script>

<style scoped>
.mirroring-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.page-header p { margin: 4px 0 0; }
.toggle-container { display: flex; align-items: center; gap: 12px; }
.toggle { position: relative; width: 44px; height: 24px; border-radius: 12px; cursor: pointer; transition: background-color 0.2s; background-color: var(--muted); }
.toggle.active { background-color: var(--primary); }
.toggle::after { content: ''; position: absolute; top: 2px; left: 2px; width: 20px; height: 20px; background-color: white; border-radius: 50%; transition: transform 0.2s; }
.toggle.active::after { transform: translateX(20px); }
.service-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(420px, 1fr)); gap: 16px; }
.card { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 16px; }
.service-card { display: flex; flex-direction: column; gap: 14px; }
.service-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.service-head h3 { margin: 0; font-size: 18px; }
.service-head p { margin: 4px 0 0; }
.interval-row { display: flex; align-items: center; gap: 8px; }
.interval-input { width: 90px; }
.interval-input::-webkit-outer-spin-button,
.interval-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.interval-input { -moz-appearance: textfield; appearance: textfield; }
.mirrored-list, .targets-list { display: flex; flex-direction: column; gap: 8px; }
.mirror-group, .target-item { display: flex; align-items: center; justify-content: space-between; gap: 12px; border: 1px solid var(--border); border-radius: 8px; padding: 10px 12px; background-color: var(--surface); }
.group-main { min-width: 0; flex: 1; display: flex; flex-direction: column; gap: 6px; }
.name-input { max-width: 260px; }
.target-line { font-size: 13px; font-weight: 700; }
.small { font-size: 12px; }
.add-target-form { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 12px; }
.add-target-form .input { flex: 1; min-width: 180px; }
.card-header { margin-bottom: 12px; }
.card-title { font-size: 18px; font-weight: 600; margin: 0 0 4px; }
.mono { font-family: 'JetBrains Mono', monospace; }
.text-muted { color: var(--text-muted); }
.empty-state { padding: 12px; text-align: center; color: var(--text-muted); font-size: 13px; }
</style>
