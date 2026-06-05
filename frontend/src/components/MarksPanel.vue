<template>
  <div class="marks-page">
    <div class="page-header">
      <div>
        <h1>Marks</h1>
        <p class="text-muted">Regex highlights shown on flow rows and inside payloads.</p>
      </div>
      <button class="btn btn-outline" @click="loadDefaults">Load default</button>
    </div>

    <div class="card form-row">
      <input v-model="draft.name" class="input" placeholder="Name (optional)" />
      <input v-model="draft.regex" class="input regex-input" placeholder="Regex" />
      <input v-model="draft.color" class="input color-input" type="color" />
      <button class="btn btn-primary" @click="createMark">Add mark</button>
    </div>

    <div class="marks-list">
      <div
        v-for="(mark, index) in marks"
        :key="mark.id"
        class="mark-card"
        :class="{ disabled: !mark.active, dragging: draggingId === mark.id, 'insert-before': insertIndex === index && draggingId !== mark.id, 'insert-after': insertIndex === index + 1 && draggingId !== mark.id }"
        draggable="true"
        @dragstart="onDragStart($event, mark.id)"
        @dragover.prevent="onDragOver($event, index)"
        @drop.prevent="onDrop"
        @dragend="clearDrag"
      >
        <span class="color-dot" :style="{ backgroundColor: mark.color }"></span>
        <div class="mark-main">
          <div class="mark-title">
            <b>{{ mark.name || mark.regex }}</b>
            <span class="count-chip">flows: {{ mark.flows || 0 }}</span>
            <span class="count-chip">groups: {{ mark.groups || 0 }}</span>
          </div>
          <code>{{ mark.regex }}</code>
        </div>
        <button class="btn btn-sm btn-outline" @click="toggleEnabled(mark)">{{ mark.active ? 'Disable' : 'Enable' }}</button>
        <button class="btn btn-sm mark-danger" @click="toggleBan(mark)">
          {{ mark.banned ? 'Unban' : 'Ban' }}
        </button>
        <button class="btn btn-sm mark-danger" @click="deleteMark(mark.id)">Delete</button>
      </div>
      <div v-if="marks.length === 0" class="empty-state">No marks yet</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import api from '@/utils/api'
import type { Mark } from '@/types'

const marks = ref<Mark[]>([])
const draft = ref({ name: '', regex: '', color: '#ef4444' })
const draggingId = ref<number | null>(null)
const insertIndex = ref<number | null>(null)
let refreshTimer: ReturnType<typeof setInterval> | null = null

async function fetchMarks() {
  if (draggingId.value !== null) return
  const { data } = await api.get('/marks')
  marks.value = data || []
}

async function createMark() {
  if (!draft.value.regex.trim()) return
  await api.post('/marks', draft.value)
  draft.value = { name: '', regex: '', color: '#ef4444' }
  await fetchMarks()
}

async function deleteMark(id: number) {
  await api.delete(`/marks/${id}`)
  await fetchMarks()
}

async function toggleBan(mark: Mark) {
  await api.post(`/marks/${mark.id}/${mark.banned ? 'unban' : 'ban'}`)
  await fetchMarks()
}

async function toggleEnabled(mark: Mark) {
  await api.post(`/marks/${mark.id}/toggle`, { active: !mark.active })
  await fetchMarks()
}

function onDragStart(event: DragEvent, id: number) {
  const target = event.target as HTMLElement | null
  if (target?.closest('button,input,select,textarea,a')) {
    event.preventDefault()
    return
  }
  event.dataTransfer?.setData('text/plain', String(id))
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
  draggingId.value = id
}

function onDragOver(event: DragEvent, index: number) {
  if (draggingId.value === null) return
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
  const el = event.currentTarget as HTMLElement
  const midpoint = el.getBoundingClientRect().top + el.offsetHeight / 2
  insertIndex.value = event.clientY < midpoint ? index : index + 1
}

async function onDrop() {
  if (draggingId.value === null || insertIndex.value === null) return clearDrag()
  const from = marks.value.findIndex(mark => mark.id === draggingId.value)
  if (from < 0) return clearDrag()
  const next = [...marks.value]
  const [moved] = next.splice(from, 1)
  let to = insertIndex.value
  if (from < to) to--
  to = Math.max(0, Math.min(next.length, to))
  if (from === to) return clearDrag()
  next.splice(to, 0, moved)
  marks.value = next
  clearDrag()
  await api.post('/marks/reorder', { ids: next.map(mark => mark.id) })
  await fetchMarks()
}

function clearDrag() {
  draggingId.value = null
  insertIndex.value = null
}

async function loadDefaults() {
  await api.post('/marks/defaults')
  await fetchMarks()
}

function startRefresh() {
  fetchMarks()
  refreshTimer = setInterval(fetchMarks, 2000)
  window.addEventListener('focus', fetchMarks)
}

function stopRefresh() {
  if (refreshTimer) clearInterval(refreshTimer)
  window.removeEventListener('focus', fetchMarks)
}

onMounted(startRefresh)
onUnmounted(stopRefresh)
</script>

<style scoped>
.marks-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.page-header p { margin: 4px 0 0; }
.card { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 16px; }
.form-row { display: grid; grid-template-columns: 180px 1fr 56px auto; gap: 10px; align-items: center; }
.regex-input { font-family: 'JetBrains Mono', monospace; }
.color-input { padding: 4px; }
.marks-list { display: flex; flex-direction: column; gap: 10px; }
.mark-card { position: relative; display: flex; align-items: center; gap: 12px; padding: 12px; border: 1px solid var(--border); border-radius: 10px; background: var(--card); cursor: grab; transition: transform .12s ease, border-color .12s ease, box-shadow .12s ease, opacity .12s ease; }
.mark-card:hover { border-color: color-mix(in srgb, var(--primary) 55%, var(--border)); }
.mark-card.dragging { opacity: 0.42; transform: scale(0.985); box-shadow: 0 10px 30px rgba(0,0,0,0.28); cursor: grabbing; }
.mark-card.insert-before::before, .mark-card.insert-after::after { content: ''; position: absolute; left: 10px; right: 10px; height: 3px; border-radius: 999px; background: #22c55e; box-shadow: 0 0 12px rgba(34,197,94,.7); }
.mark-card.insert-before::before { top: -7px; }
.mark-card.insert-after::after { bottom: -7px; }
.mark-card.disabled { opacity: 0.55; }
.mark-card button, .mark-card input, .mark-card select, .mark-card textarea { cursor: default; }
.color-dot { width: 16px; height: 16px; border-radius: 50%; border: 1px solid var(--border); flex: 0 0 auto; }
.mark-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.mark-title { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.mark-main code { color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.count-chip { padding: 2px 7px; border-radius: 999px; border: 1px solid var(--border); background: var(--surface); color: var(--text-muted); font-size: 11px; font-weight: 600; }
.mark-danger { border: 1px solid var(--destructive); color: var(--destructive); background: transparent; }
.mark-danger:hover { background: var(--destructive); color: var(--destructive-foreground); }
.text-muted { color: var(--text-muted); }
.empty-state { padding: 24px; text-align: center; color: var(--text-muted); }
</style>
