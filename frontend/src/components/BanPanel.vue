<template>
  <div class="ban-panel-page">
    <div class="page-header">
      <h1>Ban Panel</h1>
      <p class="text-muted">Regex patterns used for banning traffic</p>
    </div>

    <div class="add-pattern-card card">
      <div class="add-pattern-row">
        <input
          v-model="newPattern"
          class="input flex-1"
          placeholder="Enter word or regex pattern..."
          @keydown.enter="addPattern"
        />
        <select v-model="newPatternMode" class="select">
          <option value="C">Client→Server</option>
          <option value="S">Server→Client</option>
          <option value="B">Both directions</option>
        </select>
        <button class="btn btn-destructive" @click="addPattern">Add Pattern</button>
      </div>
    </div>

    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Pattern</th>
            <th>Mode</th>
            <th>Matches</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in patterns" :key="p.id">
            <td class="mono text-sm">{{ p.pattern }}</td>
            <td>
              <span class="badge badge-outline">{{ p.mode }}</span>
            </td>
            <td>{{ p.match_count || 0 }}</td>
            <td>
              <span class="badge" :class="p.active ? 'badge-success' : 'badge-warning'">
                {{ p.active ? 'Active' : 'Disabled' }}
              </span>
            </td>
            <td>
              <button
                class="btn btn-sm btn-ghost"
                @click="togglePattern(p.id, !p.active)"
              >
                {{ p.active ? 'Disable' : 'Enable' }}
              </button>
              <button
                class="btn btn-sm btn-destructive"
                @click="deletePattern(p.id)"
              >
                Delete
              </button>
            </td>
          </tr>
          <tr v-if="patterns.length === 0">
            <td colspan="5" class="empty-state">No ban patterns configured</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/utils/api'

interface BanPattern {
  id: number
  pattern: string
  mode: string
  match_count: number
  active: boolean
}

const patterns = ref<BanPattern[]>([])
const newPattern = ref('')
const newPatternMode = ref('B')

async function fetchPatterns() {
  try {
    const { data } = await api.get('/patterns')
    patterns.value = data
  } catch (e) {
    console.error('Failed to fetch patterns:', e)
  }
}

async function addPattern() {
  if (!newPattern.value.trim()) return
  try {
    await api.post('/patterns', {
      pattern: newPattern.value.trim(),
      description: `Ban pattern (${newPatternMode.value})`,
      mode: newPatternMode.value,
    })
    newPattern.value = ''
    await fetchPatterns()
  } catch (e) {
    console.error('Failed to add pattern:', e)
  }
}

async function togglePattern(id: number, active: boolean) {
  try {
    await api.post(`/patterns/${id}/toggle`, { active })
    await fetchPatterns()
  } catch (e) {
    console.error('Failed to toggle pattern:', e)
  }
}

async function deletePattern(id: number) {
  try {
    await api.delete(`/patterns/${id}`)
    await fetchPatterns()
  } catch (e) {
    console.error('Failed to delete pattern:', e)
  }
}

onMounted(fetchPatterns)
</script>

<style scoped>
.ban-panel-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; flex-direction: column; gap: 4px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.page-header p { font-size: 14px; margin: 0; }
.add-pattern-card { border: 1px solid var(--border); border-radius: 8px; padding: 16px; background-color: var(--card); }
.add-pattern-row { display: flex; gap: 8px; align-items: center; }
.add-pattern-row .input { flex: 1; }
.table-container { width: 100%; overflow-x: auto; border: 1px solid var(--border); border-radius: 8px; background-color: var(--card); }
.table { width: 100%; border-collapse: collapse; }
.table th { padding: 12px 16px; text-align: left; font-size: 13px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; border-bottom: 1px solid var(--border); background-color: var(--surface); color: var(--text-muted); }
.table td { padding: 12px 16px; font-size: 14px; border-bottom: 1px solid var(--border); }
.table tbody:hover { filter: brightness(1.02); }
.mono { font-family: 'JetBrains Mono', monospace; }
.text-sm { font-size: 13px; }
.text-muted { color: var(--text-muted); }
.empty-state { text-align: center; padding: 32px; color: var(--text-muted); }
.flex-1 { flex: 1; }
</style>
