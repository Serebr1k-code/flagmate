<template>
  <div class="ban-panel-page">
    <div class="page-header">
      <h1>Service Bans</h1>
      <div class="ban-mode-switch">
        <div class="switch-track">
          <div class="switch-thumb" :style="{ transform: `translateX(${banMode * 100}%)` }">
            <span v-if="banMode === 0">Block</span>
            <span v-else-if="banMode === 1">Poison</span>
            <span v-else>Ignore</span>
          </div>
          <div class="switch-labels">
            <span @click.stop="setBanMode(0)">Block</span>
            <span @click.stop="setBanMode(1)">Poison</span>
            <span @click.stop="setBanMode(2)">Ignore</span>
          </div>
        </div>
        <div class="mode-tooltip">
          <div class="tooltip-row" :class="{ active: banMode === 0 }">
            <b>Block</b><span>Return femboy media or fake flag directly on ban match.</span>
          </div>
          <div class="tooltip-row" :class="{ active: banMode === 1 }">
            <b>Poison</b><span>Let traffic through but replace real flags with fake ones.</span>
          </div>
          <div class="tooltip-row" :class="{ active: banMode === 2 }">
            <b>Ignore</b><span>Accept connection silently, hang until client timeout. No response sent.</span>
          </div>
        </div>
      </div>
    </div>

    <div class="ban-layout">
      <div class="services-list card">
        <div class="list-title">Services</div>
        <button
          v-for="service in services"
          :key="service.id"
          class="service-row"
          :class="{ active: selectedService?.id === service.id }"
          @click="selectService(service)"
        >
          <span>{{ service.name }}</span>
          <span class="mono text-muted">:{{ service.port }}</span>
        </button>
        <div v-if="services.length === 0" class="empty-state">No services configured</div>
      </div>

      <div class="patterns-pane card">
        <div v-if="selectedService" class="patterns-content">
          <div class="pane-header">
            <div>
              <h2>{{ selectedService.name }}</h2>
              <p class="text-muted mono">{{ selectedService.protocol }} :{{ selectedService.port }}</p>
            </div>
          </div>

          <div class="add-pattern-row">
            <input
              v-model="newPattern"
              class="input flex-1"
              placeholder="Word or regex for this service..."
              @keydown.enter="addPattern"
            />
            <select v-model="newPatternMode" class="select">
              <option value="C">Request</option>
              <option value="S">Response</option>
              <option value="B">Both</option>
            </select>
            <button class="btn btn-destructive" @click="addPattern">Add</button>
          </div>

          <div v-if="ruleWarnings.length" class="conflict-row">
            <span v-for="warning in ruleWarnings" :key="warning" class="warning-chip">{{ warning }}</span>
          </div>

          <div class="table-container">
            <table class="table">
              <thead>
                <tr>
                  <th>Pattern</th>
                  <th>Mode</th>
                  <th>Matches</th>
                  <th>Status</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="p in patterns" :key="p.id">
                  <td class="mono text-sm">{{ p.pattern }}</td>
                  <td><span class="badge badge-outline">{{ p.mode }}</span></td>
                  <td>{{ p.match_count || 0 }}</td>
                  <td>
                    <span class="badge" :class="p.active ? 'badge-success' : 'badge-warning'">
                      {{ p.active ? 'Active' : 'Disabled' }}
                    </span>
                  </td>
                  <td class="actions">
                    <button class="btn btn-sm btn-ghost" @click="togglePattern(p.id, !p.active)">
                      {{ p.active ? 'Disable' : 'Enable' }}
                    </button>
                    <button class="btn btn-sm btn-destructive" @click="deletePattern(p.id)">Delete</button>
                  </td>
                </tr>
                <tr v-if="patterns.length === 0">
                  <td colspan="5" class="empty-state">No bans for this service</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div v-else class="empty-state">Select a service to manage its bans</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import api from '@/utils/api'
import type { Pattern, Service } from '@/types'

const services = ref<Service[]>([])
const selectedService = ref<Service | null>(null)
const patterns = ref<Pattern[]>([])
const newPattern = ref('')
const newPatternMode = ref('B')
const banMode = ref(1)
const ruleWarnings = computed(() => {
  const warnings: string[] = []
  const never = patterns.value.filter(p => (p.match_count || 0) === 0).length
  const broad = patterns.value.filter(p => (p.match_count || 0) > 100).length
  const duplicates = patterns.value.length - new Set(patterns.value.map(p => `${p.pattern}:${p.mode}`)).size
  if (never) warnings.push(`${never} never matched`)
  if (broad) warnings.push(`${broad} very broad`)
  if (duplicates) warnings.push(`${duplicates} duplicate/conflicting`)
  return warnings
})

async function fetchServices() {
  try {
    const { data } = await api.get('/services')
    services.value = data
    if (!selectedService.value && services.value.length > 0) {
      await selectService(services.value[0])
    }
  } catch (e) {
    console.error('Failed to fetch services:', e)
  }
}

async function selectService(service: Service) {
  selectedService.value = service
  await fetchPatterns()
}

async function fetchPatterns() {
  if (!selectedService.value) return
  try {
    const { data } = await api.get('/patterns', { params: { service_id: selectedService.value.id } })
    patterns.value = data
  } catch (e) {
    console.error('Failed to fetch patterns:', e)
  }
}

