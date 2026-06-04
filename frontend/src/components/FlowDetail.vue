<template>
  <section class="flow-detail-panel" @scroll="onPanelScroll">
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

        <div class="flow-summary card">
          <div class="summary-grid">
            <div class="summary-item">
              <span class="label">Source</span>
              <span>{{ flow.src_ip }}:{{ flow.src_port }}</span>
            </div>
            <div class="summary-item">
              <span class="label">Destination</span>
              <span>{{ flow.destination || `${flow.dst_ip}:${flow.dst_port}` }}</span>
            </div>
            <div class="summary-item">
              <span class="label">Protocol</span>
              <span class="badge badge-outline">{{ flow.proto }}</span>
            </div>
            <div class="summary-item">
              <span class="label">Flows</span>
              <span>{{ flowHistory.length }}{{ hasMore ? '+' : '' }}</span>
            </div>
          </div>
          <div class="summary-actions">
            <button
              class="btn btn-sm"
              :class="flow.checker ? 'btn-success' : 'btn-secondary'"
              @click="toggleChecker"
            >
              {{ flow.checker ? 'Checker' : 'Not Checker' }}
            </button>
            <span class="badge" :class="flow.stability_pct >= 70 ? 'badge-success' : 'badge-warning'">{{ stabilityLabel(flow) }}</span>
            <span v-if="flow.banned" class="badge badge-destructive">Banned</span>
            <button
              v-if="!flow.banned"
              class="btn btn-sm btn-destructive"
              @click="banFlow"
            >
              Ban Words
            </button>
            <button
              v-if="flow.banned"
              class="btn btn-sm btn-outline"
              @click="unbanFlow"
            >
              Unban Flow
            </button>
          </div>
        </div>

        <div v-if="loading" class="empty-state">Loading flow history...</div>
        <div v-else class="transcript">
          <div
            v-for="(block, idx) in transcriptBlocks"
            :key="idx"
            class="transcript-block"
            :class="[block.isIncoming ? 'block-incoming' : 'block-outgoing', { banned: block.banned, checker: block.checker, 'negative-response': !block.isIncoming && block.response_code !== null && !isPositiveResponse(block.response_code) }]"
          >
            <div class="block-header">
              <span class="block-time">{{ formatTime(block.created_at) }}</span>
              <span v-if="block.response_code" class="badge" :class="isPositiveResponse(block.response_code) ? 'badge-success' : 'badge-warning'">{{ block.response_code }}</span>
              <span v-if="block.banned" class="badge badge-destructive">Banned</span>
              <span v-if="block.checker" class="badge badge-primary">Checker</span>
            </div>
            <pre class="block-payload">{{ formatPayload(block) }}</pre>
          </div>
          <div v-if="transcriptBlocks.length === 0" class="empty-state">
            No payload data captured
          </div>
          <div v-if="loadingMore" class="empty-state">Loading more...</div>
          <div v-else-if="!hasMore && flowHistory.length > 0" class="end-state">End of loaded history</div>
        </div>

        <div class="dialog-footer">
          <button
            v-if="!flow.banned"
            class="btn btn-destructive"
            @click="banFlow"
          >
            Ban Words
          </button>
          <button
            v-if="flow.banned"
            class="btn btn-outline"
            @click="unbanFlow"
          >
            Unban Flow
          </button>
          <button class="btn btn-outline" @click="$emit('close')">Close</button>
        </div>
  </section>

  <Teleport to="body">
    <div v-if="showUnbanConfirm" class="confirm-overlay" @click.self="showUnbanConfirm = false">
      <div class="confirm-dialog">
        <h2>Unban this flow?</h2>
        <p class="text-muted">These service ban rules match this flow and will be deleted:</p>
        <div class="confirm-list">
          <span v-for="pattern in matchingPatterns" :key="pattern.id" class="confirm-chip">
            {{ pattern.pattern }}
          </span>
        </div>
        <div class="confirm-actions">
          <button class="btn btn-outline" @click="showUnbanConfirm = false">Cancel</button>
          <button class="btn btn-destructive" @click="confirmUnbanFlow">Delete rules and unban</button>
        </div>
      </div>
    </div>
  </Teleport>

  <Teleport to="body">
    <div v-if="showCheckerConfirm" class="confirm-overlay" @click.self="showCheckerConfirm = false">
      <div class="confirm-dialog">
        <h2>Mark banned flow as checker?</h2>
        <p class="text-muted">Checkers must never be banned. These service ban rules match this flow and will be deleted before marking it as checker:</p>
        <div class="confirm-list">
          <span v-for="pattern in matchingPatterns" :key="pattern.id" class="confirm-chip">
            {{ pattern.pattern }}
          </span>
        </div>
        <div class="confirm-actions">
          <button class="btn btn-outline" @click="showCheckerConfirm = false">Cancel</button>
          <button class="btn btn-destructive" @click="confirmCheckerUnban">Delete rules and mark checker</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, defineProps, defineEmits, onMounted, watch } from 'vue'
