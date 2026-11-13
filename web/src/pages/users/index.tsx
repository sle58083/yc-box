import { Button, Form, Input, InputNumber, message, Modal, Popconfirm, Select, Space, Table, Tabs, Tag, Typography } from 'antd'
import { useEffect, useState } from 'react'
import * as api from '../../api/users'
import * as groupsApi from '../../api/groups'
import type { PanelUser, ProxyUser, Role } from '../../api/types'
import type { Group } from '../../api/groups'
import { useAuthStore } from '../../store/auth'
import { fmtBytes, fmtTime } from '../../utils/format'

export default function Users() {
  const role = useAuthStore((s) => s.role)
  return (
    <Tabs
      items={[
        { key: 'proxy', label: '代理用户', children: <ProxyUsers /> },
        ...(role === 'super_admin' ? [{ key: 'panel', label: '面板用户', children: <PanelUsers /> }] : []),
      ]}
    />
  )
}

function ProxyUsers() {
  const role = useAuthStore((s) => s.role)
  const canWrite = role === 'super_admin' || role === 'admin'
  const [rows, setRows] = useState<ProxyUser[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ProxyUser | null>(null)
  const [form] = Form.useForm()
  const [subModal, setSubModal] = useState<{ open: boolean; url: string }>({ open: false, url: '' })

  const load = () => {
    api.listProxyUsers().then(setRows).catch(() => setRows([]))
    groupsApi.listGroups().then(setGroups).catch(() => setGroups([]))
  }
  useEffect(load, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ quota_bytes: -1 })
    if (groups[0]) form.setFieldValue('group_id', groups[0].id)
    setOpen(true)
  }
  const openEdit = (r: ProxyUser) => {
    setEditing(r)
    form.setFieldsValue({ remark: r.remark, quota_bytes: r.quota_bytes, expire_at: r.expire_at ?? '', status: r.status })
    setOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    try {
      if (editing) {
        await api.updateProxyUser(editing.id, values)
      } else {
        const res = await api.createProxyUser(values)
        setSubModal({ open: true, url: res.subscribe_url })
      }
      message.success('保存成功')
      setOpen(false)
      load()
    } catch (e) {
      message.error(e instanceof Error ? e.message : '保存失败')
    }
  }

  const resetSub = async (id: string) => {
    try {
      const res = await api.resetSubToken(id)
      setSubModal({ open: true, url: res.subscribe_url })
    } catch (e) {
      message.error(e instanceof Error ? e.message : '重置失败')
    }
  }

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        {canWrite && <Button type="primary" onClick={openCreate}>新建代理用户</Button>}
      </Space>
      <Table
        rowKey="id"
        dataSource={rows}
        columns={[
          { title: '备注', dataIndex: 'remark' },
          { title: '配额', dataIndex: 'quota_bytes', render: (v) => (v < 0 ? '无限' : fmtBytes(v)) },
          { title: '已用', dataIndex: 'used_bytes', render: (v) => fmtBytes(v) },
          { title: '到期', dataIndex: 'expire_at', render: (v) => fmtTime(v) },
          { title: '状态', dataIndex: 'status', render: (v) => <Tag color={v === 'enabled' ? 'green' : 'default'}>{v}</Tag> },
          {
            title: '操作',
            render: (_, r) => (
              <Space>
                {canWrite && <Button size="small" onClick={() => openEdit(r)}>编辑</Button>}
                {canWrite && <Button size="small" onClick={() => resetSub(r.id)}>订阅链接</Button>}
                {canWrite && (
                  <Popconfirm title="确认删除？" onConfirm={async () => { try { await api.deleteProxyUser(r.id); message.success('已删除'); load() } catch (e) { message.error(e instanceof Error ? e.message : '删除失败') } }}>
                    <Button size="small" danger>删除</Button>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]}
      />

      <Modal title={editing ? '编辑代理用户' : '新建代理用户'} open={open} onOk={submit} onCancel={() => setOpen(false)} destroyOnClose>
        <Form form={form} layout="vertical">
          {!editing && (
            <Form.Item name="group_id" label="分组" rules={[{ required: true }]}>
              <Select options={groups.map((g) => ({ label: g.name, value: g.id }))} />
            </Form.Item>
          )}
          <Form.Item name="remark" label="备注"><Input /></Form.Item>
          <Form.Item name="password" label="协议密码（可选）"><Input /></Form.Item>
          <Form.Item name="quota_bytes" label="流量配额（-1=无限）"><InputNumber style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="expire_at" label="到期时间（RFC3339，可选）"><Input placeholder="2026-01-01T00:00:00Z" /></Form.Item>
          {editing && <Form.Item name="status" label="状态"><Select options={[{ label: 'enabled', value: 'enabled' }, { label: 'disabled', value: 'disabled' }]} /></Form.Item>}
        </Form>
      </Modal>

      <Modal title="订阅链接" open={subModal.open} footer={null} onCancel={() => setSubModal({ open: false, url: '' })}>
        <Typography.Paragraph copyable style={{ wordBreak: 'break-all' }}>
          {subModal.url.startsWith('http') ? subModal.url : window.location.origin + subModal.url}
        </Typography.Paragraph>
      </Modal>
    </div>
  )
}

function PanelUsers() {
  const [rows, setRows] = useState<PanelUser[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<PanelUser | null>(null)
  const [form] = Form.useForm()

  const load = () => {
    api.listPanelUsers().then(setRows).catch(() => setRows([]))
    api.listRoles().then(setRoles).catch(() => setRoles([]))
  }
  useEffect(load, [])

  const submit = async () => {
    const values = await form.validateFields()
    try {
      if (editing) await api.updatePanelUser(editing.id, values)
      else await api.createPanelUser(values)
      message.success('保存成功')
      setOpen(false)
      load()
    } catch (e) {
      message.error(e instanceof Error ? e.message : '保存失败')
    }
  }

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => { setEditing(null); form.resetFields(); setOpen(true) }}>新建面板用户</Button>
      </Space>
      <Table
        rowKey="id"
        dataSource={rows}
        columns={[
          { title: '用户名', dataIndex: 'username' },
          { title: '角色', dataIndex: 'role_id', render: (v) => roles.find((r) => r.id === v)?.name ?? v },
          { title: '状态', dataIndex: 'status' },
          {
            title: '操作',
            render: (_, r) => (
              <Space>
                <Button size="small" onClick={() => { setEditing(r); form.setFieldsValue({ username: r.username, role_id: r.role_id, status: r.status }); setOpen(true) }}>编辑</Button>
                <Popconfirm title="确认删除？" onConfirm={async () => { try { await api.deletePanelUser(r.id); message.success('已删除'); load() } catch (e) { message.error(e instanceof Error ? e.message : '删除失败') } }}>
                  <Button size="small" danger>删除</Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
      <Modal title={editing ? '编辑面板用户' : '新建面板用户'} open={open} onOk={submit} onCancel={() => setOpen(false)} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="password" label={editing ? '重置密码（留空不修改）' : '密码'} rules={editing ? [] : [{ required: true }]}><Input.Password /></Form.Item>
          <Form.Item name="role_id" label="角色" rules={[{ required: true }]}>
            <Select options={roles.map((r) => ({ label: r.name, value: r.id }))} />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue="enabled"><Select options={[{ label: 'enabled', value: 'enabled' }, { label: 'disabled', value: 'disabled' }]} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
