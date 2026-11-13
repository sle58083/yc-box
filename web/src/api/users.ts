import { del, get, post, put } from './client'
import type { PanelUser, ProxyUser, Role } from './types'

// 代理用户
export function listProxyUsers(groupId?: string): Promise<ProxyUser[]> {
  const q = groupId ? `?group_id=${groupId}` : ''
  return get<ProxyUser[]>(`/proxy-users${q}`)
}

export function createProxyUser(body: Record<string, unknown>): Promise<{ user: ProxyUser; sub_token: string; subscribe_url: string }> {
  return post('/proxy-users', body)
}

export function updateProxyUser(id: string, body: Record<string, unknown>): Promise<ProxyUser> {
  return put(`/proxy-users/${id}`, body)
}

export function deleteProxyUser(id: string): Promise<{ id: string }> {
  return del(`/proxy-users/${id}`)
}

export function resetSubToken(id: string): Promise<{ sub_token: string; subscribe_url: string }> {
  return post(`/proxy-users/${id}/reset-sub`, {})
}

// 面板用户（super_admin）
export function listPanelUsers(): Promise<PanelUser[]> {
  return get<PanelUser[]>('/panel-users')
}

export function createPanelUser(body: Record<string, unknown>): Promise<PanelUser> {
  return post('/panel-users', body)
}

export function updatePanelUser(id: string, body: Record<string, unknown>): Promise<PanelUser> {
  return put(`/panel-users/${id}`, body)
}

export function deletePanelUser(id: string): Promise<{ id: string }> {
  return del(`/panel-users/${id}`)
}

// 角色
export function listRoles(): Promise<Role[]> {
  return get<Role[]>('/roles')
}
