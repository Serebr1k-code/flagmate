<template>
  <Teleport to="body">
    <div class="dialog-overlay" @click.self="$emit('close')">
      <div class="dialog">
        <div class="dialog-header">
          <h2 class="dialog-title">Ban</h2>
          <button class="dialog-close" @click="$emit('close')">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        <div v-if="flow" class="flow-info-card">
          <span class="label">Flow:</span>
          <span class="mono">{{ flow.src_ip }}:{{ flow.src_port }} -> {{ flow.destination || `${flow.dst_ip}:${flow.dst_port}` }}</span>
        </div>

        <div class="custom-input-row">
          <input
            v-model="customWord"
            class="input flex-1"
            placeholder="Type custom word or regex..."
            @keydown.enter="addCustomWord"
          />
          <select v-model="customMode" class="select mode-select">
            <option value="B">request + response</option>
            <option value="C">request only</option>
            <option value="S">response only</option>
          </select>
          <button class="btn btn-sm btn-outline" @click="addCustomWord">Add</button>
        </div>

        <div v-if="pathWords.length > 0" class="words-section path-section">
          <h3>Endpoint / path parts</h3>
          <div class="word-chips">
            <span
              v-for="word in pathWords"
              :key="word"
              class="word-chip path-chip"
              :class="{ selected: isSelectedPattern(word) }"
              @click="toggleWord(word, 'C')"
            >
              {{ word }}
            </span>
          </div>
        </div>

        <div v-if="payloadHints.length > 0" class="words-section payload-section">
          <h3>Suspicious payload pieces / variables</h3>
          <div class="word-chips">
            <span
              v-for="hint in payloadHints"
              :key="hint.pattern + hint.mode"
              class="word-chip payload-chip"
              :class="{ selected: isSelectedPattern(hint.pattern) }"
              @click="toggleWord(hint.pattern, hint.mode)"
            >
              {{ hint.label }}
            </span>
          </div>
        </div>

        <div v-if="responseHints.length > 0" class="words-section response-section">
          <h3>Response shape</h3>
          <div class="word-chips">
            <span
              v-for="hint in responseHints"
              :key="hint.pattern + hint.mode"
              class="word-chip response-chip"
              :class="{ selected: isSelectedPattern(hint.pattern) }"
              @click="toggleWord(hint.pattern, hint.mode)"
            >
              {{ hint.label }}
            </span>
          </div>
        </div>

        <div v-if="markerHints.length > 0" class="words-section marker-section">
          <h3>Marked snippets</h3>
          <div class="word-chips">
            <span
              v-for="hint in markerHints"
              :key="hint.pattern + hint.mode + hint.label"
              class="word-chip marker-chip"
              :style="markerStyle(hint)"
              :class="{ selected: isSelectedPattern(hint.pattern) }"
              @mouseenter="showHint($event, hint.label || '')"
              @mousemove="moveHint($event)"
              @mouseleave="hideHint"
              @click="toggleWord(hint.pattern, hint.mode)"
            >
              {{ hint.pattern }}
            </span>
          </div>
        </div>

        <div class="words-section">
          <h3>Other unique words (not in checker flows)</h3>
          <div class="word-chips">
            <span
              v-for="word in nonPathWords"
              :key="word"
              class="word-chip"
              :class="{ selected: isSelectedPattern(word) }"
              @click="toggleWord(word, 'B')"
            >
              {{ word }}
            </span>
            <span v-if="uniqueWords.length === 0" class="empty-state">No unique words found</span>
          </div>
        </div>

        <div class="selected-section">
          <h3>Selected for ban ({{ selectedItems.length }})</h3>
          <div class="word-chips">
            <span
              v-for="item in selectedItems"
              :key="item.key"
              class="word-chip selected selected-ban-chip"
              @click="toggleWord(item.pattern, item.mode)"
              @contextmenu.prevent="cycleMode(item)"
            >
              {{ item.pattern }} <small>{{ modeLabel(item.mode) }}</small> ×
            </span>
            <span v-if="selectedItems.length === 0" class="empty-state">No ban rules selected</span>
          </div>
        </div>

        <div class="dialog-footer">
          <div class="footer-impact">
            <span v-if="impactPercent > 25" class="impact-warning">This will ban ~{{ impactPercent }}% flows ({{ impact.flows }}/{{ impactTotal }})</span>
            <span>groups: {{ impact.groups }}</span>
            <span>flows: {{ impact.flows }}</span>
            <span>checkers: {{ impact.checkers }}</span>
          </div>
          <button class="btn btn-outline" @click="$emit('close')">Cancel</button>
          <button class="btn btn-destructive" :disabled="selectedItems.length === 0" @click="banWords">
            Ban {{ selectedItems.length }} rule(s)
          </button>
        </div>
      </div>
      <div v-if="tooltip.text" class="mark-tooltip" :style="{ left: `${tooltip.x}px`, top: `${tooltip.y}px` }">{{ tooltip.text }}</div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import api from '@/utils/api'
