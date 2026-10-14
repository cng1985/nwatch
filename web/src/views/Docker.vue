<template>
  <div>
    <div class="page-head">
      <div>
        <h1>Docker</h1>
        <p>查看本机容器和镜像，启动、停止、重启，并查看日志。操作身份与 NMonitor 进程相同。</p>
      </div>
      <button class="btn ghost" :disabled="loading" @click="load">刷新</button>
    </div>
    <div v-if="error" class="alert-strip"><Icon name="alert" /> {{ error }}</div>
    <div v-if="notice" class="card card-pad" style="margin-bottom:14px">{{ notice }}</div>
    <div class="toolbar">
      <select v-model="view" class="select" @change="load">
        <option value="containers">容器</option>
        <option value="images">镜像</option>
      </select>
      <select v-if="view === 'containers'" v-model="state" class="select" @change="load">
        <option value="">全部状态</option>
        <option value="running">运行中</option>
        <option value="restarting">重启中</option>
        <option value="paused">已暂停</option>
        <option value="exited">已退出</option>
        <option value="created">已创建</option>
        <option value="dead">已失效</option>
      </select>
      <input v-model="keyword" class="search" :placeholder="view === 'containers' ? '搜索容器、镜像或端口' : '搜索仓库、标签或 ID'" @keyup.enter="load" />
      <button class="btn ghost" @click="load">搜索</button>
    </div>
    <div v-if="view === 'containers'" class="card table-wrap">
      <table>
        <thead>
          <tr><th>容器</th><th>状态</th><th>端口</th><th style="width:200px">操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="row in items" :key="row.name" :class="{ 'row-on': logs.name === row.name }">
            <td>
              <div class="name-cell">
                <div class="name-ico"><Icon name="docker" /></div>
                <div><b>{{ row.name }}</b><small>{{ row.image || '无镜像' }}<template v-if="row.id"> · {{ row.id }}</template></small></div>
              </div>
            </td>
            <td><span class="status" :class="stateClass(row.state)">{{ stateLabel(row.state) }}</span><small style="display:block;color:var(--muted);margin-top:2px">{{ row.status }}</small></td>
            <td>{{ row.ports || '-' }}</td>
            <td>
              <div class="actions-inline" style="flex-wrap:wrap">
                <button v-if="can('docker.write') && !alive(row)" :disabled="!!busy" @click="act(row, 'start')">{{ busy === row.name + ':start' ? '处理中' : '启动' }}</button>
                <button v-else-if="can('docker.write')" class="danger" :disabled="!!busy" @click="act(row, 'stop')">{{ busy === row.name + ':stop' ? '处理中' : '停止' }}</button>
                <button v-if="can('docker.write')" :disabled="!!busy" @click="act(row, 'restart')">重启</button>
                <button :disabled="!!busy" @click="openLogs(row)">日志</button>
              </div>
            </td>
          </tr>
          <tr v-if="!items.length"><td colspan="4" class="empty">{{ emptyText }}</td></tr>
        </tbody>
      </table>
    </div>
    <div v-else class="card table-wrap">
      <table>
        <thead>
          <tr><th>镜像</th><th>标签</th><th>大小</th><th>创建</th></tr>
        </thead>
        <tbody>
          <tr v-for="row in images" :key="row.id + row.repository + row.tag">
            <td>
              <div class="name-cell">
                <div class="name-ico"><Icon name="docker" /></div>
                <div><b>{{ row.repository }}</b><small>{{ row.id }}</small></div>
              </div>
            </td>
            <td>{{ row.tag }}</td>
            <td>{{ row.size || '-' }}</td>
            <td>{{ row.created || '-' }}</td>
          </tr>
          <tr v-if="!images.length"><td colspan="4" class="empty">{{ emptyText }}</td></tr>
        </tbody>
      </table>
    </div>
    <div v-if="logs.name" class="card card-pad" style="margin-top:14px">
      <div class="card-title">
        <div>
          <h2>{{ logs.name }}</h2>
          <p>最近 {{ logLines }} 行。日志来自 docker logs。</p>
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
    <p class="demo-note">启动和停止会等待 docker 返回。这里不进入容器，也不删除容器或镜像。关闭本功能可设置 docker.enabled 为 false，或使用环境变量 NMONITOR_DOCKER_ENABLED=false。容器里的 NMonitor 要管理宿主机 Docker 时，需要能调用 docker 客户端并访问 Docker 套接字。</p>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import { api } from '../api'
import { can } from '../session'
import { isDemo } from '../format'

const view = ref('containers')
const items = ref([])
const images = ref([])
const keyword = ref('')
const state = ref('')
const error = ref('')
const notice = ref('')
const loading = ref(false)
const busy = ref('')
const logLines = ref(200)
const follow = ref(false)
const logs = ref({ name: '', text: '' })
const emptyText = ref('没有匹配的容器')

