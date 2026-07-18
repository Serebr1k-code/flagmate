export interface Flow {
  id: string
  service_id: number | null
  direction: string
  start_ts: string | null
  end_ts: string | null
  raw_request: Record<string, any>
  raw_response: Record<string, any>
  hash: string
  stable: boolean
  stability_pct: number
  avg_interval: number
  destination: string
  checker: boolean
  banned: boolean
  mirrored: boolean
  group_name: string
  group_count: number
  marks: MarkHit[]
  response_code: number
  flow_id: number
  src_ip: string
  dst_ip: string
  src_port: number
  dst_port: number
  proto: string
  pkt_count: number
  bytes_in: number
  bytes_out: number
  created_at: string
}

export interface MarkHit {
  id: number
  name: string
  regex: string
  color: string
  banned?: boolean
  active?: boolean
  order?: number
  flows?: number
  groups?: number
}

export interface Mark extends MarkHit {}

export interface Service {
  id: number
  name: string
  port: number
  protocol: string
  created_at: string
}

export interface Pattern {
  id: number
  service_id: number | null
  pattern: string
  description: string
  mode: string
  active: boolean
  match_count: number
  created_at: string
}

export interface FlowGroup {
  hash: string
  name: string
  checker: boolean
  count: number
  example_flow_id: string
  first_seen: string
  last_seen: string
  destination: string
  method: string
  uri: string
  response_code: number
  service_id: number | null
  mirrored: boolean
  stability_pct: number
  avg_interval: number
  latest_flow?: Flow
}

export interface ServiceMirrorConfig {
  service_id: number
  enabled: boolean
  interval_seconds: number
  targets: { ip: string; port: number }[]
}
