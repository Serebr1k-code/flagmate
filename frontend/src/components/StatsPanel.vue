<template>
  <div class="stats-page">
    <div class="page-header">
      <div>
        <h1>Stats</h1>
        <p class="text-muted">Attack sessions, stolen flags, and mirroring results.</p>
      </div>
      <button class="btn btn-outline" @click="fetchAll">Refresh</button>
    </div>

    <div class="card settings-card">
      <div>
        <h3>Board settings</h3>
        <p class="text-muted">Saved for flag theft correlation. Telegram bot integration is intentionally omitted.</p>
      </div>
      <input v-model="settings.team_name" class="input" placeholder="Our team name" />
      <input v-model="settings.board_url" class="input" placeholder="Board URL" />
      <button class="btn btn-primary" @click="saveSettings">Save</button>
    </div>

    <div class="stats-grid">
      <div class="stat-tile"><span>Stolen flags</span><b>{{ thefts.total_flags }}</b></div>
      <div class="stat-tile"><span>Attack sessions</span><b>{{ sessions.length }}</b></div>
      <div class="stat-tile"><span>Mirror requests</span><b>{{ mirror.total_requests }}</b></div>
      <div class="stat-tile"><span>Mirror flags</span><b>{{ mirror.flags }}</b></div>
    </div>

    <div class="card chain-card">
      <div class="card-header">
        <div>
          <h3>Attack chain graph</h3>
          <p class="text-muted">Causal view: attacker -> service endpoint -> result{{ selectedServiceId ? ' · filtered by clicked service' : '' }}.</p>
        </div>
      </div>
      <div class="chain-graph">
        <div v-for="session in graphSessions" :key="`${session.attacker_ip}-${session.service_id}-${session.started_at}`" class="chain-row" @click="emit('openFlowId', session.flow_id)">
          <div class="graph-node attacker">
            <span>attacker</span>
            <b>{{ session.attacker_ip }}</b>
          </div>
          <div class="graph-edge"><span></span><em>{{ session.requests }} req</em></div>
          <div class="graph-node service">
            <span>service endpoint</span>
            <b>{{ session.service || `service ${session.service_id}` }}</b>
            <small>{{ session.endpoint || 'mixed endpoints' }}</small>
          </div>
          <div class="graph-edge"><span></span><em>{{ durationLabel(session.duration_seconds) }}</em></div>
          <div class="graph-node result" :class="session.flags > 0 ? 'compromised' : 'probing'">
            <span>{{ session.flags > 0 ? 'compromised' : 'probing' }}</span>
            <b>{{ session.flags }} flags</b>
          </div>
        </div>
        <div v-if="graphSessions.length === 0" class="empty-state">No attack chains yet</div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">
        <div>
          <h3>Stolen flags over time</h3>
          <p class="text-muted">Flags detected in responses from our services to incoming clients.</p>
        </div>
        <select v-model.number="minutes" class="select" @change="fetchAll">
          <option :value="60">Last hour</option>
          <option :value="120">Last 2h</option>
          <option :value="360">Last 6h</option>
          <option :value="1440">Last day</option>
        </select>
      </div>
      <svg class="line-chart" viewBox="0 0 400 100" preserveAspectRatio="none">
        <polyline v-if="thefts.series.length > 1" :points="linePoints" fill="none" stroke="#fb4934" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
        <circle v-for="(point, i) in thefts.series" :key="point.ts" :cx="pointX(i)" :cy="pointY(point.flags)" r="4" fill="#fb4934" stroke="#1a0a0a" stroke-width="2"/>
        <text v-if="thefts.series.length > 0" :x="pointX(0)" y="96" fill="#a89984" font-size="7" text-anchor="start">{{ formatTimeNoDate(thefts.series[0].ts) }}</text>
        <text v-if="thefts.series.length > 1" :x="pointX(thefts.series.length-1)" y="96" fill="#a89984" font-size="7" text-anchor="end">{{ formatTimeNoDate(thefts.series[thefts.series.length-1].ts) }}</text>
        <text v-if="thefts.series.length === 0" x="200" y="55" fill="#a89984" font-size="11" text-anchor="middle">No stolen flags detected yet</text>
      </svg>
      <div class="theft-list">
        <div v-for="item in thefts.items.slice(0, 50)" :key="`${item.flow_id}-${item.flag}`" class="stat-row clickable" @click="emit('openFlowId', item.flow_id)">
          <span class="mono">{{ item.attacker_ip }}</span>
          <span>{{ item.service || `service ${item.service_id}` }}</span>
          <b class="flag-chip">{{ item.flag }}</b>
          <span class="text-muted">{{ formatTime(item.created_at) }}</span>
        </div>
        <div v-if="thefts.items.length === 0" class="empty-state">No flag leaks in the selected window</div>
      </div>
    </div>

    <div class="card">
      <h3>Attack sessions</h3>
      <p class="text-muted">Grouped by attacker IP + service + two-minute activity window.</p>
      <div class="session-list">
        <div v-for="session in sessions" :key="`${session.attacker_ip}-${session.service_id}-${session.started_at}`" class="session-row clickable" @click="emit('openFlowId', session.flow_id)">
          <div>
            <b>{{ session.attacker_ip }}</b>
            <span class="text-muted"> attacked {{ session.service || `service ${session.service_id}` }}</span>
          </div>
          <span>{{ durationLabel(session.duration_seconds) }}</span>
          <span>{{ session.requests }} requests</span>
          <span class="flag-chip">{{ session.flags }} flags</span>
        </div>
        <div v-if="sessions.length === 0" class="empty-state">No grouped attack sessions yet</div>
      </div>
    </div>

    <div class="card">
      <h3>Mirroring stats</h3>
      <p class="text-muted">Same summary as Mirroring; kept here for one stats page.</p>
      <div class="stats-grid compact">
        <div class="stat-tile"><span>Requests</span><b>{{ mirror.total_requests }}</b></div>
        <div class="stat-tile"><span>Flags</span><b>{{ mirror.flags }}</b></div>
        <div class="stat-tile"><span>Success</span><b>{{ mirror.success_rate }}%</b></div>
      </div>
      <div class="stats-columns">
        <div>
          <h4>Teams</h4>
          <div v-for="team in mirror.teams" :key="team.target_ip" class="stat-row">
            <span class="mono">{{ team.target_ip }}</span>
            <b>{{ team.flags }} flags</b>
            <span>{{ team.success_rate }}%</span>
          </div>
        </div>
        <div>
          <h4>Mirrored flow types</h4>
          <div v-for="group in mirror.groups" :key="group.hash || group.name" class="stat-row">
            <span>{{ group.name || group.hash?.slice(0, 8) || 'group' }}</span>
            <b>{{ group.flags }} flags</b>
            <span>{{ group.success_rate }}%</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import api from '@/utils/api'

