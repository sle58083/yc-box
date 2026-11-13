import { Input } from 'antd'

interface Props {
  label: string
  value: Record<string, unknown> | null
  onChange: (v: Record<string, unknown> | null) => void
}

// JSON 编辑器（用于 tls / transport 等 JSONB 字段）。
export default function JsonEditor({ label, value, onChange }: Props) {
  const text = value == null ? '' : JSON.stringify(value, null, 2)
  return (
    <div>
      <div style={{ marginBottom: 4, color: '#666' }}>{label}（JSON）</div>
      <Input.TextArea
        rows={4}
        value={text}
        placeholder='{"enabled":true,"server_name":"example.com"}'
        onChange={(e) => {
          const s = e.target.value
          if (s.trim() === '') {
            onChange(null)
            return
          }
          try {
            onChange(JSON.parse(s))
          } catch {
            // 非法 JSON 暂不提交，交由外层校验提示
          }
        }}
      />
    </div>
  )
}
