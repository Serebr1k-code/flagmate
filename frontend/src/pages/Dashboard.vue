<template>
  <div class="dashboard">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h2>Flagmate</h2>
        <ThemeSwitcher />
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
          @open-flow="onOpenFlow"
          @open-flow-id="onOpenFlowId"
          @open-word-picker="onOpenWordPicker"
        />
      </div>

      <FlowDetail
        v-if="selectedFlow"
        class="detail-pane"
        :flow="selectedFlow"
        @close="selectedFlow = null"
        @checker-toggled="onCheckerToggled"
        @ban-clicked="onBanClicked"
      />
    </main>

    <WordPicker
      v-if="showWordPicker"
      :flow="wordPickerFlow"
      :unique-words="uniqueWords"
      @close="showWordPicker = false"
      @ban-words="onBanWords"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import ThemeSwitcher from '@/components/ThemeSwitcher.vue'
import FlowTable from '@/components/FlowTable.vue'
import FlowGroups from '@/components/FlowGroups.vue'
import ServiceManager from '@/components/ServiceManager.vue'
import BanPanel from '@/components/BanPanel.vue'
import MirroringSettings from '@/components/MirroringSettings.vue'
import FlowDetail from '@/components/FlowDetail.vue'
import WordPicker from '@/components/WordPicker.vue'
import type { Flow } from '@/types'
import api from '@/utils/api'

const router = useRouter()
const authStore = useAuthStore()
const activeTab = ref('flows')
const selectedFlow = ref<Flow | null>(null)
const showWordPicker = ref(false)
const wordPickerFlow = ref<Flow | null>(null)
const uniqueWords = ref<string[]>([])

const tabs = [
  { id: 'flows', label: 'Flows', component: FlowTable },
  { id: 'groups', label: 'Groups', component: FlowGroups },
  { id: 'services', label: 'Services', component: ServiceManager },
  { id: 'bans', label: 'Bans', component: BanPanel },
  { id: 'mirroring', label: 'Mirroring', component: MirroringSettings },
]

const currentComponent = computed(() => {
  const tab = tabs.find(t => t.id === activeTab.value)
  return tab ? tab.component : null
})

function switchTab(tabId: string) {
  activeTab.value = tabId
  selectedFlow.value = null
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    if (selectedFlow.value) selectedFlow.value = null
    if (showWordPicker.value) showWordPicker.value = false
  }
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

async function onBanClicked(flow: Flow) {
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

async function onOpenWordPicker(flow: Flow) {
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

async function onBanWords(words: string[]) {
  if (!wordPickerFlow.value) return
  for (const word of words) {
    try {
      await api.post('/patterns', {
        service_id: wordPickerFlow.value.service_id,
        pattern: word,
        description: `Auto-banned from flow ${wordPickerFlow.value?.id.substring(0, 8)}`,
        mode: 'B',
      })
    } catch (e) {
      console.error(`Failed to ban word "${word}":`, e)
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

function refreshCurrentComponent() {
  const key = activeTab.value
  activeTab.value = ''
  setTimeout(() => { activeTab.value = key }, 0)
}

onMounted(() => window.addEventListener('keydown', handleKeydown))
onUnmounted(() => window.removeEventListener('keydown', handleKeydown))
</script>

<style scoped>
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
.detail-pane { flex: 1 1 0; min-width: 0; width: 100%; overflow-y: auto; }
</style>
