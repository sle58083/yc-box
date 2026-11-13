import { Navigate } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useAuthStore } from '../store/auth'

export default function RequireAuth({ children }: { children: ReactNode }) {
  const token = useAuthStore((s) => s.token)
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}
