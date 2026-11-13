import { Button, Form, Input, InputNumber, message, Modal, Popconfirm, Select, Space, Switch, Table, Tag } from 'antd'
import { useEffect, useState } from 'react'
import * as api from '../../api/inbounds'
import * as groupsApi from '../../api/groups'
import type { InboundConfig } from '../../api/types'
import type { Group } from '../../api/groups'
import { useAuthStore } from '../../store/auth'
import { inboundProtocols } from '../../utils/protocols'
import ProtocolForm from '../../components/ProtocolForm'
import JsonEditor from '../../components/JsonEditor'

export default function Inbounds() {
  const role = useAuthStore((s) => s.role)
  const canWrite = role === 'super_admin' || role === 'admin'
  const [rows, setRows] = useState<InboundConfig[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<InboundConfig | null>(null)
  const [form] = Form.useForm()
  const [settings, setSettings] = useState<Record<string, unknown>>({})
  const [tls, setTls] = useState<Record<string, unknown> | null>(null)
  const [transport, setTransport] = useState<Record<string, unknown> | null>(null)
  const protocol = Form.useWatch('type', form) ?? 'vless'

  const load = () => {
    api.listInbounds().then(setRows).catch(() => setRows([]))
    groupsApi.listGroups().then(setGroups).catch(() => setGroups([]))
  }
  useEffect(load, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ type: 'vless', listen: '0.0.0.0', port: 443, is_user_inbound: false, status: 'enabled' })
    if (groups[0]) form.setFieldValue('group_id', groups[0].id)
    setSettings({})
    setTls(null)
    setTransport(null)
    setOpen(true)
  }
  const openEdit = (r: InboundConfig) => {
    setEditing(r)
    form.setFieldsValue({ name: r.name, group_id: r.group_id, type: r.type, listen: r.listen, port: r.port, is_user_inbound: r.is_user_inbound, status: r.status })
    setSettings(r.settings ?? {})
    setTls(r.tls)
    setTransport(r.transport)
    setOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    const body = { ...values, settings, tls, transport }
    try {
      if (editing) await api.updateInbound(editing.id, body)
      else await api.createInbound(body)
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
        {canWrite && <Button type="primary" onClick={openCreate}>新建入站</Button>}
      </Space>
      <Table
        rowKey="id"
        dataSource={rows}
        columns={[
          { title: '名称(tag)', dataIndex: 'name' },
          { title: '协议', dataIndex: 'type' },
          { title: '监听', dataIndex: 'listen' },
          { title: '端口', dataIndex: 'port' },
          { title: '用户入口', dataIndex: 'is_user_inbound', render: (v) => (v ? <Tag color="blue">是</Tag> : '-') },
          { title: '状态', dataIndex: 'status', render: (v) => <Tag color={v === 'enabled' ? 'green' : 'default'}>{v}</Tag> },
          {
            title: '操作',
            render: (_, r) => (
              <Space>
                {canWrite && <Button size="small" onClick={() => openEdit(r)}>编辑</Button>}
                {canWrite && (
                  <Popconfirm title="确认删除？" onConfirm={async () => { try { await api.deleteInbound(r.id); message.success('已删除'); load() } catch (e) { message.error(e instanceof Error ? e.message : '删除失败') } }}>
                    <Button size="small" danger>删除</Button>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]}
      />

      <Modal title={editing ? '编辑入站' : '新建入站'} open={open} onOk={submit} onCancel={() => setOpen(false)} width={640} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称（sing-box tag）" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="group_id" label="分组" rules={[{ required: true }]}>
            <Select options={groups.map((g) => ({ label: g.name, value: g.id }))} />
          </Form.Item>
          <Form.Item name="type" label="协议" rules={[{ required: true }]}>
            <Select options={inboundProtocols().map((p) => ({ label: p, value: p }))} />
          </Form.Item>
          <Form.Item name="listen" label="监听地址"><Input /></Form.Item>
          <Form.Item name="port" label="端口" rules={[{ required: true }]}><InputNumber min={1} max={65535} style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="is_user_inbound" label="代理用户入口" valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="status" label="状态"><Select options={[{ label: 'enabled', value: 'enabled' }, { label: 'disabled', value: 'disabled' }]} /></Form.Item>
          <div style={{ marginBottom: 12 }}>
            <ProtocolForm protocol={protocol} direction="inbound" value={settings} onChange={setSettings} />
          </div>
          <JsonEditor label="TLS" value={tls} onChange={setTls} />
          <div style={{ height: 12 }} />
          <JsonEditor label="Transport" value={transport} onChange={setTransport} />
        </Form>
      </Modal>
    </div>
  )
}
