import { Layout, Menu, Dropdown, Space, Tag } from 'antd'
import type { MenuProps } from 'antd'
import { UserOutlined, LogoutOutlined } from '@ant-design/icons'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'

const { Sider, Header, Content } = Layout

export default function AppLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { user, role, logout } = useAuthStore()

  const items: MenuProps['items'] = [
    { key: '/dashboard', label: '概览' },
    { key: '/nodes', label: '节点管理' },
    { key: '/inbounds', label: 'Inbound 配置' },
    { key: '/outbounds', label: 'Outbound 配置' },
    { key: '/routing', label: '路由规则' },
    { key: '/users', label: '代理用户' },
    { key: '/traffic', label: '流量统计' },
    { key: '/audit', label: '审计日志' },
    ...(role === 'super_admin' ? [{ key: '/settings', label: '系统设置' }] : []),
  ]

  const selected = String(items.find((i) => i?.key && location.pathname.startsWith(String(i.key)))?.key ?? '/dashboard')

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider theme="dark" width={200}>
        <div style={{ height: 48, margin: 16, color: '#fff', fontWeight: 600, fontSize: 16 }}>sing-box 面板</div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selected]}
          items={items}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header style={{ background: '#fff', padding: '0 24px', display: 'flex', justifyContent: 'flex-end', alignItems: 'center' }}>
          <Dropdown
            menu={{
              items: [
                { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', onClick: () => { logout(); navigate('/login') } },
              ],
            }}
          >
            <Space style={{ cursor: 'pointer' }}>
              <UserOutlined />
              {user?.username ?? '未登录'}
              <Tag color="blue">{role}</Tag>
            </Space>
          </Dropdown>
        </Header>
        <Content style={{ margin: 16 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
