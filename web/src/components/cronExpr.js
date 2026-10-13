export const cronFields = [
  { key: 'minute', label: '分', unit: '分钟', min: 0, max: 59 },
  { key: 'hour', label: '时', unit: '小时', min: 0, max: 23 },
  { key: 'dom', label: '日', unit: '日', min: 1, max: 31 },
  { key: 'month', label: '月', unit: '月', min: 1, max: 12 },
  { key: 'dow', label: '周', unit: '周', min: 0, max: 6 },
]

export const cronPresets = [
  { expr: '* * * * *', label: '每分钟' },
  { expr: '*/5 * * * *', label: '每 5 分钟' },
  { expr: '*/15 * * * *', label: '每 15 分钟' },
  { expr: '0 * * * *', label: '每小时' },
  { expr: '0 2 * * *', label: '每天 02:00' },
  { expr: '0 9 * * 1', label: '每周一 09:00' },
  { expr: '0 0 1 * *', label: '每月 1 日' },
]

const aliases = {
  '@yearly': '0 0 1 1 *',
  '@annually': '0 0 1 1 *',
  '@monthly': '0 0 1 * *',
  '@weekly': '0 0 * * 0',
  '@daily': '0 0 * * *',
  '@midnight': '0 0 * * *',
  '@hourly': '0 * * * *',
}

const monthNames = {
  jan: 1, feb: 2, mar: 3, apr: 4, may: 5, jun: 6,
  jul: 7, aug: 8, sep: 9, oct: 10, nov: 11, dec: 12,
}

const dowNames = { sun: 0, mon: 1, tue: 2, wed: 3, thu: 4, fri: 5, sat: 6 }

const weekLabels = ['日', '一', '二', '三', '四', '五', '六']

export function fieldByKey(key) {
  return cronFields.find((field) => field.key === key)
}

export function valueLabel(field, value) {
  if (field.key === 'dow') return '周' + weekLabels[value]
  if (field.key === 'month') return value + '月'
  if (field.key === 'dom') return String(value)
  return String(value).padStart(2, '0')
}

export function parseExpression(raw) {
  const text = expandAlias(raw)
  const tokens = text.split(/\s+/).filter(Boolean)
  if (tokens.length !== 5) return null
  return tokens.map((token, index) => parseToken(token, cronFields[index]))
}

export function canonicalExpr(raw) {
  const parsed = parseExpression(raw)
  if (!parsed) return String(raw || '').trim()
  return parsed.map((state, index) => formatField(state, cronFields[index]) || state.raw).join(' ')
}

export function formatField(state, field) {
  if (!state || state.mode === 'custom') return state?.raw || ''
  if (state.mode === 'every') return '*'
  if (state.mode === 'range') {
    const start = clamp(state.start, field)
    const end = clamp(state.end, field)
    if (start > end) return ''
    if (start === field.min && end === field.max) return '*'
    if (start === end) return String(start)
    return `${start}-${end}`
  }
  if (state.mode === 'step') {
    const start = clamp(state.start, field)
    const end = clamp(state.end ?? field.max, field)
    const step = normalizeStep(state.step)
    if (start > end) return ''
    if (step === 1) {
      if (start === field.min && end === field.max) return '*'
      if (start === end) return String(start)
      return `${start}-${end}`
    }
    if (start <= field.min && end >= field.max) return `*/${step}`
    if (end >= field.max) return `${start}/${step}`
    return `${start}-${end}/${step}`
  }
  if (state.mode === 'pick') {
    const values = [...new Set((state.values || []).map((value) => clamp(value, field)))].sort((a, b) => a - b)
    if (!values.length) return ''
    if (values.length === field.max - field.min + 1) return '*'
    return values.join(',')
  }
  return ''
}

export function replaceField(raw, key, state) {
  const field = fieldByKey(key)
  const token = formatField(state, field)
  if (!token) return String(raw || '').trim()
  const parsed = parseExpression(raw)
  const tokens = parsed
    ? parsed.map((item, index) => formatField(item, cronFields[index]) || item.raw)
    : ['*', '*', '*', '*', '*']
  tokens[cronFields.findIndex((item) => item.key === key)] = token
  return tokens.join(' ')
}

function expandAlias(raw) {
  const text = String(raw || '').trim()
  return aliases[text.toLowerCase()] || text
}

function parseToken(raw, field) {
  const text = String(raw || '').trim().toLowerCase()
  const names = field.key === 'month' ? monthNames : field.key === 'dow' ? dowNames : null
  const base = { raw: text, start: field.min, end: field.max, step: 1, values: [] }
  if (text === '*') return { ...base, mode: 'every' }
  let match = text.match(/^\*\/(\d+)$/)
  if (match && Number(match[1]) > 0) return { ...base, mode: 'step', step: Number(match[1]) }
  match = text.match(/^([a-z0-9]+)-([a-z0-9]+)(?:\/(\d+))?$/)
  if (match) {
    const start = nameOrNumber(match[1], field, names)
    const end = nameOrNumber(match[2], field, names)
    const step = match[3] ? Number(match[3]) : 1
    if (start == null || end == null || step <= 0 || start > end) return { ...base, mode: 'custom' }
    if (step === 1) return { ...base, mode: 'range', start, end }
    return { ...base, mode: 'step', start, end, step }
  }
  match = text.match(/^([a-z0-9]+)\/(\d+)$/)
  if (match) {
    const start = nameOrNumber(match[1], field, names)
    const step = Number(match[2])
    if (start == null || step <= 0) return { ...base, mode: 'custom' }
    return { ...base, mode: 'step', start, end: field.max, step }
  }
  const values = []
  for (const part of text.split(',')) {
    const value = nameOrNumber(part.trim(), field, names)
    if (value == null) return { ...base, mode: 'custom' }
    values.push(value)
  }
  const unique = [...new Set(values)].sort((a, b) => a - b)
  if (!unique.length) return { ...base, mode: 'custom' }
  return { ...base, mode: 'pick', start: unique[0], end: unique[unique.length - 1], values: unique }
}

function nameOrNumber(token, field, names) {
  if (!token) return null
  if (names && names[token] != null) return names[token]
  if (!/^\d+$/.test(token)) return null
  let value = Number(token)
  if (field.key === 'dow' && value === 7) value = 0
  if (value < field.min || value > field.max) return null
  return value
}

function clamp(value, field) {
  const number = Number(value)
  if (!Number.isFinite(number)) return field.min
  return Math.min(field.max, Math.max(field.min, Math.trunc(number)))
}

function normalizeStep(value) {
  const number = Number(value)
  if (!Number.isFinite(number) || number < 1) return 1
  return Math.trunc(number)
}
