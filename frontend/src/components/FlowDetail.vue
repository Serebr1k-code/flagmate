<template>
  <Teleport to="body">
    <div class="dialog-overlay" @click.self="$emit('close')">
      <div class="dialog">
        <div class="dialog-header">
          <h2 class="dialog-title">Flow History</h2>
          <span class="mono text-sm hash-label">{{ flow.hash.substring(0, 16) }}...</span>
          <button class="dialog-close" @click="$emit('close')">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        <div class="history-timeline">
          <div
            v-for="f in flowHistory"
            :key="f.id"
            class="history-item"
            :class="{ active: selectedFlow.id === f.id }"
            @click="selectFlow(f)"
          >
            <div class="history-item-time">{{ formatTime(f.created_at) }}</div>
            <div class="history-item-info">
              <span class="mono text-sm">{{ f.src_ip }}:{{ f.src_port }} → {{ f.dst_ip }}:{{ f.dst_port }}</span>
            </div>
            <div class="history-item-badges">
              <span v-if="f.stable" class="badge badge-success">Stable</span>
              <span v-if="f.checker" class="badge badge-primary">Checker</span>
              <span v-if="f.banned" class="badge badge-destructive">Banned</span>
              <span class="badge" :class="f.response_code === 200 ? 'badge-success' : 'badge-warning'">{{ f.response_code }}</span>
            </div>
          </div>
        </div>

        <div class="divider"></div>

        <div class="flow-detail-card card">
          <div class="header-grid">
            <div class="header-item">
              <span class="label">Source</span>
              <span>{{ selectedFlow.src_ip }}:{{ selectedFlow.src_port }}</span>
            </div>
            <div class="header-item">
              <span class="label">Destination</span>
              <span>{{ selectedFlow.dst_ip }}:{{ selectedFlow.dst_port }}</span>
            </div>
            <div class="header-item">
              <span class="label">Protocol</span>
              <span class="badge badge-outline">{{ selectedFlow.proto }}</span>
            </div>
            <div class="header-item">
              <span class="label">Flow ID</span>
              <span class="mono">{{ selectedFlow.flow_id }}</span>
            </div>
            <div class="header-item">
              <span class="label">Direction</span>
              <span>{{ selectedFlow.direction }}</span>
            </div>
            <div class="header-item">
              <span class="label">Response Code</span>
              <span class="badge" :class="selectedFlow.response_code === 200 ? 'badge-success' : 'badge-warning'">{{ selectedFlow.response_code }}</span>
            </div>
            <div class="header-item">
              <span class="label">Start</span>
              <span>{{ selectedFlow.start_ts ? formatTime(selectedFlow.start_ts) : 'N/A' }}</span>
            </div>
            <div class="header-item">
              <span class="label">End</span>
              <span>{{ selectedFlow.end_ts ? formatTime(selectedFlow.end_ts) : 'N/A' }}</span>
            </div>
            <div class="header-item">
              <span class="label">Packets</span>
              <span>{{ selectedFlow.pkt_count }}</span>
            </div>
            <div class="header-item">
              <span class="label">Bytes In</span>
              <span>{{ formatBytes(selectedFlow.bytes_in) }}</span>
            </div>
            <div class="header-item">
              <span class="label">Bytes Out</span>
              <span>{{ formatBytes(selectedFlow.bytes_out) }}</span>
            </div>
          </div>

          <div class="header-actions">
            <button
              class="btn btn-sm"
              :class="selectedFlow.checker ? 'btn-success' : 'btn-secondary'"
              @click="toggleChecker"
            >
              {{ selectedFlow.checker ? '✓ Checker' : '☐ Not Checker' }}
            </button>
          </div>
        </div>

        <div class="divider"></div>

        <div class="tabs">
          <button
            v-for="tab in ['request', 'response']"
            :key="tab"
            @click="activeTab = tab"
            class="tab"
            :class="{ active: activeTab === tab }"
          >
            {{ tab.charAt(0).toUpperCase() + tab.slice(1) }}
          </button>
        </div>

        <div class="content-area">
          <div v-if="activeTab === 'request'" class="code-block">
            {{ formatJSON(selectedFlow.raw_request) }}
          </div>
          <div v-else class="code-block">
            {{ formatJSON(selectedFlow.raw_response) }}
          </div>
        </div>

        <div class="dialog-footer">
          <button
            v-if="selectedFlow.response_code === 200 && !selectedFlow.banned"
            class="btn btn-destructive"
            @click="banFlow"
          >
            Ban Words
          </button>
          <button
            v-if="selectedFlow.banned"
            class="btn btn-outline"
            @click="unbanFlow"
          >
            Unban Flow
          </button>
          <button class="btn btn-outline" @click="$emit('close')">Close</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, defineProps, defineEmits, onMounted } from 'vue'
import api from '@/utils/api'
import type { Flow } from '@/types'

const props = defineProps<{ flow: Flow }>()
const emit = defineEmits<{ close: []; checkerToggled: [flow: Flow]; banClicked: [flow: Flow] }>()

const flowHistory = ref<Flow[]>([])
const selectedFlow = ref<Flow>(props.flow)
const activeTab = ref('request')

