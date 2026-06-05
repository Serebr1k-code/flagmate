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
      <div v-for="mark in marks" :key="mark.id" class="mark-card">
        <span class="color-dot" :style="{ backgroundColor: mark.color }"></span>
        <div class="mark-main">
          <b>{{ mark.name || mark.regex }}</b>
          <code>{{ mark.regex }}</code>
        </div>
        <button class="btn btn-sm btn-destructive" @click="deleteMark(mark.id)">Delete</button>
      </div>
      <div v-if="marks.length === 0" class="empty-state">No marks yet</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/utils/api'
import type { Mark } from '@/types'

const marks = ref<Mark[]>([])
const draft = ref({ name: '', regex: '', color: '#ef4444' })

async function fetchMarks() {
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

async function loadDefaults() {
  await api.post('/marks/defaults')
  await fetchMarks()
}

onMounted(fetchMarks)
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
.mark-card { display: flex; align-items: center; gap: 12px; padding: 12px; border: 1px solid var(--border); border-radius: 10px; background: var(--card); }
.color-dot { width: 16px; height: 16px; border-radius: 50%; border: 1px solid var(--border); flex: 0 0 auto; }
.mark-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.mark-main code { color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.text-muted { color: var(--text-muted); }
.empty-state { padding: 24px; text-align: center; color: var(--text-muted); }
</style>
