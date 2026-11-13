// 后端实体与响应类型（与 control-plane model 对齐）。

export interface Node {
  id: string
  group_id: string
  name: string
  address: string
  agent_version: string
  singbox_version: string
  status: 'online' | 'offline' | 'pending' | 'error'
  last_heartbeat: string | null
  tags: unknown
  created_at: string
  updated_at: string
}

export interface InboundConfig {
  id: string
  group_id: string
  node_id: string | null
  name: string
  type: string
  listen: string
  port: number
  settings: Record<string, unknown> | null
  tls: Record<string, unknown> | null
  transport: Record<string, unknown> | null
  is_user_inbound: boolean
  status: 'enabled' | 'disabled'
  created_at: string
  updated_at: string
}

export interface OutboundConfig {
  id: string
  group_id: string
  node_id: string | null
  name: string
  type: string
  settings: Record<string, unknown> | null
  tls: Record<string, unknown> | null
  transport: Record<string, unknown> | null
  status: 'enabled' | 'disabled'
  created_at: string
  updated_at: string
}

export interface RoutingRule {
  id: string
  group_id: string
  node_id: string | null
  name: string
  inbound_tags: string[]
  outbound_tag: string
  rule_type: 'geosite' | 'geoip' | 'domain' | 'ip' | 'protocol'
  value: string
  priority: number
  created_at: string
  updated_at: string
}

export interface ProxyUser {
  id: string
  group_id: string
  remark: string
  quota_bytes: number
  used_bytes: number
  expire_at: string | null
  status: 'enabled' | 'disabled' | 'expired'
  created_at: string
  updated_at: string
}

export interface PanelUser {
  id: string
  username: string
  role_id: string
  status: string
  created_at: string
  updated_at: string
}

export interface Role {
  id: string
  name: string
  permissions: string[]
  created_at: string
  updated_at: string
}

export interface ConfigVersion {
  id: string
  node_id: string
  version: number
  hash: string
  status: string
  created_by: string
  created_at: string
}

export interface AuditLog {
  id: string
  operator_id: string
  action: string
  resource: string
  detail: unknown
  ip: string
  created_at: string
}

export interface NodeStat {
  node_id: string
  up_bytes: number
  down_bytes: number
}

export interface UserStat {
  proxy_user_id: string | null
  up_bytes: number
  down_bytes: number
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
  user: { id: string; username: string; role_id: string }
}

export interface MeResponse {
  id: string
  username: string
  role: string
  role_id: string
  status: string
}
