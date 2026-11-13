import { del, get, post, put } from './client'
import type { Node } from './types'

export function listNodes(groupId?: string): Promise<Node[]> {
  const q = groupId ? `?group_id=${groupId}` : ''
  return get<Node[]>(`/nodes${q}`)
}

export function createNode(body: Partial<Node>): Promise<{ node: Node; agent_token: string }> {
  return post('/nodes', body)
}

export function updateNode(id: string, body: Partial<Node>): Promise<Node> {
  return put(`/nodes/${id}`, body)
}

export function deleteNode(id: string): Promise<{ id: string }> {
  return del(`/nodes/${id}`)
}

export function regenerateToken(id: string): Promise<{ agent_token: string }> {
  return post(`/nodes/${id}/regenerate-token`, {})
}
