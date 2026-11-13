import { Card, Col, Row, Statistic, Table } from 'antd'
import { useEffect, useState } from 'react'
import * as nodesApi from '../../api/nodes'
import * as trafficApi from '../../api/traffic'
import type { Node, NodeStat } from '../../api/types'
import { fmtBytes, fmtTime } from '../../utils/format'

export default function Dashboard() {
  const [nodes, setNodes] = useState<Node[]>([])
  const [traffic, setTraffic] = useState<NodeStat[]>([])

  useEffect(() => {
    nodesApi.listNodes().then(setNodes).catch(() => setNodes([]))
    trafficApi.nodeTraffic({}).then(setTraffic).catch(() => setTraffic([]))
  }, [])

  const online = nodes.filter((n) => n.status === 'online').length
  const totalUp = traffic.reduce((s, t) => s + t.up_bytes, 0)
  const totalDown = traffic.reduce((s, t) => s + t.down_bytes, 0)

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}><Card><Statistic title="节点总数" value={nodes.length} /></Card></Col>
        <Col span={6}><Card><Statistic title="在线节点" value={online} valueStyle={{ color: online > 0 ? '#3f8600' : '#cf1322' }} /></Card></Col>
        <Col span={6}><Card><Statistic title="累计上行" value={fmtBytes(totalUp)} /></Card></Col>
        <Col span={6}><Card><Statistic title="累计下行" value={fmtBytes(totalDown)} /></Card></Col>
      </Row>
      <Card title="节点状态">
        <Table
          rowKey="id"
          dataSource={nodes}
          pagination={false}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '状态', dataIndex: 'status' },
            { title: 'Agent 版本', dataIndex: 'agent_version' },
            { title: '内核版本', dataIndex: 'singbox_version' },
            { title: '最近心跳', dataIndex: 'last_heartbeat', render: (v) => fmtTime(v) },
          ]}
        />
      </Card>
    </div>
  )
}
