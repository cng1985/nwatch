<template>
  <div class="term-page">
    <div class="page-head">
      <div>
        <h1>在线终端</h1>
        <p>{{ summary }}</p>
      </div>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="card term-card">
      <div class="term-bar">
        <div class="term-meta">
          <span class="live" :class="liveClass"><i></i>{{ statusText }}</span>
          <code>{{ who }}</code>
        </div>
        <div class="term-actions">
          <button class="btn ghost" @click="copySelection">复制</button>
          <button class="btn ghost" @click="clearTerm">清屏</button>
          <button class="btn ghost" :disabled="status === 'demo'" @click="disconnect">断开</button>
          <button class="btn primary" :disabled="status === 'connecting' || status === 'demo'" @click="connect">重新连接</button>
        </div>
      </div>
      <div ref="host" class="term-host"></div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { ApiError, api, token } from '../api'
import { isDemo } from '../format'

const host = ref(null)
const info = ref(null)
const error = ref('')
const status = ref('closed')

let term
let fit
let socket
let observer
let disposed = false
let attempt = 0
let resizeTimer

const reasonText = {
  idle: '空闲时间过长，会话已断开',
  lifetime: '已达到最长会话时间，会话已断开',
  client: '会话已断开',
  exit: 'Shell 已退出',
}

const summary = computed(() => {
  const idle = durationText(info.value?.idleTimeoutSec, '15 分钟')
  const life = durationText(info.value?.maxLifetimeSec, '4 小时')
  const max = info.value?.maxSessions || 8
  return `在本机打开交互式 Shell，身份与 NMonitor 进程相同。空闲 ${idle}后断开，单次最长 ${life}，同时最多 ${max} 个会话。选中文本后点复制，在终端中粘贴即可。`
})

const who = computed(() => {
  const current = info.value
  if (!current) return '本机 Shell'
  const name = current.user || 'user'
  const hostName = current.hostname || 'localhost'
  const shellName = (current.shell || 'sh').split('/').pop()
  return `${name}@${hostName}:${current.cwd || '/'} · ${shellName}`
})

const statusText = computed(() => ({
  connecting: '连接中',
  connected: '已连接',
  closed: '已断开',
  demo: '演示模式',
}[status.value] || '未连接'))

const liveClass = computed(() => {
  if (status.value === 'connected') return 'on'
  if (status.value === 'closed') return 'bad'
  return ''
})

function durationText(sec, fallback) {
  const n = Number(sec)
  if (!n) return fallback
  if (n % 3600 === 0) return `${n / 3600} 小时`
  if (n % 60 === 0) return `${n / 60} 分钟`
  return `${n} 秒`
}

function plain(text) {
  return String(text || '').replace(/[\u0000-\u001f\u007f]/g, '')
}

function mountTerm() {
  term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    lineHeight: 1.2,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    theme: {
      background: '#0c1e27',
      foreground: '#d7e4ec',
      cursor: '#2fde9a',
      cursorAccent: '#0c1e27',
      selectionBackground: 'rgba(47, 222, 154, 0.28)',
    },
    scrollback: 5000,
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(host.value)
  fit.fit()
  term.onData((data) => {
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(new TextEncoder().encode(data))
    }
  })
  observer = new ResizeObserver(() => scheduleResize())
  observer.observe(host.value)
  term.focus()
}

function scheduleResize() {
  clearTimeout(resizeTimer)
  resizeTimer = setTimeout(sendResize, 40)
}

function sendResize() {
  if (!term || !fit || disposed) return
  fit.fit()
  if (!socket || socket.readyState !== WebSocket.OPEN) return
  if (term.cols < 20 || term.rows < 5) return
  socket.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
}

function closeSocket() {
  const current = socket
  socket = null
  if (!current) return
  current.onopen = null
  current.onmessage = null
  current.onerror = null
  current.onclose = null
  current.close()
}

function wsURL(ticket, cols, rows) {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const query = new URLSearchParams({
    ticket,
    cols: String(cols || 80),
    rows: String(rows || 24),
  })
  return `${proto}//${location.host}/api/shell/ws?${query}`
}

async function refreshInfo() {
  if (isDemo() || !token()) return
  try {
    info.value = await api('/api/shell')
  } catch { /* 状态栏保持上一次的环境信息 */ }
}

async function connect() {
  if (disposed || isDemo() || status.value === 'demo') return
  const current = ++attempt
  error.value = ''
  status.value = 'connecting'
  closeSocket()
  fit?.fit()
  try {
    const issued = await api('/api/shell/ticket', { method: 'POST' })
    if (current !== attempt || disposed) return
    const ws = new WebSocket(wsURL(issued.ticket, term?.cols, term?.rows))
    if (current !== attempt || disposed) {
      ws.close()
      return
    }
    socket = ws
    ws.binaryType = 'arraybuffer'
    ws.onopen = () => {
      if (socket !== ws) return
      status.value = 'connected'
      sendResize()
      term?.focus()
      refreshInfo()
    }
    ws.onmessage = (ev) => {
      if (socket !== ws || !term) return
      if (typeof ev.data === 'string') {
        handleControl(ev.data)
        return
      }
      term.write(new Uint8Array(ev.data))
    }
    ws.onerror = () => {
      if (socket !== ws) return
      error.value = '终端连接失败'
    }
    ws.onclose = () => {
      if (socket !== ws) return
      socket = null
      if (status.value === 'connected' || status.value === 'connecting') {
        status.value = 'closed'
        refreshInfo()
      }
    }
  } catch (err) {
    if (current !== attempt || disposed) return
    status.value = 'closed'
    error.value = err.message
    const message = plain(err.message)
    if (message) term?.write(`\r\n\x1b[31m${message}\x1b[0m\r\n`)
  }
}

function handleControl(raw) {
  let msg
  try {
    msg = JSON.parse(raw)
  } catch {
    return
  }
  if (msg.type === 'ready') {
    status.value = 'connected'
    return
  }
  if (msg.type === 'exit' || msg.type === 'error') {
    status.value = 'closed'
    const text = plain(msg.message || reasonText[msg.reason] || '会话已结束')
    if (msg.type === 'error') error.value = text
    term?.write(`\r\n\x1b[90m${text}\x1b[0m\r\n`)
    closeSocket()
    refreshInfo()
  }
}

function disconnect() {
  if (status.value === 'demo') return
  attempt++
  closeSocket()
  status.value = 'closed'
  refreshInfo()
}

function clearTerm() {
  term?.clear()
  term?.focus()
}

async function copySelection() {
  error.value = ''
  const text = term?.getSelection() || ''
  if (!text) {
    error.value = '请先在终端里选中要复制的文本'
    return
  }
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    error.value = '复制失败，请使用浏览器的复制菜单'
  }
}

onMounted(async () => {
  mountTerm()
  await nextTick()
  fit?.fit()
  if (isDemo()) {
    status.value = 'demo'
    term.writeln('演示模式不会连接服务器。')
    term.writeln('登录后可以在这里打开本机的交互式 Shell。')
    return
  }
  try {
    info.value = await api('/api/shell')
  } catch (err) {
    error.value = err.message
    if (err instanceof ApiError && err.status === 403) {
      term.writeln('\x1b[31m在线终端已关闭\x1b[0m')
      return
    }
  }
  if (info.value && info.value.enabled === false) {
    error.value = '在线终端已关闭'
    term.writeln('\x1b[31m在线终端已关闭\x1b[0m')
    return
  }
  await connect()
})

onBeforeUnmount(() => {
  disposed = true
  attempt++
  clearTimeout(resizeTimer)
  observer?.disconnect()
  closeSocket()
  term?.dispose()
})
</script>
