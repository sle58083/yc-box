import { del, get, post, put } from './client'
import type { InboundConfig } from './types'

export function listInbounds(groupId?: string): Promise<InboundConfig[]> {
  const q = groupId ? `?group_id=${groupId}` : ''
  return get<InboundConfig[]>(`/inbounds${q}`)
}

export function createInbound(body: Partial<InboundConfig>): Promise<InboundConfig> {
  return post('/inbounds', body)
}

export function updateInbound(id: string, body: Partial<InboundConfig>): Promise<InboundConfig> {
  return put(`/inbounds/${id}`, body)
}

export function deleteInbound(id: string): Promise<{ id: string }> {
  return del(`/inbounds/${id}`)
}

export function previewConfig(nodeId: string): Promise<Record<string, unknown>> {
  return post('/config/preview', { node_id: nodeId })
}
