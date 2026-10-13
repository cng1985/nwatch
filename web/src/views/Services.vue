<template>
  <div>
    <div class="page-head">
      <div>
        <h1>系统服务</h1>
        <p>查看本机 systemd 服务，启动、停止、重启，并查看 journal 日志。操作身份与 NMonitor 进程相同。</p>
      </div>
      <button class="btn ghost" :disabled="loading" @click="load">刷新</button>
    </div>
    <div v-if="error" class="alert-strip"><Icon name="alert" /> {{ error }}</div>
    <div v-if="notice" class="card card-pad" style="margin-bottom:14px">{{ notice }}</div>
    <div class="toolbar">
      <select v-model="state" class="select" @change="load">
        <option value="">全部状态</option>
        <option value="active">运行中</option>
        <option value="inactive">已停止</option>
        <option value="failed">失败</option>
      </select>
      <input v-model="keyword" class="search" placeholder="搜索服务名称或说明" @keyup.enter="load" />
      <button class="btn ghost" @click="load">搜索</button>
    </div>
    <div class="card table-wrap">
      <table>
        <thead>
          <tr><th>服务</th><th>状态</th><th>开机启动</th><th style="width:180px">操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="row in items" :key="row.name" :class="{ 'row-on': logs.name === row.name }">
            <td>
              <div class="name-cell">
                <div class="name-ico"><Icon name="service" /></div>
                <div><b>{{ row.name }}</b><small>{{ row.description || '无说明' }}</small></div>
              </div>
            </td>
            <td><span class="status" :class="stateClass(row)">{{ stateLabel(row) }}</span></td>
            <td>{{ bootLabel(row.unitFile) }}</td>
            <td>
              <div class="actions-inline">
                <button v-if="!running(row)" :disabled="!!busy" @click="act(row, 'start')">{{ busy === row.name + ':start' ? '处理中' : '启动' }}</button>
                <button v-else class="danger" :disabled="!!busy" @click="act(row, 'stop')">{{ busy === row.name + ':stop' ? '处理中' : '停止' }}</button>
                <button :disabled="!!busy" @click="act(row, 'restart')">重启</button>
                <button :disabled="!!busy" @click="openLogs(row)">日志</button>
              </div>
            </td>
          </tr>
          <tr v-if="!items.length"><td colspan="4" class="empty">{{ emptyText }}</td></tr>
        </tbody>
      </table>
    </div>
    <div v-if="logs.name" class="card card-pad" style="margin-top:14px">
      <div class="card-title">
        <div>
          <h2>{{ logs.name }}</h2>
          <p>最近 {{ logLines }} 行。日志来自 journalctl。</p>
        </div>
        <div class="head-actions">
          <select v-model.number="logLines" class="select" @change="loadLogs">
            <option :value="50">50 行</option>
            <option :value="200">200 行</option>
            <option :value="500">500 行</option>
          </select>
          <label class="demo-note" style="display:flex;align-items:center;gap:6px;margin:0"><input v-model="follow" type="checkbox" /> 自动刷新</label>
          <button class="btn ghost" @click="loadLogs">刷新</button>
        </div>
      </div>
      <div class="log-box" style="max-height:480px">{{ logs.text || '没有日志' }}</div>
    </div>
    <p class="demo-note">启动和停止会等待 systemctl 返回。systemd、init 不能在这里操作。关闭本功能可设置 service.enabled 为 false，或使用环境变量 NMONITOR_SERVICE_ENABLED=false。</p>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import { api } from '../api'
import { isDemo } from '../format'

const items = ref([])
const keyword = ref('')
const state = ref('')
const error = ref('')
const notice = ref('')
const loading = ref(false)
const busy = ref('')
const logLines = ref(200)
const follow = ref(false)
const logs = ref({ name: '', text: '' })
const emptyText = ref('没有匹配的服务')

const demoItems = [
  { name: 'nginx.service', description: 'A high performance web server', load: 'loaded', active: 'failed', sub: 'failed', unitFile: 'enabled' },
  { name: 'sshd.service', description: 'OpenSSH server daemon', load: 'loaded', active: 'active', sub: 'running', unitFile: 'enabled' },
  { name: 'cron.service', description: 'Regular background program processing daemon', load: 'loaded', active: 'inactive', sub: 'dead', unitFile: 'disabled' },
  { name: 'nmonitor.service', description: 'NMonitor lightweight service monitor', load: 'loaded', active: 'active', sub: 'running', unitFile: 'enabled' },
]

