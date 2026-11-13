import { create } from 'zustand'

const TOKEN_KEY = 'sbp_token'
const REFRESH_KEY = 'sbp_refresh_token'
const USER_KEY = 'sbp_user'

interface AuthUser {
  id: string
  username: string
  role: string
  role_id: string
}

interface AuthState {
  token: string | null
  refreshToken: string | null
  user: AuthUser | null
  role: string
  setAuth: (token: string, refreshToken: string, user: AuthUser) => void
  setMe: (user: AuthUser) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem(TOKEN_KEY),
  refreshToken: localStorage.getItem(REFRESH_KEY),
  user: readUser(),
  role: readUser()?.role ?? '',
  setAuth: (token, refreshToken, user) => {
    localStorage.setItem(TOKEN_KEY, token)
    localStorage.setItem(REFRESH_KEY, refreshToken)
    localStorage.setItem(USER_KEY, JSON.stringify(user))
    set({ token, refreshToken, user, role: user.role })
  },
  setMe: (user) => {
    localStorage.setItem(USER_KEY, JSON.stringify(user))
    set({ user, role: user.role })
  },
  logout: () => {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_KEY)
    localStorage.removeItem(USER_KEY)
    set({ token: null, refreshToken: null, user: null, role: '' })
  },
}))

function readUser(): AuthUser | null {
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as AuthUser
  } catch {
    return null
  }
}
