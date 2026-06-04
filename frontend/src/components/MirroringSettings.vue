<template>
  <div class="mirroring-page">
    <div class="page-header">
      <div>
        <h1>Mirroring</h1>
        <p class="text-muted">Marked flow groups are mirrored per service on your interval.</p>
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
          <label class="check-row">
            <input v-model="serviceConfig(service.id).enabled" type="checkbox" />
            mirror
          </label>
        </div>

        <div class="interval-row">
          <label>Every</label>
          <input v-model.number="serviceConfig(service.id).interval_seconds" class="input interval-input" type="number" min="1" />
          <span>seconds</span>
        </div>

        <div class="targets-list">
          <div v-for="(target, index) in serviceConfig(service.id).targets" :key="index" class="target-item">
            <span class="mono">{{ target.ip }}:{{ target.port }}</span>
            <button class="btn btn-sm btn-destructive" @click="removeTarget(service.id, index)">Remove</button>
          </div>
          <div v-if="serviceConfig(service.id).targets.length === 0" class="empty-state">No targets for this service</div>
        </div>

        <div class="add-target-form">
          <input v-model="newTargets[service.id].ip" class="input" placeholder="IP address" />
          <input v-model.number="newTargets[service.id].port" type="number" class="input" placeholder="Port" min="1" max="65535" />
          <button class="btn btn-primary" @click="addTarget(service.id)">Add</button>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">
        <h3 class="card-title">How it works</h3>
      </div>
      <div class="card-content">
        <p>Click <b>Mirror</b> on any flow: the whole flow group (same hash) is marked for mirroring.</p>
        <p>Each service has independent targets and interval. On every tick, Flagmate sends the latest flow from each marked group as one JSON line over TCP.</p>
      </div>
      <div class="card-footer">
        <button class="btn btn-primary" @click="saveConfig" :disabled="saving">{{ saving ? 'Saving...' : 'Save Configuration' }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/utils/api'
import type { MirroringConfig, Service, ServiceMirrorConfig } from '@/types'

const config = ref<MirroringConfig>({ enabled: false, targets: [], services: [] })
const services = ref<Service[]>([])
const newTargets = ref<Record<number, { ip: string; port: number }>>({})
const saving = ref(false)

async function fetchConfig() {
  try {
    const [{ data: mirrorData }, { data: serviceData }] = await Promise.all([
      api.get('/mirroring'),
      api.get('/services'),
    ])
    config.value = { enabled: false, targets: [], services: [], ...mirrorData }
    services.value = serviceData
    for (const service of services.value) {
      serviceConfig(service.id)
      if (!newTargets.value[service.id]) newTargets.value[service.id] = { ip: '', port: 0 }
    }
  } catch (e) { console.error('Failed to fetch mirroring config:', e) }
}

function serviceConfig(serviceId: number): ServiceMirrorConfig {
  let cfg = config.value.services.find(s => s.service_id === serviceId)
  if (!cfg) {
    cfg = { service_id: serviceId, enabled: false, interval_seconds: 60, targets: [] }
    config.value.services.push(cfg)
  }
  if (!cfg.interval_seconds || cfg.interval_seconds < 1) cfg.interval_seconds = 60
  if (!cfg.targets) cfg.targets = []
  return cfg
}

async function toggleMirroring() {
  config.value.enabled = !config.value.enabled
  await saveConfig()
}

function addTarget(serviceId: number) {
  const target = newTargets.value[serviceId]
  if (!target?.ip || !target?.port) return
  serviceConfig(serviceId).targets.push({ ...target })
  newTargets.value[serviceId] = { ip: '', port: 0 }
}

function removeTarget(serviceId: number, index: number) {
  serviceConfig(serviceId).targets.splice(index, 1)
}

async function saveConfig() {
  saving.value = true
  try {
    await api.post('/mirroring', config.value)
  } catch (e) { console.error('Failed to save mirroring config:', e) }
  finally { saving.value = false }
}

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
.service-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: 16px; }
.card { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 16px; }
.service-card { display: flex; flex-direction: column; gap: 14px; }
.service-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.service-head h3 { margin: 0; font-size: 18px; }
.service-head p { margin: 4px 0 0; }
.check-row { display: flex; align-items: center; gap: 8px; color: var(--text-muted); }
.interval-row { display: flex; align-items: center; gap: 8px; }
.interval-input { width: 100px; }
.interval-input::-webkit-outer-spin-button,
.interval-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.interval-input { -moz-appearance: textfield; appearance: textfield; }
.targets-list { display: flex; flex-direction: column; gap: 8px; }
.target-item { display: flex; align-items: center; justify-content: space-between; border: 1px solid var(--border); border-radius: 8px; padding: 10px 12px; background-color: var(--surface); }
.add-target-form { display: flex; gap: 8px; flex-wrap: wrap; }
.add-target-form .input { flex: 1; min-width: 140px; }
.card-header { margin-bottom: 12px; }
.card-title { font-size: 18px; font-weight: 600; margin: 0 0 4px; }
.card-content { font-size: 14px; line-height: 1.6; color: var(--text-muted); }
.card-content p { margin: 8px 0; }
.card-footer { display: flex; justify-content: flex-end; margin-top: 16px; }
.mono { font-family: 'JetBrains Mono', monospace; }
.text-muted { color: var(--text-muted); }
.empty-state { padding: 12px; text-align: center; color: var(--text-muted); font-size: 13px; }
</style>