import api from '@/utils/api'
import type { Flow, Pattern } from '@/types'

const props = defineProps<{ flow: Flow }>()
const emit = defineEmits<{ close: []; checkerToggled: [flow: Flow]; banClicked: [flow: Flow] }>()

const flowHistory = ref<Flow[]>([])
const loading = ref(true)
const loadingMore = ref(false)
const hasMore = ref(true)
const pageSize = 100
const showUnbanConfirm = ref(false)
const showCheckerConfirm = ref(false)
const matchingPatterns = ref<Pattern[]>([])

const transcriptBlocks = computed(() => {
  const blocks: Array<{
    isIncoming: boolean
    created_at: string
    response_code: number | null
    banned: boolean
    checker: boolean
    raw_request: Record<string, any> | null
    raw_response: Record<string, any> | null
  }> = []

  for (const f of flowHistory.value) {
    const hasReq = !!f.raw_request && Object.keys(f.raw_request).length > 0
    const hasResp = !!f.raw_response && Object.keys(f.raw_response).length > 0

    if (hasReq) {
      blocks.push({
        isIncoming: true,
        created_at: f.created_at,
        response_code: null,
        banned: f.banned,
        checker: f.checker,
        raw_request: f.raw_request,
        raw_response: null,
      })
    }
    if (hasResp) {
      blocks.push({
        isIncoming: false,
        created_at: f.created_at,
        response_code: f.response_code,
        banned: f.banned,
        checker: f.checker,
        raw_request: null,
        raw_response: f.raw_response,
      })
    }

    if (!hasReq && !hasResp) {
      blocks.push({
        isIncoming: false,
        created_at: f.created_at,
        response_code: f.response_code,
        banned: f.banned,
        checker: f.checker,
        raw_request: { info: `No payload captured for flow ${f.id}`, direction: f.direction },
        raw_response: null,
      })
    }
  }

  return blocks
})