onMounted(async () => {
  try {
    const { data } = await api.get('/flows/history', { params: { hash: props.flow.hash } })
    flowHistory.value = data
    if (flowHistory.value.length > 0) {
      selectedFlow.value = flowHistory.value[0]
    }
  } catch (e) {
    console.error('Failed to fetch flow history:', e)
    flowHistory.value = [props.flow]
  }
})

function selectFlow(f: Flow) {
  selectedFlow.value = f
  activeTab.value = 'request'
}

function formatTime(ts: string | null) {
  if (!ts) return 'N/A'
  return new Date(ts).toLocaleString()
}

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatJSON(obj: Record<string, any>): string {
  try { return JSON.stringify(obj, null, 2) } catch { return String(obj) }
}

async function toggleChecker() {
  try {
    await api.post(`/flows/${selectedFlow.value.id}/label`, { checker: !selectedFlow.value.checker })
    selectedFlow.value.checker = !selectedFlow.value.checker
    const idx = flowHistory.value.findIndex(f => f.id === selectedFlow.value.id)
    if (idx !== -1) {
      flowHistory.value[idx] = { ...selectedFlow.value }
    }
    emit('checkerToggled', selectedFlow.value)
  } catch (e) { console.error('Failed to toggle checker:', e) }
}

async function banFlow() {
  emit('banClicked', selectedFlow.value)
}

async function unbanFlow() {
  try {
    await api.post(`/flows/${selectedFlow.value.id}/unban`)
    selectedFlow.value.banned = false
    const idx = flowHistory.value.findIndex(f => f.id === selectedFlow.value.id)
    if (idx !== -1) {
      flowHistory.value[idx] = { ...selectedFlow.value }
    }
  } catch (e) { console.error('Failed to unban flow:', e) }
}
</script>

<style scoped>
.dialog-overlay { position: fixed; inset: 0; background-color: rgba(0,0,0,0.6); backdrop-filter: blur(4px); z-index: 1000; display: flex; align-items: center; justify-content: center; }
.dialog { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 24px; max-width: 900px; width: 95%; max-height: 90vh; overflow-y: auto; box-shadow: 0 20px 60px rgba(0,0,0,0.4); }
.dialog-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.dialog-title { font-size: 20px; font-weight: 600; margin: 0; }
.hash-label { font-size: 13px; color: var(--text-muted); margin: 0 12px; }
.dialog-close { background: none; border: none; cursor: pointer; padding: 4px; border-radius: 4px; color: var(--muted-foreground); transition: all 0.15s; }
.dialog-close:hover { filter: brightness(1.2); }
.history-timeline { display: flex; flex-direction: column; gap: 4px; margin-bottom: 16px; max-height: 200px; overflow-y: auto; border: 1px solid var(--border); border-radius: 8px; padding: 8px; background-color: var(--surface); }
.history-item { display: flex; align-items: center; gap: 12px; padding: 8px 12px; border-radius: 6px; cursor: pointer; transition: all 0.15s; }
.history-item:hover { background-color: var(--surface-hover); }
.history-item.active { background-color: var(--primary); color: var(--primary-foreground); }
.history-item.active .badge { background-color: rgba(255,255,255,0.2); border-color: transparent; }
.history-item.active .text-sm { color: var(--primary-foreground); }
.history-item-time { font-size: 12px; color: var(--text-muted); min-width: 140px; }
.history-item.active .history-item-time { color: var(--primary-foreground); opacity: 0.8; }
.history-item-info { flex: 1; }
.history-item-badges { display: flex; gap: 4px; flex-wrap: wrap; }
.flow-detail-card { border: 1px solid var(--border); border-radius: 8px; padding: 16px; margin-bottom: 16px; background-color: var(--surface); }
.header-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 12px; margin-bottom: 12px; }
.header-item { display: flex; flex-direction: column; gap: 2px; }
.header-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.divider { height: 1px; background-color: var(--border); margin: 16px 0; }
.tabs { display: flex; gap: 2px; padding: 4px; border-radius: 8px; margin-bottom: 12px; background-color: var(--muted); }
.tab { padding: 6px 16px; border-radius: 6px; font-size: 14px; font-weight: 500; cursor: pointer; border: none; background: transparent; color: var(--muted-foreground); transition: all 0.15s; }
.tab:hover { filter: brightness(1.1); }
.tab.active { background-color: var(--surface); color: var(--text); box-shadow: 0 1px 3px rgba(0,0,0,0.2); }
.content-area { margin-bottom: 16px; }
.code-block { background-color: var(--surface); color: var(--text); border: 1px solid var(--border); border-radius: 8px; padding: 12px; font-family: 'JetBrains Mono', 'Fira Code', monospace; font-size: 13px; line-height: 1.5; overflow-x: auto; white-space: pre-wrap; word-break: break-all; max-height: 400px; overflow-y: auto; }
.dialog-footer { display: flex; justify-content: flex-end; gap: 8px; }
.label { font-size: 12px; font-weight: 500; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); }
.mono { font-family: 'JetBrains Mono', monospace; }
.text-sm { font-size: 12px; }
</style>
