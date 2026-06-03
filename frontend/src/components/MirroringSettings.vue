<template>
  <div class="mirroring-page">
    <div class="page-header">
      <h1>Mirroring</h1>
      <div class="toggle-container">
        <span class="text-muted">Enabled</span>
        <div class="toggle" :class="{ active: config.enabled }" @click="toggleMirroring"></div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">
        <h3 class="card-title">Mirror Targets</h3>
        <p class="card-description">Forward EVE JSON streams to other teams via TCP</p>
      </div>

      <div class="targets-list">
        <div v-for="(target, index) in config.targets" :key="index" class="target-item card">
          <div class="target-info mono">{{ target.ip }}:{{ target.port }}</div>
          <button class="btn btn-sm btn-destructive" @click="removeTarget(index)">Remove</button>
        </div>
      </div>

      <div class="add-target-form">
        <input v-model="newTarget.ip" class="input" placeholder="IP address (e.g. 10.0.1.12)" />
        <input v-model.number="newTarget.port" type="number" class="input" placeholder="Port (e.g. 4000)" min="1" max="65535" />
        <button class="btn btn-primary" @click="addTarget" :disabled="!newTarget.ip || !newTarget.port">Add Target</button>
      </div>

      <div class="divider"></div>

      <div class="card-footer">
        <button class="btn btn-primary" @click="saveConfig" :disabled="saving">{{ saving ? 'Saving...' : 'Save Configuration' }}</button>
      </div>
    </div>

    <div class="card">
      <div class="card-header">
        <h3 class="card-title">How it works</h3>
      </div>
      <div class="card-content">
        <p>When enabled, every EVE JSON line from Suricata is forwarded to all configured targets over plain TCP.</p>
        <p>Each target receives the exact same data on the same port the service is listening on.</p>
        <p>Connections are automatically re-established if a target goes offline.</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/utils/api'
import type { MirroringConfig } from '@/types'

const config = ref<MirroringConfig>({ enabled: false, targets: [] })
const newTarget = ref({ ip: '', port: 0 })
const saving = ref(false)

async function fetchConfig() {
  try {
    const { data } = await api.get('/mirroring')
    config.value = data
  } catch (e) { console.error('Failed to fetch mirroring config:', e) }
}

async function toggleMirroring() {
  config.value.enabled = !config.value.enabled
  await saveConfig()
}

function addTarget() {
  if (newTarget.value.ip && newTarget.value.port) {
    config.value.targets.push({ ...newTarget.value })
    newTarget.value = { ip: '', port: 0 }
  }
}

function removeTarget(index: number) {
  config.value.targets.splice(index, 1)
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
.page-header { display: flex; align-items: center; justify-content: space-between; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.toggle-container { display: flex; align-items: center; gap: 12px; }
.toggle { position: relative; width: 44px; height: 24px; border-radius: 12px; cursor: pointer; transition: background-color 0.2s; background-color: var(--muted); }
.toggle.active { background-color: var(--primary); }
.toggle::after { content: ''; position: absolute; top: 2px; left: 2px; width: 20px; height: 20px; background-color: white; border-radius: 50%; transition: transform 0.2s; }
.toggle.active::after { transform: translateX(20px); }
.card { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 16px; }
.card-header { margin-bottom: 12px; }
.card-title { font-size: 18px; font-weight: 600; margin: 0 0 4px; }
.card-description { font-size: 14px; margin: 0; color: var(--muted-foreground); }
.card-content { font-size: 14px; line-height: 1.6; color: var(--text-muted); }
.card-content p { margin: 8px 0; }
.targets-list { display: flex; flex-direction: column; gap: 8px; margin-bottom: 16px; }
.target-item { display: flex; align-items: center; justify-content: space-between; border: 1px solid var(--border); border-radius: 8px; padding: 12px; background-color: var(--surface) !important; }
.target-info { font-family: monospace; }
.add-target-form { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 16px; }
.add-target-form .input { flex: 1; min-width: 150px; }
.divider { height: 1px; background-color: var(--border); margin: 16px 0; }
.card-footer { display: flex; justify-content: flex-end; }
.mono { font-family: 'JetBrains Mono', monospace; }
.text-muted { color: var(--text-muted); }
</style>