const demoItems = [
  { id: 'abc123def456', name: 'web', image: 'nginx:1.25', state: 'running', status: 'Up 2 hours', ports: '0.0.0.0:80->80/tcp' },
  { id: 'fff999aaa111', name: 'worker', image: 'app:1', state: 'exited', status: 'Exited (1) 3 minutes ago', ports: '' },
  { id: '111222333444', name: 'batch', image: 'req:1', state: 'restarting', status: 'Restarting (1) 4 seconds ago', ports: '' },
  { id: '555666777888', name: 'nmonitor', image: 'nmonitor:latest', state: 'running', status: 'Up 6 hours', ports: '0.0.0.0:8080->8080/tcp' },
]
const demoImages = [
  { id: 'aaa111', repository: 'nginx', tag: '1.25', size: '20MB', created: '2 days ago' },
  { id: 'bbb222', repository: 'nmonitor', tag: 'latest', size: '18MB', created: '5 hours ago' },
]

function alive(row) {
  return row.state === 'running' || row.state === 'restarting' || row.state === 'paused'
}

function stateClass(value) {
  if (value === 'running') return 'UP'
  if (value === 'dead' || value === 'restarting' || value === 'removing') return 'DOWN'
  return 'PAUSED'
}

function stateLabel(value) {
  return {
    running: '运行中',
    exited: '已退出',
    paused: '已暂停',
    created: '已创建',
    restarting: '重启中',
    dead: '已失效',
    removing: '删除中',
  }[value] || '未知'
}

function matchesContainer(row) {
  const q = keyword.value.trim().toLowerCase()
  if (q && !`${row.name} ${row.image} ${row.id} ${row.ports} ${row.status}`.toLowerCase().includes(q)) return false
  if (state.value && row.state !== state.value) return false
  return true
}

function matchesImage(row) {
  const q = keyword.value.trim().toLowerCase()
  return !q || `${row.repository} ${row.tag} ${row.id}`.toLowerCase().includes(q)
}

async function load() {
  error.value = ''
  if (view.value !== 'containers') logs.value = { name: '', text: '' }
  if (isDemo()) {
    notice.value = ''
    items.value = demoItems.filter(matchesContainer)
    images.value = demoImages.filter(matchesImage)
    emptyText.value = view.value === 'containers' ? '没有匹配的容器' : '没有匹配的镜像'
    return
  }
  loading.value = true
  try {
    const params = new URLSearchParams()
    if (keyword.value.trim()) params.set('q', keyword.value.trim())
    if (view.value === 'containers') {
      if (state.value) params.set('state', state.value)
      const data = await api('/api/docker/containers' + (params.toString() ? '?' + params.toString() : ''))
      items.value = data.items || []
      applyAvailability(data, '没有匹配的容器', '当前环境无法读取容器')
    } else {
      const data = await api('/api/docker/images' + (params.toString() ? '?' + params.toString() : ''))
      images.value = data.items || []
      applyAvailability(data, '没有匹配的镜像', '当前环境无法读取镜像')
    }
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

function applyAvailability(data, empty, unavailable) {
  notice.value = data.reason || ''
  if (!data.enabled) {
    notice.value = data.reason || 'Docker 管理已关闭'
    emptyText.value = 'Docker 管理已关闭'
  } else if (!data.available) {
    notice.value = data.reason || '当前环境没有 docker'
    emptyText.value = unavailable
  } else {
    emptyText.value = empty
  }
}

async function act(row, action) {
  const labels = { start: '启动', stop: '停止', restart: '重启' }
  const self = (row.name === 'nmonitor' || row.name.startsWith('nmonitor-')) && action !== 'start'
  if (action !== 'start' || self) {
    const extra = self ? '这会中断 NMonitor 自身，页面可能无法继续操作。' : ''
    if (!window.confirm(`${extra}确定${labels[action]} ${row.name}？`)) return
  }
  error.value = ''
  if (isDemo()) {
    if (action === 'start' || action === 'restart') {
      row.state = 'running'
      row.status = 'Up 1 second'
    } else {
      row.state = 'exited'
      row.status = 'Exited (0) 1 second ago'
    }
    return
  }
  busy.value = row.name + ':' + action
  try {
    await api(`/api/docker/containers/${encodeURIComponent(row.name)}/${action}`, { method: 'POST' })
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
    logs.value = { name: logs.value.name, text: `2026-10-14T08:00:00Z demo ${logs.value.name}: 示例日志\n` }
    return
  }
  try {
    const data = await api(`/api/docker/containers/${encodeURIComponent(logs.value.name)}/logs?lines=${logLines.value}`)
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
