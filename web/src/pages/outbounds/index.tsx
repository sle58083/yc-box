import { Button, Form, Input, message, Modal, Popconfirm, Select, Space, Table, Tag } from 'antd'
import { useEffect, useState } from 'react'
import * as api from '../../api/outbounds'
import * as groupsApi from '../../api/groups'
import type { OutboundConfig } from '../../api/types'
import type { Group } from '../../api/groups'
import { useAuthStore } from '../../store/auth'
import { outboundProtocols } from '../../utils/protocols'
import ProtocolForm from '../../components/ProtocolForm'
import JsonEditor from '../../components/JsonEditor'

export default function Outbounds() {
  const role = useAuthStore((s) => s.role)
  const canWrite = role === 'super_admin' || role === 'admin'
  const [rows, setRows] = useState<OutboundConfig[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<OutboundConfig | null>(null)
  const [form] = Form.useForm()
  const [settings, setSettings] = useState<Record<string, unknown>>({})
  const [tls, setTls] = useState<Record<string, unknown> | null>(null)
  const [transport, setTransport] = useState<Record<string, unknown> | null>(null)
  const protocol = Form.useWatch('type', form) ?? 'direct'

  const load = () => {
    api.listOutbounds().then(setRows).catch(() => setRows([]))
    groupsApi.listGroups().then(setGroups).catch(() => setGroups([]))
  }
  useEffect(load, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ type: 'direct', status: 'enabled' })
    if (groups[0]) form.setFieldValue('group_id', groups[0].id)
    setSettings({})
    setTls(null)
    setTransport(null)
    setOpen(true)
  }
  const openEdit = (r: OutboundConfig) => {
    setEditing(r)
    form.setFieldsValue({ name: r.name, group_id: r.group_id, type: r.type, status: r.status })
    setSettings(r.settings ?? {})
    setTls(r.tls)
    setTransport(r.transport)
    setOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    const body = { ...values, settings, tls, transport }
    try {
      if (editing) await api.updateOutbound(editing.id, body)
      else await api.createOutbound(body)
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
        {canWrite && <Button type="primary" onClick={openCreate}>新建出站</Button>}
      </Space>
      <Table
        rowKey="id"
        dataSource={rows}
        columns={[
          { title: '名称(tag)', dataIndex: 'name' },
          { title: '协议', dataIndex: 'type' },
          { title: '状态', dataIndex: 'status', render: (v) => <Tag color={v === 'enabled' ? 'green' : 'default'}>{v}</Tag> },
          {
            title: '操作',
            render: (_, r) => (
              <Space>
                {canWrite && <Button size="small" onClick={() => openEdit(r)}>编辑</Button>}
                {canWrite && (
                  <Popconfirm title="确认删除？" onConfirm={async () => { try { await api.deleteOutbound(r.id); message.success('已删除'); load() } catch (e) { message.error(e instanceof Error ? e.message : '删除失败') } }}>
                    <Button size="small" danger>删除</Button>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]}
      />

      <Modal title={editing ? '编辑出站' : '新建出站'} open={open} onOk={submit} onCancel={() => setOpen(false)} width={640} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称（sing-box tag）" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="group_id" label="分组" rules={[{ required: true }]}>
            <Select options={groups.map((g) => ({ label: g.name, value: g.id }))} />
          </Form.Item>
          <Form.Item name="type" label="协议" rules={[{ required: true }]}>
            <Select options={outboundProtocols().map((p) => ({ label: p, value: p }))} />
          </Form.Item>
          <Form.Item name="status" label="状态"><Select options={[{ label: 'enabled', value: 'enabled' }, { label: 'disabled', value: 'disabled' }]} /></Form.Item>
          <div style={{ marginBottom: 12 }}>
            <ProtocolForm protocol={protocol} direction="outbound" value={settings} onChange={setSettings} />
          </div>
          <JsonEditor label="TLS" value={tls} onChange={setTls} />
          <div style={{ height: 12 }} />
          <JsonEditor label="Transport" value={transport} onChange={setTransport} />
        </Form>
      </Modal>
    </div>
  )
}