function running(row) {
  return row.active === 'active' || row.active === 'activating' || row.active === 'reloading'
}

function stateClass(row) {
  if (row.active === 'failed') return 'DOWN'
  if (running(row)) return 'UP'
  return 'PAUSED'
}

function stateLabel(row) {
  const map = {
    'active:running': '运行中',
    'active:exited': '已完成',
    'inactive:dead': '已停止',
    failed: '失败',
    activating: '启动中',
    deactivating: '停止中',
    reloading: '重载中',
  }
  return map[`${row.active}:${row.sub}`] || map[row.active] || row.active || '未知'
}

function bootLabel(value) {
  return {
    enabled: '开机启动',
    disabled: '未启用',
    static: '静态',
    masked: '已屏蔽',
    indirect: '间接',
    generated: '生成',
    alias: '别名',
    transient: '临时',
  }[value] || value || '-'
}

function matches(row) {
  const q = keyword.value.trim().toLowerCase()
  if (q && !`${row.name} ${row.description || ''}`.toLowerCase().includes(q)) return false
  if (state.value === 'active') return running(row)
  if (state.value === 'inactive') return row.active === 'inactive' || row.active === 'deactivating'
  if (state.value === 'failed') return row.active === 'failed'
  return true
}

async function load() {
  error.value = ''
  if (isDemo()) {
    notice.value = ''
    items.value = demoItems.filter(matches)
    emptyText.value = '没有匹配的服务'
    return
  }
  loading.value = true
  try {
    const params = new URLSearchParams()
    if (state.value) params.set('state', state.value)
    if (keyword.value.trim()) params.set('q', keyword.value.trim())
    const data = await api('/api/services' + (params.toString() ? '?' + params.toString() : ''))
    items.value = data.items || []
    notice.value = data.reason || ''
    if (!data.enabled) {
      notice.value = data.reason || '系统服务管理已关闭'
      emptyText.value = '系统服务管理已关闭'
    } else if (!data.available) {
      notice.value = data.reason || '当前环境没有 systemd'
      emptyText.value = '当前环境无法读取服务'
    } else {
      emptyText.value = '没有匹配的服务'
    }
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function act(row, action) {
  const labels = { start: '启动', stop: '停止', restart: '重启' }
  const self = row.name === 'nmonitor.service' && action !== 'start'
  const ask = action !== 'start' || self
  if (ask) {
    const extra = self ? '这会中断 NMonitor 自身，页面可能无法继续操作。' : ''
    if (!window.confirm(`${extra}确定${labels[action]} ${row.name}？`)) return
  }
  error.value = ''
  if (isDemo()) {
    if (action === 'start' || action === 'restart') {
      row.active = 'active'
      row.sub = 'running'
    } else {
      row.active = 'inactive'
      row.sub = 'dead'
    }
    return
  }
  busy.value = row.name + ':' + action
  try {
    await api(`/api/services/${encodeURIComponent(row.name)}/${action}`, { method: 'POST' })
    await load()
    if (logs.value.name === row.name) await loadLogs()
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function openLogs(row) {
  logs.value = { name: row.name, text: logs.value.name === row.name ? logs.value.text : '' }
  await loadLogs()
}

async function loadLogs() {
  if (!logs.value.name) return
  if (isDemo()) {
    logs.value = { name: logs.value.name, text: `2026-10-13T08:00:00+0000 demo ${logs.value.name}: 示例日志\n` }
    return
  }
  try {
    const data = await api(`/api/services/${encodeURIComponent(logs.value.name)}/logs?lines=${logLines.value}`)
    logs.value = { name: data.name || logs.value.name, text: data.text || '没有日志' }
  } catch (err) {
    error.value = err.message
  }
}

let timer
onMounted(() => {
  load()
  timer = setInterval(() => { if (follow.value) loadLogs() }, 3000)
})
onUnmounted(() => clearInterval(timer))
</script>
