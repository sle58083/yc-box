import { Card, Col, Row } from 'antd'

export default function Settings() {
  return (
    <div>
      <Row gutter={16}>
        <Col span={8}><Card title="内核版本治理">P1：内核版本策略、升级/回滚编排</Card></Col>
        <Col span={8}><Card title="告警通道">P1：节点离线/下发失败告警</Card></Col>
        <Col span={8}><Card title="证书管理">P1：TLS 证书与 Reality 密钥托管</Card></Col>
      </Row>
    </div>
  )
}
