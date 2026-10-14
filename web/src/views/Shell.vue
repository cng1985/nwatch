<template>
  <div class="term-page">
    <div class="page-head">
      <div>
        <h1>在线终端</h1>
        <p>{{ summary }}</p>
      </div>
      <div class="head-actions">
        <button class="btn ghost" @click="openAudits">命令记录</button>
        <button class="btn primary" :disabled="!canAdd" @click="openTab()">新建终端</button>
      </div>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="card term-card">
      <div class="term-tabs">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          type="button"
          class="term-tab"
          :class="{ on: tab.id === activeId }"
          @click="activate(tab.id)"
        >
          <i class="term-dot" :class="dotClass(tab.status)"></i>
          <span>{{ tab.title }}</span>
          <span class="term-tab-x" role="button" aria-label="关闭终端" @click.stop="closeTab(tab.id)">×</span>
        </button>
        <button class="term-tab-add" type="button" :disabled="!canAdd" aria-label="新建终端" @click="openTab()">+</button>
      </div>
      <div class="term-bar">
        <div class="term-meta">
          <span class="live" :class="liveClass"><i></i>{{ statusText }}</span>
          <code>{{ who }}</code>
        </div>
        <div class="term-actions">
          <button class="btn ghost" :disabled="!active" @click="copySelection">复制</button>
          <button class="btn ghost" :disabled="!active" @click="clearTerm">清屏</button>
          <button class="btn ghost" :disabled="!active || activeStatus === 'demo'" @click="disconnect">断开</button>
          <button class="btn primary" :disabled="!active || activeStatus === 'connecting' || activeStatus === 'demo'" @click="connect(activeId)">重新连接</button>
        </div>
      </div>
      <div v-if="!tabs.length" class="term-empty">
        <p>还没有打开终端</p>
        <button class="btn primary" :disabled="!canAdd" @click="openTab()">打开终端</button>
      </div>
      <div
        v-for="tab in tabs"
        :id="'term-host-' + tab.id"
        :key="'host-' + tab.id"
        v-show="tab.id === activeId"
        class="term-host"
      ></div>
    </div>
    <div v-if="auditOpen" class="drawer-mask" @click.self="auditOpen = false">
      <aside class="drawer" role="dialog" aria-modal="true" aria-label="命令记录">
        <div class="drawer-head">
          <div>
            <h2>命令记录</h2>
            <p>记录登录用户在在线终端提交的命令。密码提示后的输入不会写入。</p>
          </div>
          <button class="drawer-close" type="button" aria-label="关闭" @click="auditOpen = false"><Icon name="close" /></button>
        </div>
        <div class="drawer-body">
          <div class="drawer-filters">
            <input v-model="auditUser" class="search" placeholder="用户名" @keyup.enter="auditPage = 1; loadAudits()" />
            <input v-model="auditKeyword" class="search" placeholder="搜索命令" @keyup.enter="auditPage = 1; loadAudits()" />
            <div class="drawer-actions">
              <button class="btn ghost" @click="auditPage = 1; loadAudits()">查询</button>
            </div>
          </div>
          <p v-if="auditError" class="error" style="margin:0 20px">{{ auditError }}</p>
          <div class="table-wrap">
            <table>
              <thead>
                <tr><th style="width:168px">时间</th><th style="width:100px">用户</th><th style="width:120px">会话</th><th>命令</th></tr>
              </thead>
              <tbody>
                <tr v-for="row in audits" :key="row.id">
                  <td>{{ clock(row.createdAt) }}</td>
                  <td>{{ row.username }}<small style="display:block;color:#7e8c98">{{ row.remote }}</small></td>
                  <td><code>{{ row.sessionId }}</code></td>
                  <td class="cmd">{{ row.command }}</td>
                </tr>
                <tr v-if="!audits.length"><td colspan="4" class="empty">这个条件下没有命令记录</td></tr>
              </tbody>
            </table>
          </div>
          <div class="pager">
            <button class="btn ghost" :disabled="auditPage === 1" @click="auditPage--; loadAudits()">上一页</button>
            <span class="demo-note">{{ auditTotal }} 条</span>
            <button class="btn ghost" :disabled="auditPage * 20 >= auditTotal" @click="auditPage++; loadAudits()">下一页</button>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import Icon from '../components/Icon.vue'
