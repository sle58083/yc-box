import { Button, Card, Input, Space, Table, Tag } from 'antd'
import { useEffect, useState } from 'react'
import * as api from '../../api/audit'
import type { AuditLog } from '../../api/types'
import { fmtTime } from '../../utils/format'

export default function Audit() {
  const [rows, setRows] = useState<AuditLog[]>([])
  const [action, setAction] = useState('')

  const load = () => {
    api.listAuditLogs({ action: action || undefined, limit: 200 }).then(setRows).catch(() => setRows([]))
  }
  useEffect(load, [])

  return (
    <Card title="审计日志">
      <Space style={{ marginBottom: 16 }}>
        <Input placeholder="按动作筛选（如 auth.login）" value={action} onChange={(e) => setAction(e.target.value)} style={{ width: 280 }} onPressEnter={load} />
        <Button onClick={load}>查询</Button>
      </Space>
      <Table
        rowKey="id"
        dataSource={rows}
        columns={[
          { title: '时间', dataIndex: 'created_at', render: (v) => fmtTime(v) },
          { title: '动作', dataIndex: 'action', render: (v) => <Tag>{v}</Tag> },
          { title: '资源', dataIndex: 'resource' },
          { title: '操作人', dataIndex: 'operator_id' },
          { title: 'IP', dataIndex: 'ip' },
        ]}
      />
    </Card>
  )
}
