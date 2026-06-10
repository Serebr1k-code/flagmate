<template>
  <div class="dashboard">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h2>Flagmate</h2>
      </div>

      <nav class="sidebar-nav">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          @click="switchTab(tab.id)"
          class="nav-item"
          :class="{ active: activeTab === tab.id }"
        >
          {{ tab.label }}
        </button>
      </nav>

      <div class="sidebar-footer">
        <button @click="authStore.logout(); router.push('/login')" class="nav-item text-destructive">
          Logout
        </button>
      </div>
    </aside>

    <main class="main-content" :class="{ 'detail-open': selectedFlow }">
      <div class="content-pane" :class="{ compact: selectedFlow }">
        <component
          :is="currentComponent"
          :selected-flow="selectedFlow"
          :selected-service-id="selectedServiceId"
          @open-flow="onOpenFlow"
          @open-flow-id="onOpenFlowId"
          @open-service-stats="onOpenServiceStats"
          @open-word-picker="onOpenWordPicker"
        />
      </div>

      <div v-if="selectedFlow" class="detail-column">
        <div v-if="compromiseAlerts.length" class="alert-stack">
          <div v-for="alert in compromiseAlerts" :key="alert.key" class="compromise-alert">
            <div>
              <b>First compromise detected</b>
              <span>{{ alert.service || 'unknown service' }} leaked {{ alert.flag }} to {{ alert.attacker_ip }}</span>
            </div>
            <button class="alert-link" @click="onOpenFlowId(alert.flow_id)">open flow</button>
            <button class="alert-close" @click="dismissAlert(alert.key)">x</button>
          </div>
        </div>
        <FlowDetail class="detail-pane"
        :flow="selectedFlow"
        @close="selectedFlow = null"
        @checker-toggled="onCheckerToggled"
        @ban-clicked="onBanClicked"
        @ban-text="onBanText"
        @flow-updated="onFlowUpdated"
      />
      </div>
    </main>

    <WordPicker
      v-if="showWordPicker"
      :flow="wordPickerFlow"
      :unique-words="uniqueWords"
      :initial-selection="initialBanText"
      @close="closeWordPicker"
      @ban-words="onBanWords"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import FlowTable from '@/components/FlowTable.vue'
import FlowGroups from '@/components/FlowGroups.vue'
import ServiceManager from '@/components/ServiceManager.vue'
import BanPanel from '@/components/BanPanel.vue'
import MarksPanel from '@/components/MarksPanel.vue'
import MirroringSettings from '@/components/MirroringSettings.vue'
import StatsPanel from '@/components/StatsPanel.vue'
import SettingsPanel from '@/components/SettingsPanel.vue'
import FlowDetail from '@/components/FlowDetail.vue'
import WordPicker from '@/components/WordPicker.vue'
import type { Flow } from '@/types'
import api from '@/utils/api'

const router = useRouter()
const authStore = useAuthStore()
const activeTab = ref('flows')
const selectedFlow = ref<Flow | null>(null)
const selectedServiceId = ref<number | null>(null)
const showWordPicker = ref(false)
const wordPickerFlow = ref<Flow | null>(null)
const uniqueWords = ref<string[]>([])
const initialBanText = ref('')
const compromiseAlerts = ref<CompromiseAlert[]>([])
let alertTimer: ReturnType<typeof setInterval> | null = null
let dashboardSocket: WebSocket | null = null
let dashboardReconnectTimer: ReturnType<typeof setTimeout> | null = null
let selectedRefreshTimer: ReturnType<typeof setTimeout> | null = null

interface CompromiseAlert { key: string; flow_id: string; service: string; attacker_ip: string; flag: string; created_at: string; expires_at: number }

const tabs = [
  { id: 'flows', label: 'Flows', component: FlowTable },
  { id: 'groups', label: 'Groups', component: FlowGroups },
  { id: 'services', label: 'Services', component: ServiceManager },
  { id: 'bans', label: 'Bans', component: BanPanel },
  { id: 'marks', label: 'Marks', component: MarksPanel },
  { id: 'mirroring', label: 'Mirroring', component: MirroringSettings },
  { id: 'stats', label: 'Stats', component: StatsPanel },
  { id: 'settings', label: 'Settings', component: SettingsPanel },
]

const currentComponent = computed(() => {
  const tab = tabs.find(t => t.id === activeTab.value)
  return tab ? tab.component : null
})

function switchTab(tabId: string) {
  activeTab.value = tabId
  selectedFlow.value = null
  if (tabId !== 'stats') selectedServiceId.value = null
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    if (showWordPicker.value) {
      closeWordPicker()
      return
    }
    if (selectedFlow.value) selectedFlow.value = null
  }
}

