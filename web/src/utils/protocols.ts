import type { ProtocolFieldMeta, ProtocolMeta } from '../api/protocols'

// 本地协议元数据（fallback）：当 GET /api/v1/protocols 不可用时使用。
// 结构必须与后端 renderer.ProtocolMeta 对齐；运行时优先使用后端下发数据。

const outboundServer: ProtocolFieldMeta[] = [
  { key: 'server', label: '服务器地址', type: 'text', required: true, encrypt: false },
  { key: 'server_port', label: '端口', type: 'number', required: true, encrypt: false },
]

const shadowsocksMethods = [
  { label: '2022-blake3-aes-128-gcm', value: '2022-blake3-aes-128-gcm' },
  { label: '2022-blake3-aes-256-gcm', value: '2022-blake3-aes-256-gcm' },
  { label: 'aes-128-gcm', value: 'aes-128-gcm' },
  { label: 'aes-256-gcm', value: 'aes-256-gcm' },
  { label: 'chacha20-ietf-poly1305', value: 'chacha20-ietf-poly1305' },
]

const congestionControl = [
  { label: 'cubic', value: 'cubic' },
  { label: 'new_reno', value: 'new_reno' },
  { label: 'bbr', value: 'bbr' },
]

export const LOCAL_PROTOCOLS: ProtocolMeta[] = [
  {
    protocol: 'vless',
    directions: ['inbound', 'outbound'],
    inbound: [{ key: 'flow', label: 'Flow', type: 'text', required: false, encrypt: false }],
    outbound: [
      ...outboundServer,
      { key: 'uuid', label: 'UUID', type: 'text', required: true, encrypt: true },
      { key: 'flow', label: 'Flow', type: 'text', required: false, encrypt: false },
    ],
    tls_allowed: ['none', 'plain', 'reality', 'ech'],
    tls_required: false,
    multi_user: true,
  },
  {
    protocol: 'vmess',
    directions: ['inbound', 'outbound'],
    inbound: [{ key: 'alter_id', label: 'Alter ID', type: 'number', required: false, encrypt: false, default: 0 }],
    outbound: [
      ...outboundServer,
      { key: 'uuid', label: 'UUID', type: 'text', required: true, encrypt: true },
      { key: 'alter_id', label: 'Alter ID', type: 'number', required: false, encrypt: false, default: 0 },
    ],
    tls_allowed: ['none'],
    tls_required: false,
    multi_user: true,
  },
  {
    protocol: 'trojan',
    directions: ['inbound', 'outbound'],
    inbound: [{ key: 'password', label: '密码', type: 'text', required: true, encrypt: true }],
    outbound: [
      ...outboundServer,
      { key: 'password', label: '密码', type: 'text', required: true, encrypt: true },
    ],
    tls_allowed: ['plain'],
    tls_required: true,
    multi_user: true,
  },
  {
    protocol: 'shadowsocks',
    directions: ['inbound', 'outbound'],
    inbound: [
      { key: 'method', label: '加密方式', type: 'select', required: true, encrypt: false, default: 'aes-128-gcm', options: shadowsocksMethods },
      { key: 'password', label: '密码', type: 'text', required: true, encrypt: true },
    ],
    outbound: [
      ...outboundServer,
      { key: 'method', label: '加密方式', type: 'select', required: true, encrypt: false, default: 'aes-128-gcm', options: shadowsocksMethods },
      { key: 'password', label: '密码', type: 'text', required: true, encrypt: true },
    ],
    tls_allowed: ['none'],
    tls_required: false,
    multi_user: false,
  },
  {
    protocol: 'hysteria2',
    directions: ['inbound', 'outbound'],
    inbound: [
      { key: 'password', label: '密码', type: 'text', required: true, encrypt: true },
      { key: 'masquerade', label: 'Masquerade', type: 'text', required: false, encrypt: false },
    ],
    outbound: [
      ...outboundServer,
      { key: 'password', label: '密码', type: 'text', required: true, encrypt: true },
    ],
    tls_allowed: ['plain'],
    tls_required: true,
    multi_user: true,
  },
  {
    protocol: 'tuic',
    directions: ['inbound', 'outbound'],
    inbound: [
      { key: 'uuid', label: 'UUID', type: 'text', required: true, encrypt: true },
      { key: 'password', label: '密码', type: 'text', required: true, encrypt: true },
      { key: 'congestion_control', label: '拥塞控制', type: 'select', required: false, encrypt: false, default: 'cubic', options: congestionControl },
    ],
    outbound: [
      ...outboundServer,
      { key: 'uuid', label: 'UUID', type: 'text', required: true, encrypt: true },
      { key: 'password', label: '密码', type: 'text', required: true, encrypt: true },
      { key: 'congestion_control', label: '拥塞控制', type: 'select', required: false, encrypt: false, default: 'cubic', options: congestionControl },
    ],
    tls_allowed: ['plain'],
    tls_required: true,
    multi_user: true,
  },
  {
    protocol: 'anytls',
    directions: ['inbound', 'outbound'],
    inbound: [{ key: 'padding_scheme', label: 'Padding 方案', type: 'array', required: false, encrypt: false }],
    outbound: [
      ...outboundServer,
      { key: 'password', label: '密码（渲染时 base64）', type: 'text', required: true, encrypt: true },
      { key: 'idle_session_check_interval', label: '会话检查间隔', type: 'duration', required: false, encrypt: false },
      { key: 'idle_session_timeout', label: '会话空闲超时', type: 'duration', required: false, encrypt: false },
      { key: 'min_idle_session', label: '最小空闲会话数', type: 'number', required: false, encrypt: false },
      { key: 'client_metadata', label: 'Client Metadata', type: 'text', required: false, encrypt: false },
    ],
    tls_allowed: ['plain'],
    tls_required: true,
    multi_user: true,
  },
  {
    protocol: 'selector',
    directions: ['outbound'],
    inbound: [],
    outbound: [
      { key: 'outbounds', label: '出站列表（数组）', type: 'array', required: true, encrypt: false },
      { key: 'default', label: '默认出站', type: 'text', required: false, encrypt: false },
    ],
    tls_allowed: [],
    tls_required: false,
    multi_user: false,
  },
  {
    protocol: 'urltest',
    directions: ['outbound'],
    inbound: [],
    outbound: [
      { key: 'outbounds', label: '出站列表（数组）', type: 'array', required: true, encrypt: false },
      { key: 'url', label: '探测 URL', type: 'text', required: true, encrypt: false },
      { key: 'interval', label: '探测间隔', type: 'duration', required: false, encrypt: false },
    ],
    tls_allowed: [],
    tls_required: false,
    multi_user: false,
  },
  {
    protocol: 'direct',
    directions: ['outbound'],
    inbound: [],
    outbound: [],
    tls_allowed: [],
    tls_required: false,
    multi_user: false,
  },
  {
    protocol: 'block',
    directions: ['outbound'],
    inbound: [],
    outbound: [],
    tls_allowed: [],
    tls_required: false,
    multi_user: false,
  },
]

export function protocolMetaFrom(metas: ProtocolMeta[], protocol: string): ProtocolMeta | undefined {
  return metas.find((p) => p.protocol === protocol)
}

export function inboundProtocolsFrom(metas: ProtocolMeta[]): string[] {
  return metas.filter((p) => p.directions.includes('inbound')).map((p) => p.protocol)
}

export function outboundProtocolsFrom(metas: ProtocolMeta[]): string[] {
  return metas.filter((p) => p.directions.includes('outbound')).map((p) => p.protocol)
}

// applyFieldDefaults 为缺失字段补默认值（如 shadowsocks 的 method、tuic 的 congestion_control）。
export function applyFieldDefaults(fields: ProtocolFieldMeta[], value: Record<string, unknown>): Record<string, unknown> {
  const out = { ...value }
  for (const f of fields) {
    if (out[f.key] === undefined && f.default !== undefined) {
      out[f.key] = f.default
    }
  }
  return out
}
