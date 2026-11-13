import { Button, Form, Input, InputNumber, message, Modal, Popconfirm, Select, Space, Table, Tag } from 'antd'
import { useEffect, useState } from 'react'
import * as api from '../../api/routing'
import * as groupsApi from '../../api/groups'
import type { RoutingRule } from '../../api/types'
import type { Group } from '../../api/groups'
import { useAuthStore } from '../../store/auth'

export default function Routing() {
  const role = useAuthStore((s) => s.role)
  const canWrite = role === 'super_admin' || role === 'admin'
  const [rows, setRows] = useState<RoutingRule[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<RoutingRule | null>(null)
  const [form] = Form.useForm()
  const [inboundTags, setInboundTags] = useState('')

  const load = () => {
    api.listRouting().then(setRows).catch(() => setRows([]))
    groupsApi.listGroups().then(setGroups).catch(() => setGroups([]))
  }
  useEffect(load, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ rule_type: 'geosite', priority: 0 })
    if (groups[0]) form.setFieldValue('group_id', groups[0].id)
    setInboundTags('')
    setOpen(true)
  }
  const openEdit = (r: RoutingRule) => {
    setEditing(r)
    form.setFieldsValue({ name: r.name, group_id: r.group_id, rule_type: r.rule_type, value: r.value, outbound_tag: r.outbound_tag, priority: r.priority })
    setInboundTags(JSON.stringify(r.inbound_tags ?? []))
    setOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    let tags: string[] = []
    try {
      tags = inboundTags.trim() ? JSON.parse(inboundTags) : []
    } catch {
      message.error('inbound_tags 必须为 JSON 数组')
      return
    }
    const body = { ...values, inbound_tags: tags }
    try {
      if (editing) await api.updateRouting(editing.id, body)
      else await api.createRouting(body)
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
        {canWrite && <Button type="primary" onClick={openCreate}>新建规则</Button>}
      </Space>
      <Table
        rowKey="id"
        dataSource={rows}
        columns={[
          { title: '名称', dataIndex: 'name' },
          { title: '类型', dataIndex: 'rule_type', render: (v) => <Tag>{v}</Tag> },
          { title: '值', dataIndex: 'value' },
          { title: '出站', dataIndex: 'outbound_tag' },
          { title: '入站', dataIndex: 'inbound_tags', render: (v) => (Array.isArray(v) ? v.join(', ') : '-') },
          { title: '优先级', dataIndex: 'priority' },
          {
            title: '操作',
            render: (_, r) => (
              <Space>
                {canWrite && <Button size="small" onClick={() => openEdit(r)}>编辑</Button>}
                {canWrite && (
                  <Popconfirm title="确认删除？" onConfirm={async () => { try { await api.deleteRouting(r.id); message.success('已删除'); load() } catch (e) { message.error(e instanceof Error ? e.message : '删除失败') } }}>
                    <Button size="small" danger>删除</Button>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]}
      />

      <Modal title={editing ? '编辑规则' : '新建规则'} open={open} onOk={submit} onCancel={() => setOpen(false)} width={560} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="规则名" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="group_id" label="分组" rules={[{ required: true }]}>
            <Select options={groups.map((g) => ({ label: g.name, value: g.id }))} />
          </Form.Item>
          <Form.Item name="rule_type" label="规则类型" rules={[{ required: true }]}>
            <Select options={['geosite', 'geoip', 'domain', 'ip', 'protocol'].map((v) => ({ label: v, value: v }))} />
          </Form.Item>
          <Form.Item name="value" label="规则值" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="outbound_tag" label="目标出站 tag" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item label="入站 tags（JSON 数组）">
            <Input.TextArea rows={2} value={inboundTags} onChange={(e) => setInboundTags(e.target.value)} placeholder='["in-vless"]' />
          </Form.Item>
          <Form.Item name="priority" label="优先级"><InputNumber min={0} style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
