import { createBrowserRouter, Navigate } from 'react-router-dom'
import AppLayout from '../components/AppLayout'
import RequireAuth from '../components/RequireAuth'
import Login from '../pages/login'
import Dashboard from '../pages/dashboard'
import Nodes from '../pages/nodes'
import Inbounds from '../pages/inbounds'
import Outbounds from '../pages/outbounds'
import Routing from '../pages/routing'
import Users from '../pages/users'
import Traffic from '../pages/traffic'
import Audit from '../pages/audit'
import Settings from '../pages/settings'

// 路由守卫：未登录 → /login；登录后按角色渲染菜单与权限。
export const router = createBrowserRouter([
  { path: '/login', element: <Login /> },
  {
    path: '/',
    element: (
      <RequireAuth>
        <AppLayout />
      </RequireAuth>
    ),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      { path: 'dashboard', element: <Dashboard /> },
      { path: 'nodes', element: <Nodes /> },
      { path: 'inbounds', element: <Inbounds /> },
      { path: 'outbounds', element: <Outbounds /> },
      { path: 'routing', element: <Routing /> },
      { path: 'users', element: <Users /> },
      { path: 'traffic', element: <Traffic /> },
      { path: 'audit', element: <Audit /> },
      { path: 'settings', element: <Settings /> },
      { path: '*', element: <Navigate to="/dashboard" replace /> },
    ],
  },
])
