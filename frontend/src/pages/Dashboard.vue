<template>
  <div class="dashboard">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h2>FlagMate</h2>
        <ThemeSwitcher />
      </div>

      <nav class="sidebar-nav">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          @click="activeTab = tab.id"
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

    <main class="main-content">
      <component :is="currentComponent" @open-flow="onOpenFlow" @open-word-picker="onOpenWordPicker" />
    </main>

    <FlowDetail
      v-if="selectedFlow"
      :flow="selectedFlow"
      @close="selectedFlow = null"
      @checker-toggled="onCheckerToggled"
      @ban-clicked="onBanClicked"
    />

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
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import ThemeSwitcher from '@/components/ThemeSwitcher.vue'
import FlowTable from '@/components/FlowTable.vue'
import FlowGroups from '@/components/FlowGroups.vue'
import ServiceManager from '@/components/ServiceManager.vue'
import PatternManager from '@/components/PatternManager.vue'
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
  { id: 'patterns', label: 'Patterns', component: PatternManager },
  { id: 'bans', label: 'Bans', component: BanPanel },
  { id: 'mirroring', label: 'Mirroring', component: MirroringSettings },
]

const currentComponent = computed(() => {
  const tab = tabs.find(t => t.id === activeTab.value)
  return tab ? tab.component : null
})

function onOpenFlow(flow: Flow) {
  selectedFlow.value = flow
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
      await api.post('/patterns', { pattern: word, description: `Auto-banned from flow ${wordPickerFlow.value?.id.substring(0, 8)}` })
    } catch (e) {
      console.error(`Failed to ban word "${word}":`, e)
    }
  }
  showWordPicker.value = false
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
.main-content { flex: 1; padding: 24px; overflow-y: auto; background-color: var(--background); }
</style>