function closeWordPicker() {
  showWordPicker.value = false
  initialBanText.value = ''
}

function onOpenFlow(flow: Flow) {
  selectedFlow.value = flow
}

async function onOpenFlowId(flowId: string) {
  try {
    const { data } = await api.get(`/flows/${flowId}`)
    selectedFlow.value = data
    activeTab.value = 'flows'
  } catch (e) {
    console.error('Failed to fetch flow:', e)
  }
}

function onOpenServiceStats(serviceId: number) {
  selectedServiceId.value = serviceId
  selectedFlow.value = null
  activeTab.value = 'stats'
}

async function onBanClicked(flow: Flow) {
  initialBanText.value = ''
  try {
    const { data } = await api.get(`/flows/${flow.id}/unique-words`)
    uniqueWords.value = data.words || []
  } catch (e) {
    console.error('Failed to fetch unique words:', e)
    uniqueWords.value = []
  }
  wordPickerFlow.value = flow
  showWordPicker.value = true
}

async function onBanText(payload: { flow: Flow; text: string }) {
  initialBanText.value = payload.text
  await openWordPicker(payload.flow)
}

async function onOpenWordPicker(flow: Flow) {
  initialBanText.value = ''
  await openWordPicker(flow)
}

async function openWordPicker(flow: Flow) {
  try {
    const { data } = await api.get(`/flows/${flow.id}/unique-words`)
    uniqueWords.value = data.words || []
  } catch (e) {
    console.error('Failed to fetch unique words:', e)
    uniqueWords.value = []
  }
  wordPickerFlow.value = flow
  showWordPicker.value = true
}

async function onBanWords(rules: Array<{ pattern: string; mode: 'B' | 'C' | 'S' }>) {
  if (!wordPickerFlow.value) return
  for (const rule of rules) {
    try {
      await api.post('/patterns', {
        service_id: wordPickerFlow.value.service_id,
        pattern: rule.pattern,
        description: `Auto-banned from flow ${wordPickerFlow.value?.id.substring(0, 8)}`,
        mode: rule.mode,
      })
    } catch (e) {
      console.error(`Failed to ban rule "${rule.pattern}":`, e)
    }
  }
  showWordPicker.value = false
  if (selectedFlow.value) {
    try {
      const { data } = await api.get(`/flows/${selectedFlow.value.id}`)
      selectedFlow.value = data
    } catch (e) {
      console.error('Failed to refresh selected flow after ban:', e)
    }
  }
  refreshCurrentComponent()
}

function onCheckerToggled(flow: Flow) {
  if (selectedFlow.value && selectedFlow.value.id === flow.id) {
    selectedFlow.value = { ...flow }
  }
}

async function onFlowUpdated(flow: Flow) {
  if (selectedFlow.value && selectedFlow.value.id === flow.id) {
    try {
      const { data } = await api.get(`/flows/${flow.id}`)
      selectedFlow.value = data
    } catch {
      selectedFlow.value = { ...flow }
    }
  }
  refreshCurrentComponent()
}

function refreshCurrentComponent() {
  const key = activeTab.value
  activeTab.value = ''
  setTimeout(() => { activeTab.value = key }, 0)
}

async function fetchCompromiseAlerts() {
  try {
    const { data } = await api.get('/stats/flag-thefts', { params: { minutes: 1440 } })
    const dismissed = dismissedAlertKeys()
    const firstByService = new Map<number | string, any>()
    for (const item of (data.items || []).slice().reverse()) {
      const key = item.service_id || item.service || 'unknown'
      if (!firstByService.has(key)) firstByService.set(key, item)
    }
    const now = Date.now()
    compromiseAlerts.value = Array.from(firstByService.values()).map(item => {
      const key = `first-compromise:${item.service_id || item.service}:${item.flag}:${item.created_at}`
      return { key, flow_id: item.flow_id, service: item.service, attacker_ip: item.attacker_ip, flag: item.flag, created_at: item.created_at, expires_at: new Date(item.created_at).getTime() + 15 * 60_000 }
    }).filter(alert => !dismissed.has(alert.key) && alert.expires_at > now)
  } catch (e) { console.error('Failed to fetch compromise alerts:', e) }
}

function dismissedAlertKeys() {
  try { return new Set(JSON.parse(localStorage.getItem('flagmate_dismissed_alerts') || '[]') as string[]) } catch { return new Set<string>() }
}

function dismissAlert(key: string) {
  const dismissed = dismissedAlertKeys()
  dismissed.add(key)
  localStorage.setItem('flagmate_dismissed_alerts', JSON.stringify(Array.from(dismissed).slice(-200)))
  compromiseAlerts.value = compromiseAlerts.value.filter(alert => alert.key !== key)
}

