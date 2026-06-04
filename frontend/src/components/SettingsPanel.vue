<template>
  <div class="settings-page">
    <div class="page-header">
      <h1>Settings</h1>
      <p class="text-muted">UI and poison response behavior.</p>
    </div>

    <div class="card setting-row">
      <div>
        <h3>Theme</h3>
        <p class="text-muted">Pick the dashboard theme.</p>
      </div>
      <ThemeSwitcher />
    </div>

    <div class="card setting-row">
      <div>
        <h3>Poison response</h3>
        <p class="text-muted">Choose what banned clients receive.</p>
      </div>
      <div class="mode-switch" :class="{ flag: poisonMode === 'flag' }" @click="togglePoisonMode">
        <span>Femboys</span>
        <span>Flag line</span>
        <div class="switch-thumb"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/utils/api'
import ThemeSwitcher from '@/components/ThemeSwitcher.vue'

const poisonMode = ref<'media' | 'flag'>('media')

async function fetchSettings() {
  try {
    const { data } = await api.get('/settings')
    poisonMode.value = data.poison_mode === 'flag' ? 'flag' : 'media'
  } catch (e) {
    console.error('Failed to fetch settings:', e)
  }
}

async function togglePoisonMode() {
  poisonMode.value = poisonMode.value === 'media' ? 'flag' : 'media'
  try {
    await api.post('/settings', { poison_mode: poisonMode.value })
  } catch (e) {
    console.error('Failed to save settings:', e)
  }
}

onMounted(fetchSettings)
</script>

<style scoped>
.settings-page { display: flex; flex-direction: column; gap: 16px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.page-header p { margin: 4px 0 0; }
.card { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 18px; }
.setting-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.setting-row h3 { margin: 0 0 4px; font-size: 18px; }
.setting-row p { margin: 0; }
.text-muted { color: var(--text-muted); }
.mode-switch { position: relative; width: 220px; height: 42px; border-radius: 999px; border: 1px solid var(--border); background: var(--surface); display: grid; grid-template-columns: 1fr 1fr; align-items: center; cursor: pointer; overflow: hidden; user-select: none; }
.mode-switch span { position: relative; z-index: 2; text-align: center; font-size: 13px; font-weight: 600; color: var(--text-muted); }
.mode-switch:not(.flag) span:first-child, .mode-switch.flag span:nth-child(2) { color: var(--primary-foreground); }
.switch-thumb { position: absolute; top: 4px; left: 4px; width: calc(50% - 4px); height: calc(100% - 8px); border-radius: 999px; background: var(--primary); transition: transform 0.2s ease; }
.mode-switch.flag .switch-thumb { transform: translateX(100%); }
</style>
