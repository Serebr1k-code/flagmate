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
          <div v-for="(item, idx) in flowHistory" :key="item.id" class="flow-occurrence">
            <div class="occurrence-header">
              <span class="block-time">{{ idx === 0 ? 'Selected stream' : `History stream #${idx + 1}` }}</span>
              <span class="text-muted">{{ formatTime(item.created_at) }}</span>
              <span v-if="item.response_code" class="badge" :class="isPositiveResponse(item.response_code) ? 'badge-success' : 'badge-warning'">{{ item.response_code }}</span>
              <span v-if="item.banned" class="badge badge-destructive">Banned</span>
              <span v-if="item.checker" class="badge badge-primary">Checker</span>
            </div>
            <div v-if="hasRequest(item)" class="transcript-block block-incoming">
              <div class="block-header"><span>client -> service</span></div>
              <pre class="block-payload">{{ formatRequestPayload(item.raw_request) }}</pre>
            </div>
            <div v-if="hasResponse(item)" class="transcript-block block-outgoing" :class="{ 'negative-response': !item.banned && !isPositiveResponse(item.response_code) }">
              <div class="block-header"><span>service -> client</span></div>
              <pre class="block-payload">{{ formatResponsePayload(item.raw_response, item.response_code) }}</pre>
            </div>
            <div v-if="!hasRequest(item) && !hasResponse(item)" class="empty-state">No payload captured for flow {{ item.id }}</div>
          </div>
          <div v-if="flowHistory.length === 0" class="empty-state">
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
import { ref, onMounted, watch } from 'vue'
import api from '@/utils/api'
import type { Flow, Pattern } from '@/types'

const props = defineProps<{ flow: Flow }>()
const emit = defineEmits<{ close: []; checkerToggled: [flow: Flow]; banClicked: [flow: Flow]; flowUpdated: [flow: Flow] }>()

const flowHistory = ref<Flow[]>([])
const loading = ref(true)
const loadingMore = ref(false)
const hasMore = ref(true)
const pageSize = 100
const showUnbanConfirm = ref(false)
const showCheckerConfirm = ref(false)
const matchingPatterns = ref<Pattern[]>([])

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

function hasRequest(flow: Flow) { return !!flow.raw_request && Object.keys(flow.raw_request).length > 0 }
function hasResponse(flow: Flow) { return !!flow.raw_response && Object.keys(flow.raw_response).length > 0 }

function formatRequestPayload(raw: Record<string, any>): string {
  const method = stringValue(raw.method || 'GET')
  const uri = stringValue(raw.uri || raw.url || '/')
  const query = stringValue(raw.query || '')
  const headers = normalizeHeaders(raw.headers)
  const body = formatBodyForDisplay(raw.body || '', headers)
  const lines: string[] = []
  lines.push(`${method} ${uri}${query ? `?${query}` : ''} HTTP`)
  for (const [key, value] of Object.entries(headers)) lines.push(`${key}: ${value}`)
  lines.push('')
  lines.push('---')
  lines.push(`method: ${method}`)
  lines.push(`uri: ${uri}`)
  if (query) lines.push(`query: ${query}`)
  if (body) {
    lines.push('')
    lines.push(body)
  } else {
    lines.push('payload: (empty)')
  }
  return lines.join('\n')
}

function formatResponsePayload(raw: Record<string, any>, responseCode: number | null): string {
  const status = Number(raw.status || responseCode || 0)
  const headers = normalizeHeaders(raw.headers)
  const body = formatBodyForDisplay(raw.body || '', headers)
  const lines: string[] = []
  if (status) lines.push(`HTTP ${status}`)
  for (const [key, value] of Object.entries(headers)) lines.push(`${key}: ${value}`)
  lines.push('')
  lines.push('---')
  if (status) lines.push(`status: ${status}`)
  if (body) {
    lines.push('')
    lines.push(body)
  } else {
    lines.push('payload: (empty)')
  }
  return lines.join('\n')
}

function normalizeHeaders(raw: any): Record<string, string> {
  const out: Record<string, string> = {}
  if (!raw || typeof raw !== 'object') return out
  for (const [key, value] of Object.entries(raw)) {
    out[key] = Array.isArray(value) ? value.map(stringValue).join(', ') : stringValue(value)
  }
  return out
}

function formatBodyForDisplay(raw: any, headers: Record<string, string>): string {
  const body = stringValue(raw)
  if (!body) return ''
  const json = tryFormatJSON(body)
  if (json) return json
  const contentType = Object.entries(headers).find(([key]) => key.toLowerCase() === 'content-type')?.[1] || ''
  if (isLongHTML(body, contentType)) return extractUsefulHTMLText(body)
  return body
}

function tryFormatJSON(body: string): string | null {
  const trimmed = body.trim()
  if (!trimmed || !['{', '['].includes(trimmed[0])) return null
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return null
  }
}

function isLongHTML(body: string, contentType: string): boolean {
  return body.length > 1200 && (contentType.toLowerCase().includes('html') || /<html|<body|<script|<div/i.test(body))
}

function extractUsefulHTMLText(body: string): string {
  const withoutScripts = body
    .replace(/<script[\s\S]*?<\/script>/gi, ' ')
    .replace(/<style[\s\S]*?<\/style>/gi, ' ')
  const text = withoutScripts
    .replace(/<[^>]+>/g, ' ')
    .replace(/&nbsp;/g, ' ')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
    .replace(/\s+/g, ' ')
    .trim()
  const vars = Array.from(withoutScripts.matchAll(/(?:value|content|data-[\w-]+)=["']([^"']{4,160})["']/gi)).map(match => match[1])
  const parts = [text.slice(0, 1800), ...vars.map(value => `var: ${value}`)]
  return parts.filter(Boolean).join('\n')
}

function stringValue(value: any): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
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
    await fetchFlowHistory(true)
    matchingPatterns.value = []
    showCheckerConfirm.value = false
    emit('checkerToggled', props.flow)
    emit('flowUpdated', props.flow)
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
    await fetchFlowHistory(true)
    emit('flowUpdated', props.flow)
  } catch (e) { console.error('Failed to unban flow:', e) }
}

async function confirmUnbanFlow() {
  try {
    await api.post(`/flows/${props.flow.id}/remove-matching-patterns`)
    props.flow.banned = false
    await fetchFlowHistory(true)
    matchingPatterns.value = []
    showUnbanConfirm.value = false
    emit('flowUpdated', props.flow)
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
.flow-occurrence { border: 1px solid var(--border); border-radius: 10px; padding: 10px; background: color-mix(in srgb, var(--surface) 70%, transparent); }
.occurrence-header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 8px; }
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
