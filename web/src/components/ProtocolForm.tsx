import { Input, InputNumber, Select, Switch } from 'antd'
import { protocolMeta, type ProtocolField } from '../utils/protocols'

interface Props {
  protocol: string
  direction: 'inbound' | 'outbound'
  value: Record<string, unknown>
  onChange: (settings: Record<string, unknown>) => void
}

// 动态协议专属字段表单：按协议元数据渲染 settings 字段。
export default function ProtocolForm({ protocol, direction, value, onChange }: Props) {
  const meta = protocolMeta(protocol)
  const fields: ProtocolField[] = direction === 'inbound' ? meta?.inbound ?? [] : meta?.outbound ?? []

  const set = (key: string, v: unknown) => {
    onChange({ ...value, [key]: v })
  }

  return (
    <div>
      {fields.map((f) => (
        <div key={f.key} style={{ marginBottom: 12 }}>
          <div style={{ marginBottom: 4, color: '#666' }}>
            {f.label}
            {f.required ? <span style={{ color: '#f5222d' }}> *</span> : null}
          </div>
          <FieldInput field={f} value={value[f.key]} onChange={(v) => set(f.key, v)} />
        </div>
      ))}
    </div>
  )
}

function FieldInput({ field, value, onChange }: { field: ProtocolField; value: unknown; onChange: (v: unknown) => void }) {
  switch (field.type) {
    case 'number':
      return <InputNumber style={{ width: '100%' }} value={value as number} onChange={(v) => onChange(v ?? 0)} />
    case 'bool':
      return <Switch checked={!!value} onChange={onChange} />
    case 'select':
      return (
        <Select
          style={{ width: '100%' }}
          value={value as string}
          placeholder={field.placeholder}
          options={field.options}
          onChange={onChange}
        />
      )
    case 'array': {
      const text = Array.isArray(value) ? JSON.stringify(value) : typeof value === 'string' ? value : ''
      return (
        <Input
          value={text}
          placeholder={field.placeholder ?? '["a","b"]'}
          onChange={(e) => {
            const s = e.target.value
            try {
              onChange(JSON.parse(s))
            } catch {
              onChange(s)
            }
          }}
        />
      )
    }
    default:
      return <Input value={value as string} placeholder={field.placeholder} onChange={(e) => onChange(e.target.value)} />
  }
}
