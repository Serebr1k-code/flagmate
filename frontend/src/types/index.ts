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
  checker: boolean
  banned: boolean
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

export interface Service {
  id: number
  name: string
  port: number
  protocol: string
  created_at: string
}

export interface Pattern {
  id: number
  pattern: string
  description: string
  created_at: string
}

export interface FlowGroup {
  hash: string
  count: number
  example_flow_id: string
  first_seen: string
  last_seen: string
}

export interface MirroringConfig {
  enabled: boolean
  targets: { ip: string; port: number }[]
}