async function addPattern() {
  if (!selectedService.value || !newPattern.value.trim()) return
  try {
    await api.post('/patterns', {
      service_id: selectedService.value.id,
      pattern: newPattern.value.trim(),
      description: `Service ${selectedService.value.name} ban (${newPatternMode.value})`,
      mode: newPatternMode.value,
    })
    newPattern.value = ''
    await fetchPatterns()
  } catch (e) {
    console.error('Failed to add pattern:', e)
  }
}

async function togglePattern(id: number, active: boolean) {
  try {
    await api.post(`/patterns/${id}/toggle`, { active })
    await fetchPatterns()
  } catch (e) {
    console.error('Failed to toggle pattern:', e)
  }
}

async function deletePattern(id: number) {
  try {
    await api.delete(`/patterns/${id}`)
    patterns.value = patterns.value.filter(p => p.id !== id)
  } catch (e) {
    console.error('Failed to delete pattern:', e)
  }
}

async function fetchBanMode() {
  try {
    const { data } = await api.get('/settings')
    banMode.value = parseInt(String(data.ban_mode || '1'), 10) || 1
  } catch (e) { console.error('Failed to fetch ban mode:', e) }
}

async function setBanMode(mode: number) {
  banMode.value = mode
  try {
    await api.post('/settings', { ban_mode: String(mode) })
  } catch (e) { console.error('Failed to set ban mode:', e) }
}

onMounted(() => { fetchServices(); fetchBanMode() })
</script>

<style scoped>
.ban-panel-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.ban-mode-switch { display: flex; align-items: center; position: relative; }
.ban-mode-switch .switch-track { position: relative; width: 280px; height: 30px; border-radius: 999px; background: var(--surface); border: 1px solid var(--border); cursor: pointer; overflow: visible; }
.ban-mode-switch .switch-labels { position: absolute; inset: 0; display: flex; }
.ban-mode-switch .switch-labels span { flex: 1; display: flex; align-items: center; justify-content: center; font-size: 11px; font-weight: 500; color: var(--text-muted); letter-spacing: .02em; z-index: 1; cursor: pointer; }
.ban-mode-switch .switch-thumb { position: absolute; top: 0; left: 0; width: 33.333%; height: 100%; background: var(--primary); border-radius: 999px; transition: transform .15s; display: flex; align-items: center; justify-content: center; z-index: 2; }
.ban-mode-switch .switch-thumb span { font-size: 11px; font-weight: 600; color: #fff; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 90%; }
.ban-mode-switch .mode-tooltip { display: none; position: absolute; top: calc(100% + 10px); left: 0; width: 100%; background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 6px 8px; box-shadow: 0 8px 24px rgba(0,0,0,.24); z-index: 50; flex-direction: column; gap: 4px; box-sizing: border-box; }
.ban-mode-switch:hover .mode-tooltip { display: flex; }
.ban-mode-switch .tooltip-row { display: flex; gap: 12px; align-items: baseline; padding: 4px 6px; border-radius: 6px; font-size: 13px; line-height: 1.4; }
.ban-mode-switch .tooltip-row b { white-space: nowrap; color: var(--primary); min-width: 90px; flex-shrink: 0; }
.ban-mode-switch .tooltip-row.active { background: color-mix(in srgb, var(--primary) 16%, transparent); border-radius: 6px; }
.ban-mode-switch .tooltip-row span { color: var(--text-muted); }
.ban-layout { display: grid; grid-template-columns: 280px minmax(0, 1fr); gap: 16px; min-height: 500px; }
.card { border: 1px solid var(--border); border-radius: 8px; background-color: var(--card); }
.services-list { padding: 12px; display: flex; flex-direction: column; gap: 8px; }
.list-title { font-size: 12px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--text-muted); margin-bottom: 4px; }
.service-row { display: flex; justify-content: space-between; align-items: center; gap: 8px; padding: 10px 12px; border-radius: 8px; border: 1px solid var(--border); background: var(--surface); color: var(--text); cursor: pointer; text-align: left; }
.service-row:hover, .service-row.active { background: var(--surface-hover); color: var(--primary); }
.patterns-pane { padding: 16px; overflow: hidden; }
.patterns-content { display: flex; flex-direction: column; gap: 16px; }
.pane-header { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.pane-header h2 { margin: 0; font-size: 20px; }
.pane-header p { margin: 4px 0 0; }
.add-pattern-row { display: flex; gap: 8px; align-items: center; }
.conflict-row { display: flex; flex-wrap: wrap; gap: 8px; }
.warning-chip { padding: 4px 9px; border-radius: 999px; border: 1px solid var(--warning); color: var(--warning); background: color-mix(in srgb, var(--warning) 14%, transparent); font-size: 12px; }
.table-container { width: 100%; overflow-x: auto; border: 1px solid var(--border); border-radius: 8px; background-color: var(--card); }
.table { width: 100%; border-collapse: collapse; }
.table th { padding: 12px 16px; text-align: left; font-size: 13px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; border-bottom: 1px solid var(--border); background-color: var(--surface); color: var(--text-muted); }
.table td { padding: 12px 16px; font-size: 14px; border-bottom: 1px solid var(--border); }
.actions { display: flex; gap: 8px; }
.mono { font-family: 'JetBrains Mono', monospace; }
.text-sm { font-size: 13px; }
.text-muted { color: var(--text-muted); }
.empty-state { text-align: center; padding: 32px; color: var(--text-muted); }
.flex-1 { flex: 1; }
</style>
