<template>
  <div class="service-manager-page">
    <div class="page-header">
      <h1>Services</h1>
      <button class="btn btn-primary" @click="showForm = true">+ Add Service</button>
    </div>

    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Name</th><th>Port</th><th>Protocol</th><th>Created</th><th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="service in services" :key="service.id" class="service-row" @click="emit('open-service-stats', service.id)">
            <td class="font-medium">{{ service.name }}</td>
            <td class="mono">{{ service.port }}</td>
            <td><span class="badge badge-outline">{{ service.protocol }}</span></td>
            <td class="text-muted">{{ formatTime(service.created_at) }}</td>
            <td>
              <button class="btn btn-sm btn-destructive" @click.stop="deleteService(service.id)">Delete</button>
            </td>
          </tr>
          <tr v-if="services.length === 0"><td colspan="5" class="empty-state">No services configured</td></tr>
        </tbody>
      </table>
    </div>

    <Teleport to="body">
      <div v-if="showForm" class="dialog-overlay" @click.self="showForm = false">
        <div class="dialog">
          <div class="dialog-header">
            <h2 class="dialog-title">Add Service</h2>
            <button class="dialog-close" @click="showForm = false">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
          </div>

          <form @submit.prevent="addService" class="form">
            <div class="form-group">
              <label class="label">Name</label>
              <input v-model="form.name" class="input" placeholder="e.g. web-service" required />
            </div>
            <div class="form-group">
              <label class="label">Port</label>
              <input v-model.number="form.port" type="number" class="input" placeholder="e.g. 8080" min="1" max="65535" required />
            </div>
            <div class="form-group">
              <label class="label">Protocol</label>
              <select v-model="form.protocol" class="select">
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
            </div>

            <div class="dialog-footer">
              <button type="button" class="btn btn-outline" @click="showForm = false">Cancel</button>
              <button type="submit" class="btn btn-primary" :disabled="loading">{{ loading ? 'Adding...' : 'Add Service' }}</button>
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
import type { Service } from '@/types'

const services = ref<Service[]>([])
const emit = defineEmits<{ 'open-service-stats': [serviceId: number] }>()
const showForm = ref(false)
const loading = ref(false)
const form = ref({ name: '', port: 0, protocol: 'tcp' })

async function fetchServices() {
  try {
    const { data } = await api.get('/services')
    services.value = data
  } catch (e) { console.error('Failed to fetch services:', e) }
}

async function addService() {
  loading.value = true
  try {
    await api.post('/services', form.value)
    form.value = { name: '', port: 0, protocol: 'tcp' }
    showForm.value = false
    await fetchServices()
  } catch (e) { console.error('Failed to add service:', e) }
  finally { loading.value = false }
}

async function deleteService(id: number) {
  try {
    await api.delete(`/services/${id}`)
    await fetchServices()
  } catch (e) { console.error('Failed to delete service:', e) }
}

function formatTime(ts: string) { return new Date(ts).toLocaleString() }

onMounted(fetchServices)
</script>

<style scoped>
.service-manager-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.table-container { width: 100%; overflow-x: auto; border: 1px solid var(--border); border-radius: 8px; }
.table { width: 100%; border-collapse: collapse; }
.table th { padding: 12px 16px; text-align: left; font-size: 13px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; border-bottom: 1px solid var(--border); background-color: var(--surface); color: var(--text-muted); }
.table td { padding: 12px 16px; font-size: 14px; border-bottom: 1px solid var(--border); }
.table tbody tr:hover { filter: brightness(1.05); }
.service-row { cursor: pointer; }
.dialog-overlay { position: fixed; inset: 0; background-color: rgba(0,0,0,0.6); backdrop-filter: blur(4px); z-index: 1000; display: flex; align-items: center; justify-content: center; }
.dialog { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 24px; max-width: 500px; width: 90%; box-shadow: 0 20px 60px rgba(0,0,0,0.4); }
.dialog-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.dialog-title { font-size: 20px; font-weight: 600; margin: 0; }
.dialog-close { background: none; border: none; cursor: pointer; padding: 4px; border-radius: 4px; color: var(--muted-foreground); transition: all 0.15s; }
.dialog-close:hover { filter: brightness(1.2); }
.form { display: flex; flex-direction: column; gap: 16px; }
.form-group { display: flex; flex-direction: column; gap: 4px; }
.dialog-footer { display: flex; justify-content: flex-end; gap: 8px; }
.mono { font-family: 'JetBrains Mono', monospace; }
.font-medium { font-weight: 500; }
.text-muted { color: var(--text-muted); }
.label { font-size: 12px; font-weight: 500; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); }
</style>
