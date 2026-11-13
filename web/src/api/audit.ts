import { get } from './client'
import type { AuditLog } from './types'

export function listAuditLogs(params: {
  action?: string
  operator_id?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}): Promise<AuditLog[]> {
  const q = new URLSearchParams()
  if (params.action) q.set('action', params.action)
  if (params.operator_id) q.set('operator_id', params.operator_id)
  if (params.from) q.set('from', params.from)
  if (params.to) q.set('to', params.to)
  if (params.limit) q.set('limit', String(params.limit))
  if (params.offset) q.set('offset', String(params.offset))
  const s = q.toString()
  return get<AuditLog[]>(`/audit-logs${s ? `?${s}` : ''}`)
}
