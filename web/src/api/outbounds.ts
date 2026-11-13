import { del, get, post, put } from './client'
import type { OutboundConfig } from './types'

export function listOutbounds(groupId?: string): Promise<OutboundConfig[]> {
  const q = groupId ? `?group_id=${groupId}` : ''
  return get<OutboundConfig[]>(`/outbounds${q}`)
}

export function createOutbound(body: Partial<OutboundConfig>): Promise<OutboundConfig> {
  return post('/outbounds', body)
}

export function updateOutbound(id: string, body: Partial<OutboundConfig>): Promise<OutboundConfig> {
  return put(`/outbounds/${id}`, body)
}

export function deleteOutbound(id: string): Promise<{ id: string }> {
  return del(`/outbounds/${id}`)
}
