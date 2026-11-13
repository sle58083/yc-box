import { get, post } from './client'
import type { ConfigVersion } from './types'

export function previewConfig(nodeId: string): Promise<Record<string, unknown>> {
  return post('/config/preview', { node_id: nodeId })
}

export function deployConfig(nodeId: string): Promise<ConfigVersion> {
  return post('/config/deploy', { node_id: nodeId })
}

export function retryConfig(nodeId: string, versionId: string): Promise<ConfigVersion> {
  return post('/config/retry', { node_id: nodeId, version_id: versionId })
}

export function rollbackConfig(nodeId: string, versionId: string): Promise<ConfigVersion> {
  return post('/config/rollback', { node_id: nodeId, version_id: versionId })
}

export function listVersions(nodeId: string): Promise<ConfigVersion[]> {
  return get<ConfigVersion[]>(`/config/versions?node_id=${nodeId}`)
}
