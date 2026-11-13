import { Button, Card, Form, Input, message } from 'antd'
import { UserOutlined, LockOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../../api/auth'
import { useAuthStore } from '../../store/auth'

export default function Login() {
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)
  const setMe = useAuthStore((s) => s.setMe)
  const [loading, setLoading] = useState(false)

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const res = await api.login(values.username, values.password)
      setAuth(res.access_token, res.refresh_token, {
        id: res.user.id,
        username: res.user.username,
        role: '',
        role_id: res.user.role_id,
      })
      try {
        const meInfo = await api.me()
        setMe({ id: meInfo.id, username: meInfo.username, role: meInfo.role, role_id: meInfo.role_id })
      } catch {
        // me 失败不阻塞登录，仅缺角色信息
      }
      message.success('登录成功')
      navigate('/dashboard', { replace: true })
    } catch (e) {
      message.error(e instanceof Error ? e.message : '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f0f2f5' }}>
      <Card style={{ width: 380 }}>
        <h2 style={{ textAlign: 'center' }}>sing-box 企业级代理面板</h2>
        <Form onFinish={onFinish} size="large">
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" block loading={loading}>
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
