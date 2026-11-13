import { Button, Form, Input, message, Modal, Popconfirm, Select, Space, Table, Tag, Typography } from 'antd'
import { useEffect, useState } from 'react'
import * as nodesApi from '../../api/nodes'
import * as groupsApi from '../../api/groups'
import { listVersions, deployConfig, retryConfig, rollbackConfig, previewConfig } from '../../api/config'
import type { Node } from '../../api/types'
import type { Group } from '../../api/groups'
import { useAuthStore } from '../../store/auth'
import { fmtTime } from '../../utils/format'

const statusColor: Record<string, string> = { online: 'green', offline: 'red', pending: 'orange', error: 'volcano' }

export default function Nodes() {
  const role = useAuthStore((s) => s.role)
  const canWrite = role === 'super_admin' || role === 'admin'
  const [nodes, setNodes] = useState<Node[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Node | null>(null)
  const [form] = Form.useForm()
  const [tokenModal, setTokenModal] = useState<{ open: boolean; token: string }>({ open: false, token: '' })

  const load = () => {
    nodesApi.listNodes().then(setNodes).catch(() => setNodes([]))
    groupsApi.listGroups().then(setGroups).catch(() => setGroups([]))
  }
  useEffect(load, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    if (groups[0]) form.setFieldValue('group_id', groups[0].id)
    setModalOpen(true)
  }
  const openEdit = (n: Node) => {
    setEditing(n)
    form.setFieldsValue({ name: n.name, address: n.address, group_id: n.group_id })
    setModalOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    try {
      if (editing) {
        await nodesApi.updateNode(editing.id, values)
      } else {
        const res = await nodesApi.createNode(values)
        setTokenModal({ open: true, token: res.agent_token })
      }
      message.success('保存成功')
      setModalOpen(false)
      load()
    } catch (e) {
      message.error(e instanceof Error ? e.message : '保存失败')
    }
  }

  const remove = async (id: string) => {
    try {
      await nodesApi.deleteNode(id)
      message.success('已删除')
      load()
    } catch (e) {
      message.error(e instanceof Error ? e.message : '删除失败')
    }
  }

  const regen = async (id: string) => {
    try {
      const res = await nodesApi.regenerateToken(id)
      setTokenModal({ open: true, token: res.agent_token })
    } catch (e) {
      message.error(e instanceof Error ? e.message : '生成令牌失败')
    }
  }

  const doDeploy = async (id: string) => {
    try {
      await deployConfig(id)
      message.success('下发成功')
    } catch (e) {
      message.error(e instanceof Error ? e.message : '下发失败')
    }
  }

  const doPreview = async (id: string) => {
    try {
      const doc = await previewConfig(id)
      Modal.info({
        title: '配置预览',
        width: 720,
        content: <pre style={{ maxHeight: 480, overflow: 'auto', fontSize: 12 }}>{JSON.stringify(doc, null, 2)}</pre>,
      })
    } catch (e) {
      message.error(e instanceof Error ? e.message : '预览失败')
    }
  }

  const copy = async (text: string) => {
    await navigator.clipboard.writeText(text)
    message.success('已复制')
  }

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        {canWrite && <Button type="primary" onClick={openCreate}>新建节点</Button>}
      </Space>
      <Table
        rowKey="id"
        dataSource={nodes}
        columns={[
          { title: '名称', dataIndex: 'name' },
          { title: '地址', dataIndex: 'address' },
          { title: '状态', dataIndex: 'status', render: (v) => <Tag color={statusColor[v]}>{v}</Tag> },
          { title: 'Agent', dataIndex: 'agent_version' },
          { title: '内核', dataIndex: 'singbox_version' },
          { title: '最近心跳', dataIndex: 'last_heartbeat', render: (v) => fmtTime(v) },
          {
            title: '操作',
            render: (_, n) => (
              <Space>
                {canWrite && <Button size="small" onClick={() => openEdit(n)}>编辑</Button>}
                {canWrite && <Button size="small" onClick={() => regen(n.id)}>复制令牌</Button>}
                {canWrite && <Button size="small" onClick={() => doPreview(n.id)}>预览</Button>}
                {canWrite && <Button size="small" type="primary" onClick={() => doDeploy(n.id)}>下发</Button>}
                {canWrite && <VersionsButton nodeId={n.id} />}
                {canWrite && (
                  <Popconfirm title="确认删除？" onConfirm={() => remove(n.id)}>
                    <Button size="small" danger>删除</Button>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={editing ? '编辑节点' : '新建节点'}
        open={modalOpen}
        onOk={submit}
        onCancel={() => setModalOpen(false)}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="节点名" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="group_id" label="分组" rules={[{ required: true }]}>
            <Select options={groups.map((g) => ({ label: g.name, value: g.id }))} />
          </Form.Item>
          <Form.Item name="address" label="地址"><Input /></Form.Item>
        </Form>
      </Modal>

      <Modal title="Agent 令牌（仅显示一次）" open={tokenModal.open} footer={null} onCancel={() => setTokenModal({ open: false, token: '' })}>
        <Typography.Paragraph copyable={{ text: tokenModal.token }} style={{ wordBreak: 'break-all' }}>
          {tokenModal.token}
        </Typography.Paragraph>
        <Button onClick={() => copy(tokenModal.token)}>复制到剪贴板</Button>
      </Modal>
    </div>
  )
}

function VersionsButton({ nodeId }: { nodeId: string }) {
  const [open, setOpen] = useState(false)
  const [versions, setVersions] = useState<{ id: string; version: number; status: string; created_at: string }[]>([])

  const load = async () => {
    try {
      const v = await listVersions(nodeId)
      setVersions(v)
    } catch {
      setVersions([])
    }
  }

  return (
    <>
      <Button size="small" onClick={async () => { setOpen(true); await load() }}>版本</Button>
      <Modal title="历史版本" open={open} footer={null} onCancel={() => setOpen(false)}>
        <Table
          rowKey="id"
          size="small"
          dataSource={versions}
          pagination={false}
          columns={[
            { title: '版本', dataIndex: 'version' },
            { title: '状态', dataIndex: 'status' },
            { title: '时间', dataIndex: 'created_at', render: (v) => fmtTime(v) },
            {
              title: '操作',
              render: (_, v) => (
                <Space>
                  <Button size="small" onClick={async () => { try { await retryConfig(nodeId, v.id); message.success('已重试'); } catch (e) { message.error(e instanceof Error ? e.message : '重试失败') } }}>重试</Button>
                  <Button size="small" onClick={async () => { try { await rollbackConfig(nodeId, v.id); message.success('已回滚'); } catch (e) { message.error(e instanceof Error ? e.message : '回滚失败') } }}>回滚</Button>
                </Space>
              ),
            },
          ]}
        />
      </Modal>
    </>
  )
}
