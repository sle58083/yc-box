import { get, post } from './client'

export interface Group {
  id: string
  name: string
  desc: string
  parent_id: string | null
  created_at: string
  updated_at: string
}

export function listGroups(): Promise<Group[]> {
  return get<Group[]>('/groups')
}

export function createGroup(body: { name: string; desc?: string }): Promise<Group> {
  return post('/groups', body)
}