async function fetchFlowHistory(reset = true) {
  if (reset) {
    loading.value = true
    flowHistory.value = []
    hasMore.value = true
  } else if (loadingMore.value || !hasMore.value) {
    return
  } else {
    loadingMore.value = true
  }
  try {
    const { data } = await api.get('/flows/history', {
      params: { hash: props.flow.hash, limit: pageSize, offset: reset ? 0 : flowHistory.value.length }
    })
    const rows = Array.isArray(data) ? data : []
    flowHistory.value = reset ? (rows.length > 0 ? rows : [props.flow]) : [...flowHistory.value, ...rows]
    hasMore.value = rows.length === pageSize
  } catch (e) {
    console.error('Failed to fetch flow history:', e)
    if (reset) flowHistory.value = [props.flow]
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

onMounted(() => fetchFlowHistory(true))
watch(() => props.flow.id, () => fetchFlowHistory(true))

function onPanelScroll(event: Event) {
  const el = event.currentTarget as HTMLElement
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 320) {
    fetchFlowHistory(false)
  }
}

function formatTime(ts: string | null) {
  if (!ts) return 'N/A'
  return new Date(ts).toLocaleString()
}

function stabilityLabel(flow: Flow) {
  const pct = Math.round(flow.stability_pct || 0)
  const avg = Number(flow.avg_interval || 0)
  return `${pct}%/${avg > 0 ? avg.toFixed(1) : '—'}s`
}

function isPositiveResponse(code: number) {
  return code === 101 || (code >= 200 && code < 400)
}

function formatPayload(block: typeof transcriptBlocks.value[0]): string {
  const raw = block.raw_request || block.raw_response
  if (!raw) return '(empty)'
  try { return JSON.stringify(raw, null, 2) } catch { return String(raw) }
}

async function toggleChecker() {
  try {
    if (!props.flow.checker && props.flow.banned) {
      const { data } = await api.get(`/flows/${props.flow.id}/matching-patterns`)
      matchingPatterns.value = Array.isArray(data) ? data : []
      if (matchingPatterns.value.length > 0) {
        showCheckerConfirm.value = true
        return
      }
      await api.post(`/flows/${props.flow.id}/unban`)
      props.flow.banned = false
    }
    await api.post(`/flows/${props.flow.id}/label`, { checker: !props.flow.checker })
    props.flow.checker = !props.flow.checker
    emit('checkerToggled', props.flow)
  } catch (e) { console.error('Failed to toggle checker:', e) }
}

async function confirmCheckerUnban() {
  try {
    await api.post(`/flows/${props.flow.id}/remove-matching-patterns`)
    await api.post(`/flows/${props.flow.id}/label`, { checker: true })
    props.flow.banned = false
    props.flow.checker = true
    matchingPatterns.value = []
    showCheckerConfirm.value = false
    emit('checkerToggled', props.flow)
  } catch (e) { console.error('Failed to mark checker after unban:', e) }
}

async function banFlow() {
  emit('banClicked', props.flow)
}

async function unbanFlow() {
  try {
    const { data } = await api.get(`/flows/${props.flow.id}/matching-patterns`)
    matchingPatterns.value = Array.isArray(data) ? data : []
    if (matchingPatterns.value.length > 0) {
      showUnbanConfirm.value = true
      return
    }
    await api.post(`/flows/${props.flow.id}/unban`)
    props.flow.banned = false
  } catch (e) { console.error('Failed to unban flow:', e) }
}

async function confirmUnbanFlow() {
  try {
    await api.post(`/flows/${props.flow.id}/remove-matching-patterns`)
    props.flow.banned = false
    matchingPatterns.value = []
    showUnbanConfirm.value = false
  } catch (e) { console.error('Failed to remove matching patterns:', e) }
}
</script>

<style scoped>
.flow-detail-panel { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 20px; height: calc(100vh - 48px); width: 100%; box-sizing: border-box; overflow-y: auto; box-shadow: 0 18px 48px rgba(0,0,0,0.28); }
.dialog-header { display: flex; align-items: center; justify-content: space-between; margin: -20px -20px 16px; padding: 16px 20px; position: sticky; top: -20px; z-index: 5; background-color: var(--card); border-bottom: 1px solid var(--border); }
.dialog-title { font-size: 20px; font-weight: 600; margin: 0; }
.hash-label { font-size: 13px; color: var(--text-muted); margin: 0 12px; }
.dialog-close { background: none; border: none; cursor: pointer; padding: 4px; border-radius: 4px; color: var(--muted-foreground); transition: all 0.15s; }
.dialog-close:hover { filter: brightness(1.2); }
.flow-summary { border: 1px solid var(--border); border-radius: 8px; padding: 16px; margin: 0 0 16px; background-color: var(--surface); position: sticky; top: 50px; z-index: 4; }
.summary-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 12px; margin-bottom: 12px; }
.summary-item { display: flex; flex-direction: column; gap: 2px; }
.summary-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.transcript { margin-bottom: 16px; display: flex; flex-direction: column; gap: 10px; }
.transcript-block { margin: 10px 0; border-radius: 6px; overflow: visible; display: block; width: 100%; box-sizing: border-box; }
.block-incoming { border: 2px solid #ef4444; background: #1a0a0a; }
.block-outgoing { border: 2px solid #22c55e; background: #0a1a0a; }
.block-outgoing.negative-response { border-color: #f59e0b; background: rgba(245, 158, 11, 0.16); }
.block-header { display: flex; align-items: center; gap: 8px; padding: 10px 14px; background: rgba(255,255,255,0.08); flex-wrap: wrap; border-bottom: 1px solid rgba(255,255,255,0.1); }
.block-time { font-size: 13px; color: #ccc; margin-right: auto; font-weight: 600; }
.block-payload { padding: 14px; font-family: 'JetBrains Mono', 'Fira Code', monospace; font-size: 13px; line-height: 1.6; overflow: visible; white-space: pre-wrap; word-break: break-word; display: block; min-height: 50px; color: #eee; width: 100%; box-sizing: border-box; }
.code-block { background-color: var(--surface); color: var(--text); }
.empty-state { text-align: center; padding: 32px; color: var(--text-muted); }
.end-state { text-align: center; padding: 12px; color: var(--text-muted); font-size: 12px; }
.dialog-footer { display: flex; justify-content: flex-end; gap: 8px; padding-top: 16px; border-top: 1px solid var(--border); }
.label { font-size: 12px; font-weight: 500; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); }
.mono { font-family: 'JetBrains Mono', monospace; }
.text-sm { font-size: 12px; }
.text-muted { color: var(--text-muted); }
.confirm-overlay { position: fixed; inset: 0; z-index: 1100; display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,0.65); backdrop-filter: blur(4px); }
.confirm-dialog { width: min(560px, 94vw); background: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 22px; box-shadow: 0 20px 60px rgba(0,0,0,0.45); }
.confirm-dialog h2 { margin: 0 0 8px; font-size: 20px; }
.confirm-list { display: flex; flex-wrap: wrap; gap: 8px; margin: 16px 0; max-height: 260px; overflow-y: auto; }
.confirm-chip { padding: 6px 10px; border-radius: 6px; border: 1px solid var(--destructive); background: rgba(239, 68, 68, 0.16); color: var(--text); font-family: 'JetBrains Mono', monospace; font-size: 13px; }
.confirm-actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
