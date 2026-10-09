export class ApiError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
}

export function token() {
  return localStorage.getItem('nmonitor_token') || ''
}

export async function api(path, options = {}) {
  const headers = new Headers(options.headers || {})
  const current = token()
  if (current) headers.set('Authorization', 'Bearer ' + current)
  let body = options.body
  if (body !== undefined && typeof body !== 'string') {
    headers.set('Content-Type', 'application/json')
    body = JSON.stringify(body)
  }
  const res = await fetch(path, { method: options.method || 'GET', headers, body })
  if (res.status === 401 && !path.includes('/api/auth/login')) {
    localStorage.removeItem('nmonitor_token')
    if (location.pathname !== '/login') location.assign('/login')
  }
  const text = await res.text()
  let payload = {}
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = { error: text }
    }
  }
  if (!res.ok) throw new ApiError(res.status, payload.error || '请求失败')
  return payload.data
}

export async function download(path, filename) {
  const res = await fetch(path, { headers: { Authorization: 'Bearer ' + token() } })
  if (!res.ok) throw new ApiError(res.status, '下载失败')
  const blob = await res.blob()
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}