import { ApiError, api, token } from '../api'
import { clock, isDemo } from '../format'

const info = ref(null)
const pageError = ref('')
const tabs = ref([])
const activeId = ref(0)
const auditOpen = ref(false)
const audits = ref([])
const auditTotal = ref(0)
const auditPage = ref(1)
const auditUser = ref('')
const auditKeyword = ref('')
const auditError = ref('')

const terms = new Map()
let disposed = false
let seq = 0

const reasonText = {
  idle: '空闲时间过长，会话已断开',
  lifetime: '已达到最长会话时间，会话已断开',
  client: '会话已断开',
  exit: 'Shell 已退出',
}

const maxSessions = computed(() => info.value?.maxSessions || 8)
const active = computed(() => tabs.value.find((tab) => tab.id === activeId.value) || null)
const activeStatus = computed(() => active.value?.status || 'closed')
const error = computed(() => active.value?.error || pageError.value)

const canAdd = computed(() => {
  if (isDemo()) return tabs.value.length < 8
  if (info.value && info.value.enabled === false) return false
  const live = tabs.value.filter((tab) => tab.status === 'connected' || tab.status === 'connecting').length
  return live < maxSessions.value
})

const summary = computed(() => {
  const idle = durationText(info.value?.idleTimeoutSec, '15 分钟')
  const life = durationText(info.value?.maxLifetimeSec, '4 小时')
  return `可以同时打开多个本机 Shell，身份与 NMonitor 进程相同。空闲 ${idle}后断开，单次最长 ${life}，同时最多 ${maxSessions.value} 个会话。提交的命令会记下登录用户、会话和来源地址。`
})

const who = computed(() => {
  const current = info.value
  if (!current) return '本机 Shell'
  const name = current.user || 'user'
  const hostName = current.hostname || 'localhost'
  const shellName = (current.shell || 'sh').split(/[/\\]/).pop()
  const sid = active.value?.sessionId ? ` · ${active.value.sessionId}` : ''
  return `${name}@${hostName}:${current.cwd || '/'} · ${shellName}${sid}`
})

const statusText = computed(() => ({
  connecting: '连接中',
  connected: '已连接',
  closed: '已断开',
  demo: '演示模式',
}[activeStatus.value] || '未连接'))

