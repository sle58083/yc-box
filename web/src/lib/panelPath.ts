/**
 * 读取部署时注入的面板访问路径。
 *
 * 约定：nginx entrypoint 会在 index.html 中注入 `window.__PANEL_PATH__`，
 * 形如 '/a1b2c3'；为空表示根路径（兼容旧实例）。返回标准化的 basename：
 * 以 '/' 开头、结尾无 '/'，如 '/a1b2c3'；根路径返回 ''。
 */
export function getPanelPath(): string {
  const raw = typeof window !== 'undefined' ? window.__PANEL_PATH__ : undefined
  const value = (raw ?? '').trim()
  if (!value || value === '/') {
    return ''
  }
  let path = value.startsWith('/') ? value : `/${value}`
  path = path.replace(/\/+$/, '')
  return path
}
