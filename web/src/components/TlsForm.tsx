import type { ReactNode } from 'react'
import { Input, Switch, Tag } from 'antd'
import type { TLSMode } from '../api/protocols'

interface Props {
  direction: 'inbound' | 'outbound'
  tlsAllowed: TLSMode[]
  tlsRequired: boolean
  value: Record<string, unknown> | null
  onChange: (v: Record<string, unknown> | null) => void
}

// 结构化 TLS 配置表单：替代原先的裸 JSON 编辑器，按协议允许的 TLS 组合动态渲染。
// 仅修改 known 字段，保留 JSON 中的其它未知字段，避免破坏已有配置。
export default function TlsForm({ direction, tlsAllowed, tlsRequired, value, onChange }: Props) {
  const supportsPlain = tlsRequired || tlsAllowed.includes('plain')
  const supportsReality = tlsAllowed.includes('reality')
  const supportsECH = tlsAllowed.includes('ech')

  if (!supportsPlain && !supportsReality && !supportsECH) {
    return null
  }

  // tlsRequired 时无值也视为 enabled=true；否则从空对象开始。
  const current: Record<string, unknown> = value ?? (tlsRequired ? { enabled: true } : {})
  const enabled = current.enabled === true || tlsRequired

  const setTop = (key: string, v: unknown) => {
    onChange({ ...current, [key]: v })
  }

  const nestedOf = (key: string): Record<string, unknown> => (current[key] as Record<string, unknown>) ?? {}

  const setNested = (key: string, subKey: string, v: unknown) => {
    onChange({ ...current, [key]: { ...nestedOf(key), [subKey]: v } })
  }

  const toggleNested = (key: string, on: boolean) => {
    if (on) {
      setNested(key, 'enabled', true)
    } else {
      const rest = { ...current }
      delete rest[key]
      onChange(rest)
    }
  }

  return (
    <div>
      <div style={{ marginBottom: 4, color: '#666' }}>TLS 配置</div>

      <div style={{ marginBottom: 12 }}>
        <span style={{ color: '#666', marginRight: 8 }}>
          启用 TLS{tlsRequired ? '（该协议必需）' : ''}
        </span>
        {tlsRequired ? (
          <Tag color="green">必需</Tag>
        ) : (
          <Switch checked={enabled} onChange={(v) => setTop('enabled', v)} />
        )}
      </div>

      {supportsPlain && direction === 'outbound' ? (
        <Field label="server_name">
          <Input
            value={(current.server_name as string) ?? ''}
            placeholder="example.com"
            onChange={(e) => setTop('server_name', e.target.value)}
          />
        </Field>
      ) : null}

      {supportsPlain && direction === 'inbound' ? (
        <>
          <Field label="certificate_path（证书路径）">
            <Input
              value={(current.certificate_path as string) ?? ''}
              placeholder="/etc/sing-box/fullchain.pem"
              onChange={(e) => setTop('certificate_path', e.target.value)}
            />
          </Field>
          <Field label="key_path（私钥路径）">
            <Input
              value={(current.key_path as string) ?? ''}
              placeholder="/etc/sing-box/privkey.pem"
              onChange={(e) => setTop('key_path', e.target.value)}
            />
          </Field>
        </>
      ) : null}

      {supportsReality ? (
        <div style={{ border: '1px dashed #d9d9d9', padding: 8, marginBottom: 12 }}>
          <div style={{ marginBottom: 8 }}>
            <span style={{ color: '#666', marginRight: 8 }}>Reality</span>
            <Switch checked={current.reality != null} onChange={(v) => toggleNested('reality', v)} />
          </div>
          {current.reality != null ? (
            <>
              <Field label="handshake server_name">
                <Input
                  value={(nestedOf('reality').server_name as string) ?? ''}
                  placeholder="www.microsoft.com"
                  onChange={(e) => setNested('reality', 'server_name', e.target.value)}
                />
              </Field>
              {direction === 'inbound' ? (
                <Field label="private_key">
                  <Input
                    value={(nestedOf('reality').private_key as string) ?? ''}
                    placeholder="🔐 私钥"
                    onChange={(e) => setNested('reality', 'private_key', e.target.value)}
                  />
                </Field>
              ) : (
                <Field label="public_key">
                  <Input
                    value={(nestedOf('reality').public_key as string) ?? ''}
                    placeholder="公钥"
                    onChange={(e) => setNested('reality', 'public_key', e.target.value)}
                  />
                </Field>
              )}
              <Field label="short_id">
                <Input
                  value={(nestedOf('reality').short_id as string) ?? ''}
                  placeholder="6ba85179e30d4fc2"
                  onChange={(e) => setNested('reality', 'short_id', e.target.value)}
                />
              </Field>
            </>
          ) : null}
        </div>
      ) : null}

      {supportsECH ? (
        <div style={{ border: '1px dashed #d9d9d9', padding: 8, marginBottom: 12 }}>
          <div style={{ marginBottom: 8 }}>
            <span style={{ color: '#666', marginRight: 8 }}>ECH</span>
            <Switch checked={current.ech != null} onChange={(v) => toggleNested('ech', v)} />
          </div>
          {current.ech != null ? (
            <Field label="config（JSON 数组）">
              <Input.TextArea
                rows={3}
                value={JSON.stringify(nestedOf('ech').config ?? [])}
                placeholder='["ech.example.com"]'
                onChange={(e) => {
                  const s = e.target.value
                  try {
                    setNested('ech', 'config', JSON.parse(s))
                  } catch {
                    setNested('ech', 'config', s)
                  }
                }}
              />
            </Field>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div style={{ marginBottom: 12 }}>
      <div style={{ marginBottom: 4, color: '#666' }}>{label}</div>
      {children}
    </div>
  )
}
