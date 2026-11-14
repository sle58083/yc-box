import { Card, Col, Row, Statistic, Table } from 'antd'
import { useEffect, useState } from 'react'
import * as api from '../../api/traffic'
import type { NodeStat, UserStat } from '../../api/types'
import { fmtBytes } from '../../utils/format'

export default function Traffic() {
  const [nodes, setNodes] = useState<NodeStat[]>([])
  const [users, setUsers] = useState<UserStat[]>([])

  useEffect(() => {
    api.nodeTraffic({}).then((v) => setNodes(v ?? [])).catch(() => setNodes([]))
    api.userTraffic({}).then((v) => setUsers(v ?? [])).catch(() => setUsers([]))
  }, [])

  const nodeUp = (nodes ?? []).reduce((s, t) => s + t.up_bytes, 0)
  const nodeDown = (nodes ?? []).reduce((s, t) => s + t.down_bytes, 0)

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={12}><Card><Statistic title="节点累计上行" value={fmtBytes(nodeUp)} /></Card></Col>
        <Col span={12}><Card><Statistic title="节点累计下行" value={fmtBytes(nodeDown)} /></Card></Col>
      </Row>
      <Card title="节点级流量" style={{ marginBottom: 16 }}>
        <Table
          rowKey="node_id"
          dataSource={nodes}
          pagination={false}
          columns={[
            { title: '节点', dataIndex: 'node_id' },
            { title: '上行', dataIndex: 'up_bytes', render: (v) => fmtBytes(v) },
            { title: '下行', dataIndex: 'down_bytes', render: (v) => fmtBytes(v) },
          ]}
        />
      </Card>
      <Card title="用户级流量">
        <Table
          rowKey="proxy_user_id"
          dataSource={users}
          pagination={false}
          columns={[
            { title: '用户', dataIndex: 'proxy_user_id', render: (v) => v ?? '节点级' },
            { title: '上行', dataIndex: 'up_bytes', render: (v) => fmtBytes(v) },
            { title: '下行', dataIndex: 'down_bytes', render: (v) => fmtBytes(v) },
          ]}
        />
      </Card>
    </div>
  )
}
