import { Input, InputNumber, Select, Switch, Tag } from 'antd'
import type { ProtocolFieldMeta } from '../api/protocols'
import { useProtocolsStore } from '../store/protocols'
import { protocolMetaFrom } from '../utils/protocols'

interface Props {
  protocol: string
  direction: 'inbound' | 'outbound'
  value: Record<string, unknown>
  onChange: (settings: Record<string, unknown>) => void
}

// 动态协议专属字段表单：按协议元数据（优先后端）渲染 settings 字段，
// 并展示后端下发的默认值（如 shadowsocks 的加密方式）。
export default function ProtocolForm({ protocol, direction, value, onChange }: Props) {
  const metas = useProtocolsStore((s) => s.metas)
  const meta = protocolMetaFrom(metas, protocol)
  const fields: ProtocolFieldMeta[] = direction === 'inbound' ? meta?.inbound ?? [] : meta?.outbound ?? []

  const set = (key: string, v: unknown) => {
    onChange({ ...value, [key]: v })
  }

  return (
    <div>
      {fields.map((f) => {
        const raw = value[f.key]
        const effective = raw === undefined && f.default !== undefined ? f.default : raw
        return (
          <div key={f.key} style={{ marginBottom: 12 }}>
            <div style={{ marginBottom: 4, color: '#666' }}>
              {f.label}
              {f.required ? <span style={{ color: '#f5222d' }}> *</span> : null}
              {f.default !== undefined ? (
                <Tag style={{ marginLeft: 6 }} color="blue">默认: {String(f.default)}</Tag>
              ) : null}
            </div>
            <FieldInput field={f} value={effective} onChange={(v) => set(f.key, v)} />
          </div>
        )
      })}
    </div>
  )
}

function FieldInput({ field, value, onChange }: { field: ProtocolFieldMeta; value: unknown; onChange: (v: unknown) => void }) {
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
          options={field.options}
          onChange={onChange}
        />
      )
    case 'array': {
      const text = Array.isArray(value) ? JSON.stringify(value) : typeof value === 'string' ? value : ''
      return (
        <Input
          value={text}
          placeholder='["a","b"]'
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
    default: {
      const placeholder = typeof field.default === 'string' ? `默认: ${field.default}` : undefined
      return <Input value={value as string} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} />
    }
  }
}
