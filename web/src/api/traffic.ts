import { get } from './client'
import type { NodeStat, UserStat } from './types'

export function nodeTraffic(params: { node_id?: string; from?: string; to?: string }): Promise<NodeStat[]> {
  const q = new URLSearchParams()
  if (params.node_id) q.set('node_id', params.node_id)
  if (params.from) q.set('from', params.from)
  if (params.to) q.set('to', params.to)
  const s = q.toString()
  return get<NodeStat[]>(`/traffic/nodes${s ? `?${s}` : ''}`)
}

export function userTraffic(params: { proxy_user_id?: string; from?: string; to?: string }): Promise<UserStat[]> {
  const q = new URLSearchParams()
  if (params.proxy_user_id) q.set('proxy_user_id', params.proxy_user_id)
  if (params.from) q.set('from', params.from)
  if (params.to) q.set('to', params.to)
  const s = q.toString()
  return get<UserStat[]>(`/traffic/users${s ? `?${s}` : ''}`)
}