import type { Flow } from '@/types'

const props = defineProps<{ flow: Flow | null; uniqueWords: string[]; initialSelection?: string }>()
type BanMode = 'B' | 'C' | 'S'
type BanCandidate = { pattern: string; mode: BanMode; label?: string; color?: string }

const emit = defineEmits<{ close: []; banWords: [rules: BanCandidate[]] }>()

const selectedWords = ref(new Set<string>())
const customWord = ref('')
const customMode = ref<BanMode>('B')
const impact = ref({ groups: 0, flows: 0, checkers: 0 })
const impactTotal = ref(0)
const tooltip = ref({ text: '', x: 0, y: 0 })
let impactTimer: ReturnType<typeof setTimeout> | null = null
const impactPercent = computed(() => impactTotal.value ? Math.round((impact.value.flows / impactTotal.value) * 100) : 0)

const pathWords = computed(() => {
  const uri = String(props.flow?.raw_request?.uri || props.flow?.raw_request?.url || '')
  if (!uri) return []
  const clean = uri.split('?')[0].trim()
  const parts = clean.split('/').filter(Boolean)
  const candidates: string[] = []
  if (clean.startsWith('/')) candidates.push(clean)
  if (clean) candidates.push(clean.replace(/^\//, ''))
  for (let i = 0; i < parts.length; i++) {
    const prefix = '/' + parts.slice(0, i + 1).join('/')
    candidates.push(prefix)
    candidates.push(parts[i])
  }
  return Array.from(new Set(candidates.filter(Boolean)))
})

const nonPathWords = computed(() => {
  const pathSet = new Set(pathWords.value)
  const used = new Set([...pathWords.value, ...payloadHints.value.map(h => h.pattern), ...responseHints.value.map(h => h.pattern)])
  return props.uniqueWords.filter(word => !pathSet.has(word) && !used.has(word))
})

const payloadHints = computed<BanCandidate[]>(() => {
  const flow = props.flow
  if (!flow) return []
  const hints: BanCandidate[] = []
  collectSuspicious(hints, flow.raw_request?.query, 'query', 'C')
  collectSuspicious(hints, flow.raw_request?.headers, 'header', 'C')
  collectSuspicious(hints, flow.raw_request?.body, 'payload', 'C')
  return uniqueCandidates(hints).slice(0, 30)
})

const responseHints = computed<BanCandidate[]>(() => {
  const flow = props.flow
  if (!flow) return []
  const hints: BanCandidate[] = []
  const status = Number(flow.raw_response?.status || flow.response_code || 0)
  if (status) hints.push({ pattern: String(status), mode: 'S', label: `status ${status}` })
  const headers = flow.raw_response?.headers
  if (headers && typeof headers === 'object') {
    for (const [key, value] of Object.entries(headers)) {
      const text = valueToString(value)
      const lower = key.toLowerCase()
      if (['content-type', 'server', 'x-powered-by', 'location'].includes(lower) && text) {
        hints.push({ pattern: text, mode: 'S', label: `${key}: ${shorten(text, 32)}` })
      }
    }
  }
  collectSuspicious(hints, flow.raw_response?.body, 'response', 'S')
  return uniqueCandidates(hints).slice(0, 30)
})

const markerHints = computed<BanCandidate[]>(() => {
  const flow = props.flow
  if (!flow?.marks?.length) return []
  const text = `${valueToString(flow.raw_request)}\n${valueToString(flow.raw_response)}`
  const hints: BanCandidate[] = []
  for (const mark of flow.marks) {
    try {
      const re = compileMarkRegex(mark.regex)
      for (const match of text.matchAll(re)) {
        if (!match[0] || match[0].length > 160) continue
        hints.push({ pattern: match[0], mode: 'B', label: mark.name || mark.regex, color: mark.color })
      }
    } catch {}
  }
  return uniqueCandidates(hints).slice(0, 30)
})

const selectedItems = computed(() => Array.from(selectedWords.value).map(parseKey))

function keyFor(pattern: string, mode: BanMode) {
  return `${mode}:${pattern}`
}

function parseKey(key: string) {
  const mode = key.slice(0, 1) as BanMode
  const pattern = key.slice(2)
  return { key, mode, pattern }
}

function isSelectedPattern(pattern: string) {
  return Array.from(selectedWords.value).some(key => key.slice(2) === pattern)
}

function toggleWord(word: string, mode: BanMode) {
  const key = keyFor(word, mode)
  if (selectedWords.value.has(key)) {
    selectedWords.value.delete(key)
  } else {
    for (const existing of Array.from(selectedWords.value)) {
      if (existing.slice(2) === word) selectedWords.value.delete(existing)
    }
    selectedWords.value.add(key)
  }
  selectedWords.value = new Set(selectedWords.value)
}

function cycleMode(item: { pattern: string; mode: BanMode }) {
  const next = item.mode === 'B' ? 'C' : item.mode === 'C' ? 'S' : 'B'
  selectedWords.value.delete(keyFor(item.pattern, item.mode))
  selectedWords.value.add(keyFor(item.pattern, next))
  selectedWords.value = new Set(selectedWords.value)
}

function addCustomWord() {
  if (!customWord.value.trim()) return
  selectedWords.value.add(keyFor(customWord.value.trim(), customMode.value))
  selectedWords.value = new Set(selectedWords.value)
  customWord.value = ''
}

function banWords() {
  emit('banWords', selectedItems.value.map(item => ({ pattern: item.pattern, mode: item.mode })))
  selectedWords.value.clear()
}

function collectSuspicious(out: BanCandidate[], src: unknown, source: string, mode: BanMode) {
  if (!src) return
  if (typeof src === 'string') {
    collectFromText(out, src, source, mode)
    return
  }
  if (typeof src === 'object') {
    for (const [key, value] of Object.entries(src as Record<string, unknown>)) {
      const text = valueToString(value)
      if (isSuspiciousKey(key)) out.push({ pattern: key, mode, label: `${source} key: ${key}` })
      if (isSuspiciousValue(text)) out.push({ pattern: text, mode, label: `${key}=${shorten(text, 36)}` })
      collectFromText(out, text, key, mode)
    }
  }
}

function collectFromText(out: BanCandidate[], text: string, source: string, mode: BanMode) {
  const snippets = text.match(/[A-Za-z0-9_./?&=%:-]{6,96}/g) || []
  for (const snippet of snippets) {
    if (isSuspiciousValue(snippet)) out.push({ pattern: snippet, mode, label: `${source}: ${shorten(snippet, 42)}` })
  }
}

function isSuspiciousKey(key: string) {
  return /(flag|token|secret|pass|auth|cmd|exec|file|path|url|admin|debug|shell|sploit|payload)/i.test(key)
}

function isSuspiciousValue(value: string) {
  if (!value || value.length < 6) return false
  return value.length > 18 || /(flag|token|secret|pass|admin|cmd|exec|shell|sploit|payload|\.php|\.sh|\.py|\.env|\/etc\/|base64|select|union|script)/i.test(value) || /[?&=:%/]{2,}/.test(value)
}

function uniqueCandidates(items: BanCandidate[]) {
  const seen = new Set<string>()
  return items.filter(item => {
    const key = keyFor(item.pattern, item.mode)
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

function valueToString(value: unknown): string {
  if (Array.isArray(value)) return value.map(valueToString).join(', ')
  if (value && typeof value === 'object') return JSON.stringify(value)
  return String(value || '')
}

function shorten(value: string, max: number) {
  return value.length > max ? `${value.slice(0, max - 1)}…` : value
}

function modeLabel(mode: BanMode) {
  if (mode === 'C') return 'request'
  if (mode === 'S') return 'response'
  return 'both'
}

function compileMarkRegex(regex: string) {
  let source = regex
  let flags = 'g'
  if (source.startsWith('(?i)')) {
    source = source.slice(4)
    flags += 'i'
  }
  return new RegExp(source, flags)
}

watch(selectedItems, () => {
  if (impactTimer) clearTimeout(impactTimer)
  impactTimer = setTimeout(fetchImpact, 250)
})

watch(() => props.initialSelection, (value) => {
  if (value && value.trim()) {
    selectedWords.value.add(keyFor(value.trim(), 'B'))
    selectedWords.value = new Set(selectedWords.value)
  }
}, { immediate: true })

async function fetchImpact() {
  if (!props.flow || selectedItems.value.length === 0) {
    impact.value = { groups: 0, flows: 0, checkers: 0 }
    return
  }
  try {
    const { data } = await api.post('/patterns/preview', { service_id: props.flow.service_id, rules: selectedItems.value })
    impact.value = { groups: data.groups || 0, flows: data.flows || 0, checkers: data.checkers || 0 }
    impactTotal.value = data.total_flows || 0
  } catch (e) { console.error('Failed to preview ban impact:', e) }
}

function markerStyle(hint: BanCandidate) {
  const selected = isSelectedPattern(hint.pattern)
  const color = hint.color || '#ef4444'
  return { borderColor: color, backgroundColor: selected ? `${color}66` : `${color}22`, color }
}

function showHint(event: MouseEvent, text: string) { tooltip.value = { text, x: event.clientX + 12, y: event.clientY + 12 } }
function moveHint(event: MouseEvent) { if (tooltip.value.text) tooltip.value = { ...tooltip.value, x: event.clientX + 12, y: event.clientY + 12 } }
function hideHint() { tooltip.value = { text: '', x: 0, y: 0 } }
</script>

<style scoped>
.dialog-overlay { position: fixed; inset: 0; background-color: rgba(0,0,0,0.6); backdrop-filter: blur(4px); z-index: 1000; display: flex; align-items: center; justify-content: center; }
.dialog { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 24px; max-width: 700px; width: 95%; max-height: 85vh; overflow-y: auto; box-shadow: 0 20px 60px rgba(0,0,0,0.4); }
.dialog-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.dialog-title { font-size: 20px; font-weight: 600; margin: 0; }
.dialog-close { background: none; border: none; cursor: pointer; padding: 4px; border-radius: 4px; color: var(--muted-foreground); transition: all 0.15s; }
.dialog-close:hover { filter: brightness(1.2); }
.flow-info-card { display: flex; gap: 8px; align-items: center; padding: 12px; border-radius: 8px; background-color: var(--surface); border: 1px solid var(--border); margin-bottom: 16px; }
.custom-input-row { display: flex; gap: 8px; margin-bottom: 16px; }
.custom-input-row .input { flex: 1; }
.words-section, .selected-section { margin-bottom: 16px; }
.words-section h3, .selected-section h3 { font-size: 14px; font-weight: 600; margin: 0 0 8px 0; color: var(--text-muted); }
.word-chips { display: flex; flex-wrap: wrap; gap: 6px; }
.word-chip { padding: 6px 12px; border-radius: 6px; font-size: 13px; cursor: pointer; border: 1px solid var(--border); background-color: var(--surface); color: var(--text); transition: all 0.15s; user-select: none; }
.word-chip:hover { filter: brightness(1.1); }
.word-chip.selected { background-color: var(--destructive); color: var(--destructive-foreground); border-color: var(--destructive); }
.selected-ban-chip { background-color: rgba(239, 68, 68, 0.16) !important; border-color: rgba(239, 68, 68, 0.7) !important; color: #fca5a5 !important; }
.path-chip { background-color: rgba(59, 130, 246, 0.16); border-color: rgba(59, 130, 246, 0.65); color: #93c5fd; font-weight: 600; }
.path-chip.selected { background-color: #2563eb; border-color: #60a5fa; color: #fff; }
.payload-chip { background-color: rgba(245, 158, 11, 0.14); border-color: rgba(245, 158, 11, 0.55); color: #fbbf24; font-weight: 600; }
.payload-chip.selected { background-color: #d97706; border-color: #fbbf24; color: #fff; }
.response-chip { background-color: rgba(16, 185, 129, 0.14); border-color: rgba(16, 185, 129, 0.55); color: #6ee7b7; font-weight: 600; }
.response-chip.selected { background-color: #059669; border-color: #6ee7b7; color: #fff; }
.word-chip small { margin-left: 6px; opacity: 0.75; font-size: 11px; }
.empty-state { padding: 16px; text-align: center; color: var(--text-muted); font-size: 14px; }
.dialog-footer { position: sticky; bottom: 0; display: flex; justify-content: flex-end; align-items: center; gap: 8px; padding-top: 12px; border-top: 1px solid var(--border); background: var(--card); z-index: 5; }
.footer-impact { margin-right: auto; display: flex; align-items: center; gap: 8px; flex-wrap: wrap; color: var(--text-muted); font-size: 12px; }
.footer-impact span:not(.impact-warning) { padding: 3px 8px; border-radius: 999px; border: 1px solid var(--border); background: var(--surface); }
.impact-warning { color: var(--destructive); font-weight: 700; }
.mark-tooltip { position: fixed; z-index: 1300; pointer-events: none; padding: 6px 9px; border-radius: 6px; background: var(--card); border: 1px solid var(--border); color: var(--text); box-shadow: 0 8px 24px rgba(0,0,0,.35); font-size: 12px; }
.mono { font-family: 'JetBrains Mono', monospace; }
.label { font-size: 12px; font-weight: 500; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }
.flex-1 { flex: 1; }
.mode-select { width: 170px; }
</style>
