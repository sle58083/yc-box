import { del, get, post, put } from './client'
import type { RoutingRule } from './types'

export function listRouting(groupId?: string): Promise<RoutingRule[]> {
  const q = groupId ? `?group_id=${groupId}` : ''
  return get<RoutingRule[]>(`/routing${q}`)
}

export function createRouting(body: Partial<RoutingRule>): Promise<RoutingRule> {
  return post('/routing', body)
}

export function updateRouting(id: string, body: Partial<RoutingRule>): Promise<RoutingRule> {
  return put(`/routing/${id}`, body)
}

export function deleteRouting(id: string): Promise<{ id: string }> {
  return del(`/routing/${id}`)
}
