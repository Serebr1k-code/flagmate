<template>
  <div v-if="selectedAttempt" class="mirroring-page attempt-detail-page">
    <div class="page-header">
      <div>
        <h1>Mirror attempt</h1>
        <p class="text-muted mono">{{ selectedAttempt.target_ip }}:{{ selectedAttempt.target_port }} · {{ formatTime(selectedAttempt.created_at) }}</p>
      </div>
      <button class="btn btn-outline" @click="selectedAttempt = null">Back to mirroring</button>
    </div>
    <div class="card attempt-detail-card">
      <div class="attempt-summary-grid">
        <div><span>Status</span><b :class="selectedAttempt.success ? 'text-success' : 'text-danger'">{{ selectedAttempt.success ? 'success' : 'miss' }}</b></div>
        <div><span>Target</span><b class="mono">{{ selectedAttempt.target_ip }}:{{ selectedAttempt.target_port }}</b></div>
        <div><span>Flow</span><b class="mono">{{ shortId(selectedAttempt.flow_id) }}</b></div>
        <div><span>Flags</span><b>{{ selectedAttempt.flag || '-' }}</b></div>
      </div>
      <div class="attempt-transcript">
        <div class="block-header"><span>mirror response</span></div>
        <pre>{{ selectedAttempt.response || 'No response captured' }}</pre>
      </div>
    </div>
  </div>
  <div v-else class="mirroring-page">
    <div class="page-header">
      <div>
        <h1>Mirroring</h1>
        <p class="text-muted">Global switch, per-service intervals, and persistent mirrored group names.</p>
      </div>
      <div class="toggle-container">
        <span class="text-muted">Enabled</span>
        <div class="toggle" :class="{ active: config.enabled }" @click="toggleMirroring"></div>
      </div>
    </div>

    <div class="service-grid">
      <div v-for="service in services" :key="service.id" class="card service-card">
        <div class="service-head">
          <div>
            <h3>{{ service.name }}</h3>
            <p class="mono text-muted">{{ service.protocol }} :{{ service.port }} · {{ bannedCounts[service.id] || 0 }} banned flows</p>
          </div>
          <div class="interval-row">
            <label>Every</label>
            <input v-model.number="serviceConfig(service.id).interval_seconds" class="input interval-input" type="number" min="1" @change="saveConfig" />
            <span>seconds</span>
          </div>
        </div>

        <div class="mirrored-list">
          <div v-for="group in groupsForService(service.id)" :key="group.hash" class="mirror-group-wrap">
            <div class="mirror-group" @click="toggleGroup(group.hash)">
              <div class="group-main">
              <input
                :value="draftNames[group.hash] ?? group.name"
                class="input name-input"
                placeholder="Group name"
                @input="draftNames[group.hash] = ($event.target as HTMLInputElement).value"
                @change="renameGroup(group)"
                @click.stop
              />
              <div class="mono target-line">{{ displayGroup(group) }}</div>
              <div class="text-muted small">{{ group.count }} streams · latest {{ formatTime(group.last_seen) }}</div>
              </div>
              <button class="btn btn-sm btn-destructive" @click.stop="unmirror(group)">Remove</button>
            </div>
            <div v-if="expandedGroup === group.hash" class="team-drilldown">
              <button
                v-for="target in config.targets"
                :key="target.ip"
                class="team-pill"
                :class="{ active: selectedAttemptKey === attemptKey(group.hash, target.ip) }"
                @click="fetchAttempts(group.hash, target.ip)"
              >
                {{ target.ip }} <span>{{ teamSummary(target.ip) }}</span>
              </button>
              <div v-if="selectedAttemptKey && selectedAttemptKey.startsWith(group.hash + '|')" class="attempt-list">
                <div v-for="attempt in selectedAttempts" :key="attempt.id" class="attempt-row" :class="{ success: attempt.success }" @click="selectedAttempt = attempt">
                  <div>
                    <span class="attempt-status" :class="attempt.success ? 'ok' : 'miss'">{{ attempt.success ? 'success' : 'miss' }}</span>
                    <span class="mono">{{ attempt.target_ip }}:{{ attempt.target_port }}</span>
                    <span class="mono flow-id">{{ shortId(attempt.flow_id) }}</span>
                    <span v-if="attempt.flag" class="flag-chip">{{ attempt.flag }}</span>
                  </div>
                  <span class="attempt-preview">{{ responsePreview(attempt.response) }}</span>
                  <span class="text-muted small">{{ formatTime(attempt.created_at) }}</span>
                </div>
                <div v-if="selectedAttempts.length === 0" class="empty-state">No attempts for this group/team yet</div>
              </div>
            </div>
          </div>
          <div v-if="groupsForService(service.id).length === 0 && config.targets.length === 0" class="empty-state">Add a team IP to start mirroring</div>
          <div v-else-if="groupsForService(service.id).length === 0" class="empty-state">No banned flows yet — attack traffic that gets banned will appear here</div>
        </div>
      </div>
    </div>

    <div class="card stats-card">
      <div class="card-header stats-head">
        <div>
          <h3 class="card-title">Stats</h3>
          <p class="text-muted">Mirror attempts, stolen flags, and success rates.</p>
        </div>
        <button class="btn btn-sm btn-outline" @click="fetchStats">Refresh stats</button>
      </div>
      <div class="stats-grid">
        <div class="stat-tile"><span>Requests</span><b>{{ stats.total_requests }}</b></div>
        <div class="stat-tile"><span>Flags</span><b>{{ stats.flags }}</b></div>
        <div class="stat-tile"><span>Success</span><b>{{ stats.success_rate }}%</b></div>
      </div>
      <div class="stats-columns">
        <div>
          <h4>Teams</h4>
          <div v-for="team in stats.teams" :key="team.target_ip" class="stat-row">
            <span class="mono">{{ team.target_ip }}</span>
            <b>{{ team.flags }} flags</b>
            <span>{{ team.success_rate }}%</span>
          </div>
        </div>
        <div>
          <h4>Mirrored flow types</h4>
          <div v-for="group in stats.groups" :key="group.hash || group.name" class="stat-row">
            <span>{{ group.name || group.hash?.slice(0, 8) || 'group' }}</span>
            <b>{{ group.flags }} flags</b>
            <span>{{ group.success_rate }}%</span>
          </div>
        </div>
      </div>
      <div class="chart-tabs">
        <button v-for="bucket in chartBuckets" :key="bucket" class="btn btn-sm" :class="activeBucket === bucket ? 'btn-primary' : 'btn-outline'" @click="activeBucket = bucket">{{ bucket }}</button>
      </div>
      <div class="bar-chart">
        <div v-for="point in chartSeries" :key="point.ts" class="bar-wrap" :title="`${formatTime(point.ts)} · ${point.flags} flags · ${point.requests} req`">
          <div class="bar" :style="{ height: `${barHeight(point)}%` }"></div>
        </div>
      </div>
    </div>

    <div class="card target-card">
      <div class="card-header">
        <h3 class="card-title">Mirror teams</h3>
          <p class="text-muted">Set a team IP and optional port override. Empty/0 port mirrors to the original service port.</p>
      </div>
      <div class="targets-list">
        <div v-for="(target, index) in config.targets" :key="index" class="target-item">
          <span class="mono">{{ target.ip }}:{{ target.port || '<service port>' }}</span>
          <input v-model.number="target.port" class="input target-port" type="number" min="0" max="65535" placeholder="port" @change="saveConfig" />
          <button class="btn btn-sm btn-destructive" @click="removeTarget(index)">Remove</button>
        </div>
        <div v-if="config.targets.length === 0" class="empty-state">No mirror targets</div>
      </div>
      <div class="add-target-form">
        <input v-model="newTargetIp" class="input" placeholder="Team IP" />
        <input v-model.number="newTargetPort" class="input target-port" type="number" min="0" max="65535" placeholder="Port override" />
        <button class="btn btn-primary" @click="addTarget">Add target</button>
        <button class="btn btn-outline" @click="saveConfig" :disabled="saving">{{ saving ? 'Saving...' : 'Save' }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import api from '@/utils/api'
import type { FlowGroup, MirroringConfig, Service, ServiceMirrorConfig } from '@/types'

const config = ref<MirroringConfig>({ enabled: false, targets: [], services: [] })
const services = ref<Service[]>([])
const mirroredGroups = ref<FlowGroup[]>([])
const draftNames = ref<Record<string, string>>({})
const newTargetIp = ref('')
const newTargetPort = ref(0)
const saving = ref(false)
const expandedGroup = ref('')
const selectedAttemptKey = ref('')
const selectedAttempts = ref<MirrorAttempt[]>([])
const selectedAttempt = ref<MirrorAttempt | null>(null)
const activeBucket = ref<'minute' | '10m' | '30m' | 'hour'>('10m')
const chartBuckets = ['minute', '10m', '30m', 'hour'] as const
const stats = ref<MirrorStats>({ total_requests: 0, successes: 0, success_rate: 0, flags: 0, teams: [], groups: [], series: { minute: [], '10m': [], '30m': [], hour: [] } })
const bannedCounts = ref<Record<number, number>>({})

interface MirrorAttempt { id: number; service_id: number; hash: string; flow_id: string; target_ip: string; target_port: number; success: boolean; flag: string; response: string; created_at: string }
interface StatItem { target_ip?: string; hash?: string; name?: string; requests: number; successes: number; flags: number; success_rate: number }
interface SeriesPoint { ts: string; requests: number; successes: number; flags: number; success_rate: number }
interface MirrorStats { total_requests: number; successes: number; success_rate: number; flags: number; teams: StatItem[]; groups: StatItem[]; series: Record<'minute' | '10m' | '30m' | 'hour', SeriesPoint[]> }

async function fetchConfig() {
  try {
    const [{ data: mirrorData }, { data: serviceData }, { data: groupData }, { data: statData }, { data: bannedData }] = await Promise.all([
      api.get('/mirroring'),
      api.get('/services'),
      api.get('/mirroring/groups'),
      api.get('/mirroring/stats'),
      api.get('/flow-groups', { params: { top: 500 } }),
    ])
    config.value = { enabled: false, targets: [], services: [], ...mirrorData }
    services.value = serviceData
    mirroredGroups.value = groupData
    stats.value = statData
    // Merge mirrored groups with banned groups
    const bannedGroups = (bannedData || []).filter((g: any) => g.count > 0)
    const merged = [...mirroredGroups.value]
    for (const bg of bannedGroups) {
      if (!merged.find(m => m.hash === bg.hash)) {
        merged.push(bg)
      }
    }
    mirroredGroups.value = merged
    // Count banned groups per service
    const counts: Record<number, number> = {}
    for (const g of bannedGroups) {
      const sid = g.service_id || 0
      counts[sid] = (counts[sid] || 0) + g.count
    }
    bannedCounts.value = counts
    for (const service of services.value) serviceConfig(service.id)
    for (const group of mirroredGroups.value) draftNames.value[group.hash] = group.name || ''
  } catch (e) { console.error('Failed to fetch mirroring config:', e) }
}

const chartSeries = computed(() => stats.value.series[activeBucket.value] || [])

function serviceConfig(serviceId: number): ServiceMirrorConfig {
  let cfg = config.value.services.find(s => s.service_id === serviceId)
	if (!cfg) {
		cfg = { service_id: serviceId, enabled: true, interval_seconds: 60, targets: [] }
		config.value.services.push(cfg)
	}
	cfg.enabled = true
	if (!cfg.interval_seconds || cfg.interval_seconds < 1) cfg.interval_seconds = 60
  return cfg
}

function groupsForService(serviceId: number) {
  return mirroredGroups.value.filter(group => group.service_id === serviceId)
}

async function toggleMirroring() {
  config.value.enabled = !config.value.enabled
  await saveConfig()
}

function addTarget() {
	const ip = newTargetIp.value.trim()
	if (!ip) return
	config.value.targets.push({ ip, port: Number(newTargetPort.value) || 0 })
	newTargetIp.value = ''
	newTargetPort.value = 0
	saveConfig()
}

function removeTarget(index: number) {
  config.value.targets.splice(index, 1)
  saveConfig()
}

async function renameGroup(group: FlowGroup) {
  const name = draftNames.value[group.hash] || ''
  try {
    await api.post(`/flow-groups/${group.hash}/name`, { name })
    group.name = name
  } catch (e) { console.error('Failed to rename mirrored group:', e) }
}

async function unmirror(group: FlowGroup) {
  const flowId = group.latest_flow?.id || group.example_flow_id
  if (!flowId) return
  try {
    await api.post(`/flows/${flowId}/mirror`, { enabled: false })
    mirroredGroups.value = mirroredGroups.value.filter(g => g.hash !== group.hash)
  } catch (e) { console.error('Failed to remove mirrored group:', e) }
}

function toggleGroup(hash: string) {
  expandedGroup.value = expandedGroup.value === hash ? '' : hash
  selectedAttemptKey.value = ''
  selectedAttempts.value = []
}

function attemptKey(hash: string, ip: string) {
  return `${hash}|${ip}`
}

async function fetchAttempts(hash: string, ip: string) {
  selectedAttemptKey.value = attemptKey(hash, ip)
  try {
    const { data } = await api.get('/mirroring/attempts', { params: { hash, target_ip: ip, limit: 100 } })
    selectedAttempts.value = data
  } catch (e) { console.error('Failed to fetch mirror attempts:', e) }
}

async function fetchStats() {
  try {
    const { data } = await api.get('/mirroring/stats')
    stats.value = data
  } catch (e) { console.error('Failed to fetch mirroring stats:', e) }
}

function teamSummary(ip: string) {
  const team = stats.value.teams.find(item => item.target_ip === ip)
  if (!team) return '0 flags'
  return `${team.flags} flags · ${team.success_rate}%`
}

function shortId(id: string) {
  return id ? `${id.slice(0, 8)}...` : '-'
}

function responsePreview(response: string) {
  const clean = String(response || '').replace(/\s+/g, ' ').trim()
  if (!clean) return 'no response'
  return clean.length > 120 ? clean.slice(0, 120) + '...' : clean
}

async function saveConfig() {
  saving.value = true
  try {
    config.value.services = config.value.services.map(s => ({ ...s, enabled: true, targets: [] }))
    await api.post('/mirroring', config.value)
  } catch (e) { console.error('Failed to save mirroring config:', e) }
  finally { saving.value = false }
}

function displayGroup(group: FlowGroup) {
  const flow = group.latest_flow
  const uri = group.uri || flow?.raw_request?.uri || flow?.raw_request?.url || ''
  const port = flow?.dst_port || group.destination?.match(/:(\d+)/)?.[1] || ''
  const method = group.method || flow?.raw_request?.method || 'HTTP'
  return `${method} ${port}${String(uri).startsWith('/') ? uri : `/${uri}`} -> ${group.response_code}`
}

function formatTime(ts: string) { return ts ? new Date(ts).toLocaleString() : '—' }

function barHeight(point: SeriesPoint) {
  const max = Math.max(1, ...chartSeries.value.map(item => item.flags || item.successes || item.requests))
  const value = point.flags || point.successes || point.requests
  return Math.max(8, Math.round((value / max) * 100))
}

onMounted(fetchConfig)
</script>

<style scoped>
.mirroring-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.page-header h1 { font-size: 24px; font-weight: 700; margin: 0; }
.page-header p { margin: 4px 0 0; }
.toggle-container { display: flex; align-items: center; gap: 12px; }
.toggle { position: relative; width: 44px; height: 24px; border-radius: 12px; cursor: pointer; transition: background-color 0.2s; background-color: var(--muted); }
.toggle.active { background-color: var(--primary); }
.toggle::after { content: ''; position: absolute; top: 2px; left: 2px; width: 20px; height: 20px; background-color: white; border-radius: 50%; transition: transform 0.2s; }
.toggle.active::after { transform: translateX(20px); }
.service-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(420px, 1fr)); gap: 16px; }
.card { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 16px; }
.service-card { display: flex; flex-direction: column; gap: 14px; }
.service-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.service-head h3 { margin: 0; font-size: 18px; }
.service-head p { margin: 4px 0 0; }
.interval-row { display: flex; align-items: center; gap: 8px; }
.interval-input { width: 90px; }
.interval-input::-webkit-outer-spin-button,
.interval-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.interval-input { -moz-appearance: textfield; appearance: textfield; }
.mirrored-list, .targets-list { display: flex; flex-direction: column; gap: 8px; }
.mirror-group-wrap { display: flex; flex-direction: column; gap: 8px; }
.mirror-group { display: flex; align-items: center; justify-content: space-between; gap: 12px; border: 1px solid var(--border); border-radius: 8px; padding: 10px 12px; background-color: var(--surface); }
.target-item { display: grid; grid-template-columns: 1fr 120px auto; align-items: center; gap: 12px; border: 1px solid var(--border); border-radius: 8px; padding: 10px 12px; background-color: var(--surface); }
.mirror-group { cursor: pointer; }
.mirror-group:hover { background-color: var(--surface-hover); }
.group-main { min-width: 0; flex: 1; display: flex; flex-direction: column; gap: 6px; }
.name-input { max-width: 260px; }
.target-line { font-size: 13px; font-weight: 700; }
.small { font-size: 12px; }
.add-target-form { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 12px; }
.add-target-form .input { flex: 1; min-width: 180px; }
.target-port { max-width: 140px; }
.card-header { margin-bottom: 12px; }
.card-title { font-size: 18px; font-weight: 600; margin: 0 0 4px; }
.team-drilldown { padding: 10px; border: 1px dashed var(--border); border-radius: 10px; background: color-mix(in srgb, var(--surface) 65%, transparent); }
.team-pill { margin: 0 6px 6px 0; padding: 6px 10px; border-radius: 999px; border: 1px solid var(--border); background: var(--card); color: var(--text); cursor: pointer; }
.team-pill.active { border-color: var(--primary); color: var(--primary); }
.team-pill span { margin-left: 6px; color: var(--text-muted); font-size: 11px; }
.attempt-list { display: flex; flex-direction: column; gap: 6px; margin-top: 8px; }
.attempt-row { display: grid; grid-template-columns: 1.2fr 1.4fr auto; align-items: center; gap: 10px; padding: 9px 10px; border-radius: 8px; background: var(--card); border: 1px solid var(--border); cursor: pointer; }
.attempt-row:hover { border-color: var(--primary); background: var(--surface-hover); }
.attempt-row.success { border-color: var(--success); background: color-mix(in srgb, var(--success) 12%, var(--card)); }
.attempt-row > div { display: flex; align-items: center; gap: 8px; min-width: 0; }
.attempt-status { padding: 2px 7px; border-radius: 999px; font-size: 11px; font-weight: 700; text-transform: uppercase; }
.attempt-status.ok { background: color-mix(in srgb, var(--success) 22%, transparent); color: var(--success); }
.attempt-status.miss { background: color-mix(in srgb, var(--warning) 20%, transparent); color: var(--warning); }
.attempt-preview { color: var(--text-muted); overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.flow-id { color: var(--text-muted); font-size: 12px; }
.flag-chip { margin-left: 8px; color: var(--success); font-family: 'JetBrains Mono', monospace; }
.attempt-detail-page { height: 100%; overflow-y: auto; }
.attempt-detail-card { display: flex; flex-direction: column; gap: 16px; }
.attempt-summary-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.attempt-summary-grid div { padding: 12px; border: 1px solid var(--border); border-radius: 10px; background: var(--surface); display: flex; flex-direction: column; gap: 4px; }
.attempt-summary-grid span { color: var(--text-muted); font-size: 12px; text-transform: uppercase; letter-spacing: .05em; }
.text-danger { color: var(--destructive); }
.attempt-transcript { border: 1px solid var(--border); border-radius: 10px; overflow: hidden; background: var(--surface); }
.attempt-transcript .block-header { padding: 10px 14px; border-bottom: 1px solid var(--border); color: var(--text-muted); text-transform: uppercase; font-size: 12px; letter-spacing: .05em; }
.attempt-transcript pre { margin: 0; padding: 14px; white-space: pre-wrap; word-break: break-word; font-family: 'JetBrains Mono', monospace; font-size: 12px; line-height: 1.55; }
.stats-head { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; }
.stats-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin-bottom: 14px; }
.stat-tile { padding: 12px; border-radius: 10px; background: var(--surface); border: 1px solid var(--border); display: flex; flex-direction: column; gap: 4px; }
.stat-tile span { color: var(--text-muted); font-size: 12px; }
.stat-tile b { font-size: 22px; }
.stats-columns { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.stats-columns h4 { margin: 0 0 8px; font-size: 14px; color: var(--text-muted); }
.stat-row { display: grid; grid-template-columns: 1fr auto auto; gap: 10px; align-items: center; padding: 8px 0; border-bottom: 1px solid var(--border); font-size: 13px; }
.chart-tabs { display: flex; gap: 8px; margin: 14px 0 10px; }
.bar-chart { height: 120px; display: flex; align-items: end; gap: 4px; padding: 10px; border: 1px solid var(--border); border-radius: 10px; background: var(--surface); }
.bar-wrap { flex: 1; height: 100%; display: flex; align-items: end; min-width: 4px; }
.bar { width: 100%; border-radius: 4px 4px 0 0; background: linear-gradient(180deg, var(--success), var(--primary)); opacity: 0.85; }
.mono { font-family: 'JetBrains Mono', monospace; }
.text-muted { color: var(--text-muted); }
.empty-state { padding: 12px; text-align: center; color: var(--text-muted); font-size: 13px; }
</style>