function connectDashboardSocket() {
  if (dashboardSocket && (dashboardSocket.readyState === WebSocket.OPEN || dashboardSocket.readyState === WebSocket.CONNECTING)) return
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  dashboardSocket = new WebSocket(`${proto}://${window.location.host}/ws`)
  dashboardSocket.onmessage = event => {
    try {
      const flow = JSON.parse(event.data)
      if (selectedFlow.value && flow.id === selectedFlow.value.id) scheduleSelectedFlowRefresh(flow.id)
    } catch {}
    fetchCompromiseAlerts()
  }
  dashboardSocket.onclose = () => {
    dashboardSocket = null
    if (!dashboardReconnectTimer) {
      dashboardReconnectTimer = setTimeout(() => {
        dashboardReconnectTimer = null
        connectDashboardSocket()
      }, 1500)
    }
  }
  dashboardSocket.onerror = () => dashboardSocket?.close()
}

function scheduleSelectedFlowRefresh(flowId: string) {
  if (selectedRefreshTimer) return
  selectedRefreshTimer = setTimeout(async () => {
    selectedRefreshTimer = null
    if (!selectedFlow.value || selectedFlow.value.id !== flowId) return
    try {
      const { data } = await api.get(`/flows/${flowId}`)
      selectedFlow.value = data
    } catch (e) { console.error('Failed to refresh live selected flow:', e) }
  }, 200)
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  fetchCompromiseAlerts()
  connectDashboardSocket()
  alertTimer = setInterval(fetchCompromiseAlerts, 30_000)
})
onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  if (alertTimer) clearInterval(alertTimer)
  if (dashboardReconnectTimer) clearTimeout(dashboardReconnectTimer)
  if (selectedRefreshTimer) clearTimeout(selectedRefreshTimer)
  dashboardSocket?.close()
})
</script>

<style scoped>
.alert-stack { display: flex; flex-direction: column; gap: 8px; margin-bottom: 12px; }
.compromise-alert { display: grid; grid-template-columns: 1fr auto auto; gap: 12px; align-items: center; border: 1px solid var(--destructive); background: linear-gradient(135deg, rgba(239, 68, 68, .22), rgba(127, 29, 29, .24)); color: var(--text); border-radius: 14px; padding: 12px 14px; box-shadow: 0 12px 28px rgba(0,0,0,.18); }
.compromise-alert div { display: flex; flex-direction: column; gap: 2px; }
.compromise-alert b { color: #fecaca; text-transform: uppercase; font-size: 12px; letter-spacing: .06em; }
.alert-link, .alert-close { border: 1px solid var(--border); background: var(--surface); color: var(--text); border-radius: 8px; padding: 6px 10px; cursor: pointer; }
.alert-close { color: var(--destructive); font-weight: 800; }
.dashboard { display: flex; min-height: 100vh; }
.sidebar { width: 240px; background-color: var(--surface); border-right: 1px solid var(--border); display: flex; flex-direction: column; padding: 16px; flex-shrink: 0; }
.sidebar-header { display: flex; align-items: center; justify-content: space-between; padding-bottom: 16px; border-bottom: 1px solid var(--border); margin-bottom: 16px; }
.sidebar-header h2 { font-size: 20px; font-weight: 700; margin: 0; color: var(--primary); }
.sidebar-nav { flex: 1; display: flex; flex-direction: column; gap: 4px; }
.nav-item { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-radius: 8px; border: none; cursor: pointer; font-size: 14px; font-weight: 500; transition: all 0.15s; width: 100%; text-align: left; background: transparent; color: var(--text-muted); }
.nav-item:hover:not(.active) { background-color: var(--surface-hover); color: var(--text); }
.nav-item.active { background-color: var(--surface-hover); color: var(--primary); font-weight: 600; }
.sidebar-footer { padding-top: 16px; border-top: 1px solid var(--border); }
.text-destructive { color: var(--destructive); }
.main-content { flex: 1; padding: 24px; overflow: hidden; background-color: var(--background); display: flex; gap: 18px; min-width: 0; }
.content-pane { flex: 1 1 auto; min-width: 0; overflow-y: auto; transition: flex-basis 0.2s ease, max-width 0.2s ease; }
.main-content.detail-open .content-pane { flex: 0 0 280px; max-width: 280px; }
.detail-column { display: flex; flex-direction: column; flex: 1 1 0; min-width: 0; overflow-y: auto; }
.detail-pane { flex: 1; min-width: 0; width: 100%; }
</style>
