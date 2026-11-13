import { get } from './client'

// 后端 GET /api/v1/protocols 返回的协议元数据类型（与 renderer.ProtocolMeta 对齐）。

export type ProtocolFieldType = 'text' | 'number' | 'bool' | 'select' | 'array' | 'duration'

export interface ProtocolFieldOption {
  label: string
  value: string
}

export interface ProtocolFieldMeta {
  key: string
  label: string
  type: ProtocolFieldType
  required: boolean
  encrypt: boolean
  help?: string
  options?: ProtocolFieldOption[]
  default?: unknown
}

export type TLSMode = 'none' | 'plain' | 'reality' | 'ech'

export interface ProtocolMeta {
  protocol: string
  directions: string[]
  inbound: ProtocolFieldMeta[]
  outbound: ProtocolFieldMeta[]
  tls_allowed: TLSMode[] | null
  tls_required: boolean
  multi_user: boolean
}

export function listProtocols(): Promise<ProtocolMeta[]> {
  return get<ProtocolMeta[]>('/protocols')
}
