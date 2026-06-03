<template>
  <div class="pattern-manager-page">
    <div class="page-header">
      <h1>Ban Patterns</h1>
      <button class="btn btn-primary" @click="showForm = true">+ Add Pattern</button>
    </div>

    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Pattern</th><th>Description</th><th>Created</th><th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="pattern in patterns" :key="pattern.id">
            <td><code class="code-block">{{ pattern.pattern }}</code></td>
            <td class="text-muted">{{ pattern.description || '—' }}</td>
            <td class="text-muted">{{ formatTime(pattern.created_at) }}</td>
            <td>
              <button class="btn btn-sm btn-destructive" @click="deletePattern(pattern.id)">Delete</button>
            </td>
          </tr>
          <tr v-if="patterns.length === 0"><td colspan="4" class="empty-state">No ban patterns configured</td></tr>
        </tbody>
      </table>
    </div>

    <Teleport to="body">
      <div v-if="showForm" class="dialog-overlay" @click.self="showForm = false">
        <div class="dialog">
          <div class="dialog-header">
            <h2 class="dialog-title">Add Pattern</h2>
            <button class="dialog-close" @click="showForm = false">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
          </div>

          <form @submit.prevent="addPattern" class="form">
            <div class="form-group">
              <label class="label">Regex Pattern</label>
              <input v-model="form.pattern" class="input" placeholder="e.g. FLAG\{[^\}]+\}" required />
            </div>
            <div class="form-group">
              <label class="label">Description</label>
              <input v-model="form.description" class="input" placeholder="e.g. Flag format pattern" />
            </div>

            <div class="dialog-footer">
              <button type="button" class="btn btn-outline" @click="showForm = false">Cancel</button>
              <button type="submit" class="btn btn-primary" :disabled="loading">{{ loading ? 'Adding...' : 'Add Pattern' }}</button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/utils/api'
import type { Pattern } from '@/types'

const patterns = ref<Pattern[]>([])
const showForm = ref(false)
const loading = ref(false)
const form = ref({ pattern: '', description: '' })

async function fetchPatterns() {
  try {
    const { data } = await api.get('/patterns')
    patterns.value = data
  } catch (e) { console.error('Failed to fetch patterns:', e) }
}

async function addPattern() {
  loading.value = true
  try {
    await api.post('/patterns', form.value)
    form.value = { pattern: '', description: '' }
    showForm.value = false
    await fetchPatterns()
  } catch (e) { console.error('Failed to add pattern:', e) }
  finally { loading.value = false }
}

async function deletePattern(id: number) {
  try {
    await api.delete(`/patterns/${id}`)
    await fetchPatterns()
  } catch (e) { console.error('Failed to delete pattern:', e) }
}

function formatTime(ts: string) { return new Date(ts).toLocaleString() }

onMounted(fetchPatterns)
</script>

<style scoped>
.pattern-manager-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.table-container { width: 100%; overflow-x: auto; border: 1px solid var(--border); border-radius: 8px; }
.table { width: 100%; border-collapse: collapse; }
.table th { padding: 12px 16px; text-align: left; font-size: 13px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; border-bottom: 1px solid var(--border); background-color: var(--surface); color: var(--text-muted); }
.table td { padding: 12px 16px; font-size: 14px; border-bottom: 1px solid var(--border); }
.table tbody tr:hover { filter: brightness(1.05); }
.code-block { background-color: var(--surface); color: var(--accent); border: 1px solid var(--border); border-radius: 4px; padding: 4px 8px; font-family: 'JetBrains Mono', monospace; font-size: 13px; white-space: pre; }
.dialog-overlay { position: fixed; inset: 0; background-color: rgba(0,0,0,0.6); backdrop-filter: blur(4px); z-index: 1000; display: flex; align-items: center; justify-content: center; }
.dialog { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 24px; max-width: 500px; width: 90%; box-shadow: 0 20px 60px rgba(0,0,0,0.4); }
.dialog-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.dialog-title { font-size: 20px; font-weight: 600; margin: 0; }
.dialog-close { background: none; border: none; cursor: pointer; padding: 4px; border-radius: 4px; color: var(--muted-foreground); transition: all 0.15s; }
.dialog-close:hover { filter: brightness(1.2); }
.form { display: flex; flex-direction: column; gap: 16px; }
.form-group { display: flex; flex-direction: column; gap: 4px; }
.dialog-footer { display: flex; justify-content: flex-end; gap: 8px; }
.text-muted { color: var(--text-muted); }
.label { font-size: 12px; font-weight: 500; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); }
</style>
