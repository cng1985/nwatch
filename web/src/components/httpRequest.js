export const bodyTypes = [
  { id: 'json', label: 'JSON' },
  { id: 'form', label: 'Form' },
  { id: 'formdata', label: 'FormData' },
  { id: 'xml', label: 'XML' },
  { id: 'text', label: 'Text' },
]

export const bodyTypeLabels = {
  json: 'POST JSON',
  form: 'POST 表单',
  formdata: 'POST FormData',
  xml: 'POST XML',
  text: 'POST 文本',
}

export function headersToText(headers) {
  return Object.entries(headers || {}).map(([key, value]) => `${key}: ${value}`).join('\n')
}

export function textToHeaders(text) {
  const headers = {}
  for (const line of String(text || '').split('\n')) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const index = trimmed.indexOf(':')
    if (index <= 0) throw new Error('请求头写成 Header: 值，每行一个')
    const key = trimmed.slice(0, index).trim()
    if (!key) throw new Error('请求头写成 Header: 值，每行一个')
    headers[key] = trimmed.slice(index + 1).trim()
  }
  return headers
}

export function parsePairs(body) {
  const text = String(body || '').replace(/\r\n/g, '\n')
  if (!text.trim()) return []
  if (text.includes('\n')) {
    return text.split('\n').map((line) => {
      const trimmed = line.trim()
      if (!trimmed) return null
      const index = trimmed.indexOf('=')
      if (index <= 0) return { key: trimmed, value: '', invalid: true }
      return { key: trimmed.slice(0, index).trim(), value: trimmed.slice(index + 1).trim() }
    }).filter(Boolean)
  }
  if (!text.includes('=')) return [{ key: text.trim(), value: '', invalid: true }]
  return [...new URLSearchParams(text)].map(([key, value]) => ({ key, value }))
}

export function pairsToText(pairs) {
  const lines = (pairs || [])
    .filter((pair) => String(pair.key || '').trim())
    .map((pair) => `${String(pair.key).trim()}=${pair.value ?? ''}`)
  if (!lines.length) return ''
  let text = lines.join('\n')
  if (!text.includes('\n') && text.includes('&')) text += '\n'
  return text
}

export function encodeForm(body) {
  const params = new URLSearchParams()
  for (const pair of parsePairs(body)) {
    if (!pair.key || pair.invalid) continue
    params.append(pair.key, pair.value)
  }
  return [...params.entries()]
    .sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0))
    .map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value).replace(/%20/g, '+')}`)
    .join('&')
}

export function previewBody(bodyType, body) {
  if (bodyType === 'json') {
    const text = String(body || '').trim()
    if (!text) return ''
    try {
      return JSON.stringify(JSON.parse(text), null, 2)
    } catch {
      return text
    }
  }
  if (bodyType === 'form') return encodeForm(body)
  if (bodyType === 'formdata') {
    return parsePairs(body)
      .filter((pair) => pair.key && !pair.invalid)
      .map((pair) => `${pair.key}: ${pair.value}`)
      .join('\n')
  }
  return body || ''
}

export function previewNote(bodyType, body) {
  if (bodyType === 'json') {
    const text = String(body || '').trim()
    if (!text) return 'Content-Type: application/json'
    try {
      JSON.parse(text)
      return 'Content-Type: application/json'
    } catch {
      return 'JSON 格式还不正确'
    }
  }
  if (bodyType === 'form') return 'Content-Type: application/x-www-form-urlencoded'
  if (bodyType === 'formdata') return 'Content-Type: multipart/form-data'
  if (bodyType === 'xml') return 'Content-Type: application/xml'
  if (bodyType === 'text') return 'Content-Type: text/plain'
  return ''
}

export function requestLabel(method, bodyType) {
  if (method === 'POST') return bodyTypeLabels[bodyType] || 'POST'
  return method || 'GET'
}
