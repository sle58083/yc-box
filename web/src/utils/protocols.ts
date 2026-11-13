// 前端本地协议枚举与字段元数据（MVP）。
// TODO(P1)：后端实现 GET /protocols 后，改为从后端下发 Meta()，避免前后端漂移。

export interface ProtocolField {
  key: string
  label: string
  type: 'text' | 'number' | 'duration' | 'select' | 'array' | 'bool'
  required?: boolean
  placeholder?: string
  options?: { label: string; value: string }[]
}

export interface ProtocolMeta {
  protocol: string
  label: string
  inbound: ProtocolField[]
  outbound: ProtocolField[]
}

const commonOutboundServer: ProtocolField[] = [
  { key: 'server', label: '服务器地址', type: 'text', required: true },
  { key: 'server_port', label: '端口', type: 'number', required: true },
]

export const PROTOCOLS: ProtocolMeta[] = [
  {
    protocol: 'vless',
    label: 'VLESS',
    inbound: [
      { key: 'flow', label: 'Flow', type: 'text', placeholder: 'xtls-rprx-vision' },
    ],
    outbound: [
      ...commonOutboundServer,
      { key: 'uuid', label: 'UUID', type: 'text', required: true },
      { key: 'flow', label: 'Flow', type: 'text' },
    ],
  },
  {
    protocol: 'vmess',
    label: 'VMess',
    inbound: [{ key: 'alter_id', label: 'Alter ID', type: 'number' }],
    outbound: [
      ...commonOutboundServer,
      { key: 'uuid', label: 'UUID', type: 'text', required: true },
      { key: 'alter_id', label: 'Alter ID', type: 'number' },
    ],
  },
  {
    protocol: 'trojan',
    label: 'Trojan',
    inbound: [{ key: 'password', label: '密码', type: 'text', required: true }],
    outbound: [
      ...commonOutboundServer,
      { key: 'password', label: '密码', type: 'text', required: true },
    ],
  },
  {
    protocol: 'shadowsocks',
    label: 'Shadowsocks',
    inbound: [
      {
        key: 'method',
        label: '加密方式',
        type: 'select',
        required: true,
        options: [
          { label: '2022-blake3-aes-128-gcm', value: '2022-blake3-aes-128-gcm' },
          { label: '2022-blake3-aes-256-gcm', value: '2022-blake3-aes-256-gcm' },
          { label: 'aes-128-gcm', value: 'aes-128-gcm' },
          { label: 'aes-256-gcm', value: 'aes-256-gcm' },
          { label: 'chacha20-ietf-poly1305', value: 'chacha20-ietf-poly1305' },
        ],
      },
      { key: 'password', label: '密码', type: 'text', required: true },
    ],
    outbound: [
      ...commonOutboundServer,
      { key: 'method', label: '加密方式', type: 'select', required: true },
      { key: 'password', label: '密码', type: 'text', required: true },
    ],
  },
  {
    protocol: 'hysteria2',
    label: 'Hysteria2',
    inbound: [
      { key: 'password', label: '密码', type: 'text', required: true },
      { key: 'masquerade', label: 'Masquerade', type: 'text' },
    ],
    outbound: [
      ...commonOutboundServer,
      { key: 'password', label: '密码', type: 'text', required: true },
    ],
  },
  {
    protocol: 'tuic',
    label: 'TUIC',
    inbound: [
      { key: 'uuid', label: 'UUID', type: 'text', required: true },
      { key: 'password', label: '密码', type: 'text', required: true },
      {
        key: 'congestion_control',
        label: '拥塞控制',
        type: 'select',
        options: [
          { label: 'cubic', value: 'cubic' },
          { label: 'new_reno', value: 'new_reno' },
          { label: 'bbr', value: 'bbr' },
        ],
      },
    ],
    outbound: [
      ...commonOutboundServer,
      { key: 'uuid', label: 'UUID', type: 'text', required: true },
      { key: 'password', label: '密码', type: 'text', required: true },
      {
        key: 'congestion_control',
        label: '拥塞控制',
        type: 'select',
        options: [
          { label: 'cubic', value: 'cubic' },
          { label: 'new_reno', value: 'new_reno' },
          { label: 'bbr', value: 'bbr' },
        ],
      },
    ],
  },
  {
    protocol: 'anytls',
    label: 'AnyTLS',
    inbound: [
      { key: 'padding_scheme', label: 'Padding 方案（数组）', type: 'array', placeholder: '["tls-tcp"]' },
    ],
    outbound: [
      ...commonOutboundServer,
      { key: 'password', label: '密码（渲染时 base64）', type: 'text', required: true },
      { key: 'idle_session_check_interval', label: '会话检查间隔', type: 'duration', placeholder: '30s' },
      { key: 'idle_session_timeout', label: '会话空闲超时', type: 'duration', placeholder: '30s' },
      { key: 'min_idle_session', label: '最小空闲会话数', type: 'number' },
      { key: 'client_metadata', label: 'Client Metadata', type: 'text' },
    ],
  },
  {
    protocol: 'selector',
    label: 'Selector',
    inbound: [],
    outbound: [
      { key: 'outbounds', label: '出站列表（数组）', type: 'array', required: true, placeholder: '["auto","direct"]' },
      { key: 'default', label: '默认出站', type: 'text' },
    ],
  },
  {
    protocol: 'urltest',
    label: 'URLTest',
    inbound: [],
    outbound: [
      { key: 'outbounds', label: '出站列表（数组）', type: 'array', required: true },
      { key: 'url', label: '探测 URL', type: 'text', required: true, placeholder: 'http://www.gstatic.com/generate_204' },
      { key: 'interval', label: '探测间隔', type: 'duration' },
    ],
  },
  {
    protocol: 'direct',
    label: 'Direct',
    inbound: [],
    outbound: [],
  },
  {
    protocol: 'block',
    label: 'Block',
    inbound: [],
    outbound: [],
  },
]

export function protocolMeta(protocol: string): ProtocolMeta | undefined {
  return PROTOCOLS.find((p) => p.protocol === protocol)
}

export function inboundProtocols(): string[] {
  return PROTOCOLS.filter((p) => p.inbound.length > 0 || ['vless', 'vmess', 'trojan', 'shadowsocks', 'hysteria2', 'tuic', 'anytls'].includes(p.protocol)).map((p) => p.protocol)
}

export function outboundProtocols(): string[] {
  return PROTOCOLS.map((p) => p.protocol)
}