const props = defineProps<{ selectedServiceId?: number | null }>()
const emit = defineEmits<{ openFlowId: [flowId: string] }>()

const minutes = ref(120)
const settings = ref({ team_name: '', board_url: '' })
const sessions = ref<AttackSession[]>([])
const thefts = ref<FlagThefts>({ total_flags: 0, items: [], series: [] })
const mirror = ref<MirrorStats>({ total_requests: 0, successes: 0, success_rate: 0, flags: 0, teams: [], groups: [], series: {} })

interface AttackSession { attacker_ip: string; service_id: number; service: string; endpoint?: string; started_at: string; ended_at: string; duration_seconds: number; requests: number; flags: number; flow_id: string }
interface FlagTheft { flow_id: string; service_id: number; service: string; attacker_ip: string; flag: string; created_at: string }
interface FlagThefts { total_flags: number; items: FlagTheft[]; series: Array<{ ts: string; flags: number }> }
interface StatItem { target_ip?: string; hash?: string; name?: string; requests: number; successes: number; flags: number; success_rate: number }
interface MirrorStats { total_requests: number; successes: number; success_rate: number; flags: number; teams: StatItem[]; groups: StatItem[]; series: Record<string, unknown> }

const maxFlags = computed(() => Math.max(1, ...thefts.value.series.map(p => p.flags)))
const graphSessions = computed(() => sessions.value.slice().filter(session => !props.selectedServiceId || session.service_id === props.selectedServiceId).sort((a, b) => b.flags - a.flags || b.requests - a.requests).slice(0, 8))
const linePoints = computed(() => thefts.value.series.map((p, i) => `${pointX(i)},${100 - pointY(p.flags)}`).join(' '))
function pointX(i: number) { return thefts.value.series.length <= 1 ? 200 : 20 + (i / (thefts.value.series.length - 1)) * 360 }
function pointY(flags: number) { return Math.max(2, Math.round((flags / maxFlags.value) * 80)) }
function formatTimeNoDate(ts: string) {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

async function fetchAll() {
  try {
    const [{ data: settingsData }, { data: sessionData }, { data: theftData }, { data: mirrorData }] = await Promise.all([
      api.get('/stats/settings'),
      api.get('/stats/attack-sessions', { params: { minutes: minutes.value, window: 120 } }),
      api.get('/stats/flag-thefts', { params: { minutes: minutes.value } }),
      api.get('/mirroring/stats'),
    ])
    settings.value = { team_name: settingsData.team_name || '', board_url: settingsData.board_url || '' }
    sessions.value = sessionData || []
    thefts.value = { total_flags: 0, items: [], series: [], ...theftData }
    mirror.value = { total_requests: 0, successes: 0, success_rate: 0, flags: 0, teams: [], groups: [], series: {}, ...mirrorData }
  } catch (e) { console.error('Failed to fetch stats:', e) }
}

async function saveSettings() {
  try { await api.post('/stats/settings', settings.value) } catch (e) { console.error('Failed to save stats settings:', e) }
}

function durationLabel(seconds: number) { return seconds < 60 ? `${seconds}s` : `${Math.round(seconds / 60)}m` }
function formatTime(value: string) { return value ? new Date(value).toLocaleString() : '-' }

onMounted(fetchAll)
</script>

<style scoped>
.stats-page { padding: 24px; display: flex; flex-direction: column; gap: 18px; overflow-y: auto; height: 100%; }
.page-header, .card-header { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.page-header h1, .card h3 { margin: 0; }
.card { background-color: var(--card); color: var(--card-foreground); border: 1px solid var(--border); border-radius: 12px; padding: 16px; }
.settings-card { display: grid; grid-template-columns: 1.4fr 1fr 1.4fr auto; gap: 12px; align-items: end; }
.stats-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.stats-grid.compact { grid-template-columns: repeat(3, minmax(0, 1fr)); margin-bottom: 16px; }
.stat-tile { background: var(--surface); border: 1px solid var(--border); border-radius: 12px; padding: 14px; display: flex; flex-direction: column; gap: 6px; }
.stat-tile span, .text-muted { color: var(--text-muted); }
.stat-tile b { font-size: 24px; }
.line-chart { width: 100%; height: 110px; border: 1px solid var(--border); border-radius: 12px; background: var(--surface); margin-bottom: 12px; }
.theft-list, .session-list { display: flex; flex-direction: column; gap: 8px; margin-top: 12px; }
.stat-row, .session-row { display: grid; grid-template-columns: 1fr 1fr auto auto; gap: 12px; align-items: center; padding: 10px; border: 1px solid var(--border); border-radius: 10px; background: var(--surface); }
.session-row { grid-template-columns: 1.4fr auto auto auto; }
.clickable { cursor: pointer; }
.clickable:hover { border-color: var(--primary); }
.flag-chip { border: 1px solid var(--destructive); color: var(--destructive); border-radius: 999px; padding: 2px 8px; font-size: 12px; }
.stats-columns { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.chain-card { overflow: hidden; }
.chain-graph { display: flex; flex-direction: column; gap: 12px; padding-top: 8px; }
.chain-row { display: grid; grid-template-columns: 1fr 120px 1fr 120px 1fr; gap: 10px; align-items: center; padding: 12px; border: 1px solid var(--border); border-radius: 16px; background: radial-gradient(circle at 10% 50%, rgba(59,130,246,.14), transparent 28%), radial-gradient(circle at 90% 50%, rgba(239,68,68,.16), transparent 30%), var(--surface); cursor: pointer; }
.chain-row:hover { border-color: var(--primary); transform: translateY(-1px); }
.graph-node { min-height: 76px; border: 1px solid var(--border); border-radius: 16px; padding: 12px; display: flex; flex-direction: column; justify-content: center; gap: 6px; box-shadow: inset 0 0 24px rgba(255,255,255,.03); }
.graph-node span { color: var(--text-muted); font-size: 11px; text-transform: uppercase; letter-spacing: .08em; }
.graph-node b { overflow-wrap: anywhere; }
.graph-node small { color: var(--text-muted); overflow-wrap: anywhere; }
.graph-node.attacker { background: rgba(239, 68, 68, .16); border-color: rgba(239, 68, 68, .45); }
.graph-node.service { background: rgba(59, 130, 246, .14); border-color: rgba(59, 130, 246, .45); }
.graph-node.result.compromised { background: rgba(239, 68, 68, .20); border-color: var(--destructive); }
.graph-node.result.probing { background: rgba(245, 158, 11, .14); border-color: rgba(245, 158, 11, .45); }
.graph-edge { display: flex; flex-direction: column; align-items: center; gap: 6px; color: var(--text-muted); font-size: 12px; }
.graph-edge span { width: 100%; height: 3px; border-radius: 999px; background: linear-gradient(90deg, transparent, var(--primary), transparent); position: relative; }
.graph-edge span::after { content: ''; position: absolute; right: 0; top: 50%; width: 9px; height: 9px; border-top: 3px solid var(--primary); border-right: 3px solid var(--primary); transform: translateY(-50%) rotate(45deg); }
.mono { font-family: 'JetBrains Mono', monospace; }
.empty-state { padding: 18px; text-align: center; color: var(--text-muted); }
@media (max-width: 900px) { .settings-card, .stats-grid, .stats-grid.compact, .stats-columns, .chain-row { grid-template-columns: 1fr; } .graph-edge { display: none; } .stat-row, .session-row { grid-template-columns: 1fr; } }
</style>
