export function isDemo() {
  return localStorage.getItem('nmonitor_demo') === '1' && !localStorage.getItem('nmonitor_token')
}

export function todayLabel(date = new Date()) {
  const weeks = ['日', '一', '二', '三', '四', '五', '六']
  return `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日 周${weeks[date.getDay()]}`
}

export function statusLabel(status) {
  return { UP: '正常运行', DOWN: '服务异常', PAUSED: '已暂停', UNKNOWN: '等待检测' }[status] || '等待检测'
}

export function joinNames(items) {
  const names = (items || []).map((item) => item.name).filter(Boolean)
  if (names.length <= 1) return names[0] || ''
  if (names.length === 2) return `${names[0]}与${names[1]}`
  return `${names.slice(0, -1).join('、')}与${names.at(-1)}`
}

export const statusText = {
  UP: '正常',
  DOWN: '异常',
  UNKNOWN: '未知',
  PAUSED: '暂停',
  NORMAL: '正常',
  WARNING: '预警',
  CRITICAL: '严重',
  EXPIRED: '已过期',
  INVALID: '无效',
}

export const typeText = { http: 'HTTP', tls: '证书', tcp: 'TCP', cpu: 'CPU', memory: '内存', disk: '磁盘', script: '脚本' }

export const eventText = {
  MONITOR_DOWN: '服务异常',
  MONITOR_RECOVERED: '服务恢复',
  TLS_WARNING: '证书预警',
  TLS_CRITICAL: '证书严重',
  TLS_EXPIRED: '证书过期',
  TLS_INVALID: '证书无效',
  TLS_RECOVERED: '证书恢复',
  TEST: '通知测试',
}

export const intervals = [
  [15, '15 秒'],
  [30, '30 秒'],
  [60, '1 分钟'],
  [300, '5 分钟'],
  [600, '10 分钟'],
  [1800, '30 分钟'],
  [3600, '1 小时'],
  [21600, '6 小时'],
  [43200, '12 小时'],
  [86400, '24 小时'],
]

export function intervalText(seconds) {
  const found = intervals.find((item) => item[0] === seconds)
  if (found) return found[1]
  if (!seconds) return '-'
  if (seconds % 86400 === 0) return seconds / 86400 + ' 天'
  if (seconds % 3600 === 0) return seconds / 3600 + ' 小时'
  if (seconds % 60 === 0) return seconds / 60 + ' 分钟'
  return seconds + ' 秒'
}

export function relative(value) {
  if (!value) return '-'
  const diff = (Date.now() - new Date(value).getTime()) / 1000
  if (Number.isNaN(diff)) return '-'
  if (diff < 60) return Math.max(1, Math.floor(diff)) + '秒前'
  if (diff < 3600) return Math.floor(diff / 60) + '分钟前'
  if (diff < 86400) return Math.floor(diff / 3600) + '小时前'
  return Math.floor(diff / 86400) + '天前'
}

export function clock(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  const p = (n) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${p(date.getMonth() + 1)}-${p(date.getDate())} ${p(date.getHours())}:${p(date.getMinutes())}:${p(date.getSeconds())}`
}

export function duration(seconds, from) {
  let sec = seconds
  if ((sec === undefined || sec === null || sec === 0) && from) {
    sec = Math.max(0, Math.floor((Date.now() - new Date(from).getTime()) / 1000))
  }
  if (sec === undefined || sec === null) return '-'
  sec = Math.max(0, Math.floor(sec))
  const days = Math.floor(sec / 86400)
  const hours = Math.floor((sec % 86400) / 3600)
  const mins = Math.floor((sec % 3600) / 60)
  const rest = sec % 60
  if (days > 0) return `${days}天${hours}小时`
  if (hours > 0) return `${hours}小时${mins}分`
  if (mins > 0) return `${mins}分${rest}秒`
  return `${rest}秒`
}

export function targetOf(row) {
  if (!row) return '-'
  if (row.type === 'cpu') return `CPU 超过 ${row.threshold || 90}%`
  if (row.type === 'memory') return `内存超过 ${row.threshold || 90}%`
  if (row.type === 'disk') return `${row.host || '/'} 超过 ${row.threshold || 90}%`
  if (row.type === 'script') return (row.command || '').split('\n').map((line) => line.trim()).find(Boolean) || '脚本'
  if (row.type === 'http') return row.url || '-'
  if (row.url) return row.url
  if (row.host) return row.port ? `${row.host}:${row.port}` : row.host
  return '-'
}

export function iconOf(row) {
  const type = row?.type
  if (type === 'tcp') return 'db'
  if (type === 'tls') return 'cert'
  if (type === 'cpu') return 'cpu'
  if (type === 'memory') return 'mem'
  if (type === 'disk') return 'disk'
  if (type === 'script') return 'script'
  if ((row?.url || '').startsWith('https')) return 'lock'
  return 'globe'
}

export function bytes(value) {
  let n = Number(value) || 0
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return (i === 0 ? n.toFixed(0) : n.toFixed(1)) + ' ' + units[i]
}

export function percent(value, has) {
  if (!has && (value === 0 || value === undefined || value === null)) return '-'
  return Number(value).toFixed(2) + '%'
}
