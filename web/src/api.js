export class ApiError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
}

export function token() {
  return localStorage.getItem('nmonitor_token') || ''
}

// 统一解析后端外壳：
// - 单条结果 {"code":200,"msg":"success","data":...}，应用错误同形状携带 code/msg
// - 分页结果原样输出 {"data":[...],"total":...,"page":...,"page_size":...}
// - 网络与代理错误仍可能是真实非 200
function authExpired() {
  localStorage.removeItem('nmonitor_token')
  if (location.pathname !== '/login') location.assign('/login')
}

export async function api(path, options = {}) {
  const headers = new Headers(options.headers || {})
  const current = token()
  if (current) headers.set('Authorization', 'Bearer ' + current)
  let body = options.body
  const form = typeof FormData !== 'undefined' && body instanceof FormData
  if (!form && body !== undefined && typeof body !== 'string') {
    headers.set('Content-Type', 'application/json')
    body = JSON.stringify(body)
  }
  const res = await fetch(path, { method: options.method || 'GET', headers, body })
  const text = await res.text()
  let payload = {}
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = { error: text }
    }
  }
  if (!res.ok) {
    if (res.status === 401) authExpired()
    throw new ApiError(res.status, payload.msg || payload.error || '请求失败')
  }
  if (payload.code !== undefined) {
    // 单条结果外壳
    if (payload.code === 200) return payload.data
    if (payload.code === 401) authExpired()
    throw new ApiError(payload.code, payload.msg || '请求失败')
  }
  if (payload.total !== undefined && Array.isArray(payload.data)) {
    // 直接分页结果
    return { items: payload.data, total: payload.total, page: payload.page, pageSize: payload.page_size }
  }
  return payload.data
}

export async function download(path, filename) {
  const res = await fetch(path, { headers: { Authorization: 'Bearer ' + token() } })
  if (res.status === 401) {
    authExpired()
    throw new ApiError(401, '登录已过期')
  }
  const type = res.headers.get('Content-Type') || ''
  if (!res.ok || type.includes('application/json')) {
    let message = '下载失败'
    try {
      const payload = JSON.parse(await res.text())
      if (payload.msg) message = payload.msg
      if (payload.error) message = payload.error
    } catch { /* 保留默认提示 */ }
    throw new ApiError(res.status, message)
  }
  const blob = await res.blob()
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}