const liveClass = computed(() => {
  if (activeStatus.value === 'connected') return 'on'
  if (activeStatus.value === 'closed') return 'bad'
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

function tabById(id) {
  return tabs.value.find((tab) => tab.id === id)
}

function dotClass(status) {
  if (status === 'connected') return 'on'
  if (status === 'closed') return 'bad'
  return ''
}

function mountTerm(id, el) {
  const item = terms.get(id)
  if (!item || !el) return
  const term = new Terminal({
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
  const fit = new FitAddon()
  term.loadAddon(fit)
  term.open(el)
  fit.fit()
  term.onData((data) => {
    const current = terms.get(id)
    if (current?.socket && current.socket.readyState === WebSocket.OPEN) {
      current.socket.send(new TextEncoder().encode(data))
    }
  })
  const observer = new ResizeObserver(() => scheduleResize(id))
  observer.observe(el)
  item.term = term
  item.fit = fit
  item.observer = observer
  if (id === activeId.value) term.focus()
}

function scheduleResize(id) {
  const item = terms.get(id)
  if (!item) return
  clearTimeout(item.resizeTimer)
  item.resizeTimer = setTimeout(() => sendResize(id), 40)
}

function sendResize(id) {
  const item = terms.get(id)
  if (!item?.term || !item.fit || disposed || id !== activeId.value) return
  item.fit.fit()
  if (!item.socket || item.socket.readyState !== WebSocket.OPEN) return
  if (item.term.cols < 20 || item.term.rows < 5) return
  item.socket.send(JSON.stringify({ type: 'resize', cols: item.term.cols, rows: item.term.rows }))
}

function closeSocket(id) {
  const item = terms.get(id)
  const current = item?.socket
  if (!item || !current) return
  item.socket = null
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

async function openTab(opts = {}) {
  if (!opts.force && tabs.value.length > 0 && !canAdd.value) {
    pageError.value = `同时最多 ${maxSessions.value} 个终端`
    return
  }
  const id = ++seq
  tabs.value.push({ id, title: `终端 ${id}`, status: 'closed', error: '', sessionId: '' })
  activeId.value = id
  terms.set(id, { attempt: 0, socket: null, term: null, fit: null, observer: null, resizeTimer: null })
  await nextTick()
  const el = document.getElementById('term-host-' + id)
  if (!el || disposed) return
  mountTerm(id, el)
  await nextTick()
  terms.get(id)?.fit?.fit()
  if (isDemo()) {
    const tab = tabById(id)
    if (tab) tab.status = 'demo'
    const term = terms.get(id)?.term
    term?.writeln('演示模式不会连接服务器。')
    term?.writeln('登录后可以在这里同时打开多个本机 Shell，并记录每个用户执行的命令。')
    return
  }
  if (opts.blocked || (info.value && info.value.enabled === false)) {
    const tab = tabById(id)
    const message = pageError.value || '在线终端已关闭'
    if (tab) tab.error = message
    terms.get(id)?.term?.writeln(`\x1b[31m${plain(message) || '在线终端已关闭'}\x1b[0m`)
    return
  }
  await connect(id)
}

async function connect(id) {
  if (disposed || isDemo()) return
  const item = terms.get(id)
  const tab = tabById(id)
  if (!item || !tab || tab.status === 'demo') return
  const current = ++item.attempt
  tab.error = ''
  pageError.value = ''
  tab.status = 'connecting'
  closeSocket(id)
  item.fit?.fit()
  try {
    const issued = await api('/api/shell/ticket', { method: 'POST' })
    if (current !== item.attempt || disposed || !terms.has(id)) return
    const ws = new WebSocket(wsURL(issued.ticket, item.term?.cols, item.term?.rows))
    if (current !== item.attempt || disposed || !terms.has(id)) {
      ws.close()
      return
    }
    item.socket = ws
    ws.binaryType = 'arraybuffer'
    ws.onopen = () => {
      if (item.socket !== ws) return
      tab.status = 'connected'
      sendResize(id)
      if (id === activeId.value) item.term?.focus()
      refreshInfo()
    }
    ws.onmessage = (ev) => {
      if (item.socket !== ws || !item.term) return
      if (typeof ev.data === 'string') {
        handleControl(id, ev.data)
        return
      }
      item.term.write(new Uint8Array(ev.data))
    }
    ws.onerror = () => {
      if (item.socket !== ws) return
      tab.error = '终端连接失败'
    }
    ws.onclose = () => {
      if (item.socket !== ws) return
      item.socket = null
      if (tab.status === 'connected' || tab.status === 'connecting') {
        tab.status = 'closed'
        refreshInfo()
      }
    }
  } catch (err) {
    if (current !== item.attempt || disposed) return
    tab.status = 'closed'
    tab.error = err.message
    const message = plain(err.message)
    if (message) item.term?.write(`\r\n\x1b[31m${message}\x1b[0m\r\n`)
  }
}

function handleControl(id, raw) {
  const item = terms.get(id)
  const tab = tabById(id)
  if (!item || !tab) return
  let msg
  try {
    msg = JSON.parse(raw)
  } catch {
    return
  }
  if (msg.type === 'ready') {
    tab.status = 'connected'
    tab.sessionId = msg.sessionId || ''
    return
  }
  if (msg.type === 'exit' || msg.type === 'error') {
    tab.status = 'closed'
    const text = plain(msg.message || reasonText[msg.reason] || '会话已结束')
    if (msg.type === 'error') tab.error = text
    item.term?.write(`\r\n\x1b[90m${text}\x1b[0m\r\n`)
    closeSocket(id)
    refreshInfo()
  }
}

function activate(id) {
  if (!tabById(id)) return
  activeId.value = id
  nextTick(() => {
    const item = terms.get(id)
    item?.fit?.fit()
    sendResize(id)
    item?.term?.focus()
  })
}

function closeTab(id) {
  const item = terms.get(id)
  if (item) {
    item.attempt += 1
    clearTimeout(item.resizeTimer)
    item.observer?.disconnect()
    closeSocket(id)
    item.term?.dispose()
  }
  terms.delete(id)
  const idx = tabs.value.findIndex((tab) => tab.id === id)
  if (idx >= 0) tabs.value.splice(idx, 1)
  if (activeId.value === id) {
    const next = tabs.value[idx] || tabs.value[idx - 1]
    activeId.value = next?.id || 0
    if (next) activate(next.id)
  }
  refreshInfo()
}

function disconnect() {
  const id = activeId.value
  const tab = tabById(id)
  const item = terms.get(id)
  if (!tab || !item || tab.status === 'demo') return
  item.attempt += 1
  closeSocket(id)
  tab.status = 'closed'
  refreshInfo()
}

function clearTerm() {
  const item = terms.get(activeId.value)
  item?.term?.clear()
  item?.term?.focus()
}

async function copySelection() {
  const tab = tabById(activeId.value)
  const item = terms.get(activeId.value)
  if (tab) tab.error = ''
  const text = item?.term?.getSelection() || ''
  if (!text) {
    if (tab) tab.error = '请先在终端里选中要复制的文本'
    return
  }
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    if (tab) tab.error = '复制失败，请使用浏览器的复制菜单'
  }
}

function openAudits() {
  auditOpen.value = true
  auditPage.value = 1
  auditError.value = ''
  loadAudits()
}

async function loadAudits() {
  auditError.value = ''
  if (isDemo()) {
    audits.value = [
      { id: 1, username: 'admin', sessionId: 'demo0001', remote: '127.0.0.1', command: 'uname -a', createdAt: new Date().toISOString() },
      { id: 2, username: 'ops', sessionId: 'demo0002', remote: '10.0.0.8', command: 'systemctl status nmonitor', createdAt: new Date(Date.now() - 60000).toISOString() },
    ]
    auditTotal.value = audits.value.length
    return
  }
  try {
    const params = new URLSearchParams({ page: String(auditPage.value), pageSize: '20' })
    if (auditUser.value.trim()) params.set('username', auditUser.value.trim())
    if (auditKeyword.value.trim()) params.set('keyword', auditKeyword.value.trim())
    const data = await api('/api/shell/commands?' + params.toString())
    audits.value = data.items || []
    auditTotal.value = data.total || 0
  } catch (err) {
    auditError.value = err.message
  }
}

onMounted(async () => {
  if (!isDemo() && token()) {
    try {
      info.value = await api('/api/shell')
    } catch (err) {
      pageError.value = err.message
      await openTab({ force: true, blocked: err instanceof ApiError && err.status === 403 })
      return
    }
  }
  await openTab({ force: true })
})

onBeforeUnmount(() => {
  disposed = true
  for (const tab of tabs.value) {
    const item = terms.get(tab.id)
    if (!item) continue
    item.attempt += 1
    clearTimeout(item.resizeTimer)
    item.observer?.disconnect()
    closeSocket(tab.id)
    item.term?.dispose()
  }
  terms.clear()
})
</script>
