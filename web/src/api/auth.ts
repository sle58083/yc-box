import { get, post } from './client'
import type { LoginResponse, MeResponse } from './types'

export function login(username: string, password: string): Promise<LoginResponse> {
  return post<LoginResponse>('/auth/login', { username, password })
}

export function refresh(refreshToken: string): Promise<{ access_token: string; refresh_token: string; expires_in: number }> {
  return post('/auth/refresh', { refresh_token: refreshToken })
}

export function me(): Promise<MeResponse> {
  return get<MeResponse>('/auth/me')
}
