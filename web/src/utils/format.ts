// 通用格式化工具。

export function fmtBytes(n: number): string {
  if (!n || n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(2)} ${units[i]}`
}

export function fmtTime(s: string | null | undefined): string {
  if (!s) return '-'
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? s : d.toLocaleString()
}
