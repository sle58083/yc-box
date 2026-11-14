import { createBrowserRouter, Navigate } from 'react-router-dom'
import type { RouteObject } from 'react-router-dom'
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

// 路由表：登录页 + 受保护的布局及子路由。
// 路由守卫：未登录 → /login；登录后按角色渲染菜单与权限。
const routes: RouteObject[] = [
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
]

// 根据面板路径（basename）创建路由：'/a1b2c3' 挂载到子路径，''=根路径。
export function createAppRouter(basename: string) {
  return createBrowserRouter(routes, { basename })
}
