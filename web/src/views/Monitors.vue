<template>
  <div class="monitor-page">
    <div class="page-head">
      <div><h1>监控列表</h1><p>按状态查看全部监控，异常会排在最上方。可以导出完整 Excel，也可以只按名称、分组、类型、地址导入导出网络请求。</p></div>
      <div class="head-actions">
        <button class="btn ghost" type="button" :disabled="exporting" @click="exportRequests"><Icon name="download" /> 导出网络请求</button>
        <button v-if="can('monitor.write')" class="btn ghost" type="button" :disabled="importing" @click="requestFileRef.click()"><Icon name="upload" /> 导入网络请求</button>
        <button class="btn ghost" type="button" :disabled="exporting" @click="downloadRequestTemplate">请求模板</button>
        <button class="btn ghost" type="button" :disabled="exporting" @click="exportExcel"><Icon name="download" /> 导出</button>
        <button v-if="can('monitor.write')" class="btn ghost" type="button" :disabled="importing" @click="fileRef.click()"><Icon name="upload" /> 导入</button>
        <button class="btn ghost" type="button" :disabled="exporting" @click="downloadTemplate">下载模板</button>
        <router-link v-if="can('monitor.write')" class="btn primary" to="/monitors/new"><Icon name="plus" style="width:16px;height:16px" /> 新增监控</router-link>
      </div>
    </div>
    <p class="demo-note">网络请求表格只有名称、分组、类型、地址。类型填 http 或 https。按名称更新已有网络请求，不改检测间隔、请求头和通知渠道，也不删除表格里没有的项。</p>
    <p class="demo-note">完整表格按编号更新已有监控；编号留空时按名称新建或更新。不会删除表格里没有的监控。从其他环境导入时，请先清空编号列。</p>
    <p v-if="notice" class="demo-note ok-note">{{ notice }}</p>
    <p v-if="error" class="error">{{ error }}</p>
    <input ref="fileRef" hidden type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" @change="importExcel" />
    <input ref="requestFileRef" hidden type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" @change="importRequests" />
    <div class="toolbar">
      <input v-model="query.keyword" class="search" placeholder="搜索名称或地址" @keyup.enter="search" />
      <select v-model="query.type" class="select" @change="search"><option value="">全部类型</option><option value="http">HTTP</option><option value="tls">证书</option><option value="tcp">TCP</option><option value="cpu">CPU</option><option value="memory">内存</option><option value="disk">磁盘</option><option value="script">脚本</option><option value="process">进程</option></select>
      <select v-model="query.status" class="select" @change="search"><option value="">全部状态</option><option value="UP">正常运行</option><option value="DOWN">服务异常</option><option value="PAUSED">已暂停</option><option value="UNKNOWN">等待检测</option></select>
      <select v-if="can('group.read')" v-model="query.groupId" class="select" @change="search">
        <option value="">全部分组</option>
        <option v-for="g in groups" :key="g.id" :value="String(g.id)">{{ g.name }}</option>
      </select>
      <button class="btn ghost" @click="search">查询</button>
      <div class="seg view-switch" role="group" aria-label="显示方式">
        <button type="button" :class="{ on: view === 'list' }" :aria-pressed="view === 'list'" @click="setView('list')"><Icon name="list" /> 列表</button>
        <button type="button" :class="{ on: view === 'card' }" :aria-pressed="view === 'card'" @click="setView('card')"><Icon name="grid" /> 卡片</button>
      </div>
    </div>

    <div v-if="view === 'list'" class="card monitor-board">
      <div class="monitor-scroll">
        <table class="monitor-fit">
          <colgroup>
            <col class="col-name" />
            <col class="col-group" />
            <col class="col-status" />
            <col class="col-result" />
            <col class="col-avail" />
            <col class="col-time" />
            <col class="col-actions" />
          </colgroup>
          <thead><tr><th>监控名称</th><th>分组</th><th>状态</th><th>响应时间</th><th>24h 可用率</th><th>最近检测</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="row in items" :key="row.id">
              <td><router-link class="name-cell" :to="'/monitors/' + row.id"><span class="name-ico"><Icon :name="iconOf(row)" /></span><span><b :title="row.name">{{ row.name }}</b><small :title="targetOf(row)">{{ targetOf(row) }}</small></span></router-link></td>
              <td :title="groupName(row)">{{ groupName(row) }}</td>
              <td><span class="status" :class="row.status"><i class="mark" :class="markOf(row.status)"></i>{{ statusLabel(row.status) }}</span></td>
              <td :title="resultText(row)">{{ resultText(row) }}</td>
              <td><span class="bar" :class="{ bad: row.status === 'DOWN' }"><i :style="{ width: (row.hasAvailability ? row.availability24h : 0) + '%' }"></i></span><span class="pct">{{ availabilityText(row) }}</span></td>
              <td>{{ relative(row.lastCheckAt) }}</td>
              <td class="actions-inline">
                <template v-if="can('monitor.write')">
                  <button @click="check(row)">检测</button>
                  <router-link class="link" :to="'/monitors/' + row.id + '/edit'">编辑</router-link>
                  <button class="danger" @click="remove(row)">删除</button>
                </template>
              </td>
            </tr>
            <tr v-if="!items.length"><td colspan="7" class="empty">还没有监控</td></tr>
          </tbody>
        </table>
      </div>
      <div class="pager">
        <button class="btn ghost" :disabled="page === 1" @click="page--; load()">上一页</button>
        <span class="demo-note">{{ total }} 项</span>
        <button class="btn ghost" :disabled="page * 20 >= total" @click="page++; load()">下一页</button>
      </div>
    </div>

    <div v-else class="monitor-board">
      <div class="monitor-scroll">
      <div v-if="items.length" class="monitor-grid">
        <article v-for="row in items" :key="row.id" class="card monitor-card" :class="row.status">
          <router-link class="monitor-card-head" :to="'/monitors/' + row.id">
            <span class="name-ico"><Icon :name="iconOf(row)" /></span>
            <span class="monitor-card-name">
              <b>{{ row.name }}</b>
              <small :title="targetOf(row)">{{ targetOf(row) }}</small>
            </span>
            <span class="status" :class="row.status"><i class="mark" :class="markOf(row.status)"></i>{{ statusLabel(row.status) }}</span>
          </router-link>
          <div class="monitor-metrics">
            <div><span>响应时间</span><b>{{ resultText(row) }}</b></div>
            <div><span>最近检测</span><b>{{ relative(row.lastCheckAt) }}</b></div>
            <div class="wide">
              <span>24h 可用率</span>
              <b>{{ availabilityText(row) }}</b>
              <span class="bar" :class="{ bad: row.status === 'DOWN' }"><i :style="{ width: (row.hasAvailability ? row.availability24h : 0) + '%' }"></i></span>
            </div>
          </div>
          <div class="monitor-card-foot">
            <span class="monitor-type">{{ typeText[row.type] || row.type || '监控' }} · {{ groupName(row) }}</span>
            <div class="actions-inline">
              <template v-if="can('monitor.write')">
                <button @click="check(row)">检测</button>
                <router-link class="link" :to="'/monitors/' + row.id + '/edit'">编辑</router-link>
                <button class="danger" @click="remove(row)">删除</button>
              </template>
            </div>
          </div>
        </article>
      </div>
      <div v-else class="card empty">还没有监控</div>
      </div>
      <div class="pager monitor-pager">
        <button class="btn ghost" :disabled="page === 1" @click="page--; load()">上一页</button>
        <span class="demo-note">{{ total }} 项</span>
        <button class="btn ghost" :disabled="page * 20 >= total" @click="page++; load()">下一页</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import Icon from '../components/Icon.vue'
import { api, download } from '../api'
import { can } from '../session'
import { demoGroups, presentDemo } from '../demo'
import { iconOf, isDemo, relative, statusLabel, targetOf, typeText } from '../format'

const VIEW_KEY = 'nmonitor_monitors_view'
const items = ref([])
const total = ref(0)
const page = ref(1)
const view = ref(localStorage.getItem(VIEW_KEY) === 'card' ? 'card' : 'list')
const query = reactive({ keyword: '', type: '', status: '', groupId: '' })
const groups = ref([])
const fileRef = ref()
const requestFileRef = ref()
const exporting = ref(false)
const importing = ref(false)
const notice = ref('')
const error = ref('')
const resourceTypes = ['cpu', 'memory', 'disk', 'script', 'process']

function setView(next) {
  view.value = next
  localStorage.setItem(VIEW_KEY, next)
}
function markOf(status) {
  if (status === 'DOWN') return 'red'
  if (status === 'UP') return 'green'
  return 'gray'
}
function resultText(row) {
  if (resourceTypes.includes(row.type)) return row.lastMessage || '-'
  return row.lastResponseTime ? row.lastResponseTime + ' ms' : '-'
}
function availabilityText(row) {
  return row.hasAvailability ? Number(row.availability24h).toFixed(2) + '%' : '-'
}
function groupName(row) {
  if (row.group?.name) return row.group.name
  const found = groups.value.find((item) => String(item.id) === String(row.groupId || ''))
  return found?.name || '未分组'
}
function search() {
  page.value = 1
  return load()
}

async function load() {
  if (isDemo()) {
    let list = presentDemo().overview
    const keyword = query.keyword.trim().toLowerCase()
    if (keyword) list = list.filter((row) => (row.name + ' ' + targetOf(row)).toLowerCase().includes(keyword))
    if (query.type) list = list.filter((row) => row.type === query.type)
    if (query.status) list = list.filter((row) => row.status === query.status)
    if (query.groupId) list = list.filter((row) => String(row.groupId || '') === query.groupId)
    items.value = list
    total.value = list.length
    return
  }
  const params = new URLSearchParams({ page: String(page.value), pageSize: '20' })
  for (const key of ['keyword', 'type', 'status', 'groupId']) if (query[key]) params.set(key, query[key])
  const data = await api('/api/monitors?' + params.toString())
  items.value = data.items || []
  total.value = data.total || 0
}
async function loadGroups() {
  if (isDemo()) {
    groups.value = demoGroups
    return
  }
  if (!can('group.read')) {
    groups.value = []
    return
  }
  try {
    groups.value = await api('/api/groups') || []
  } catch {
    groups.value = []
  }
}
async function check(row) {
  if (isDemo()) return
  await api('/api/monitors/' + row.id + '/check', { method: 'POST' })
  await load()
}
async function remove(row) {
  if (isDemo() || !confirm('删除监控「' + row.name + '」？')) return
  await api('/api/monitors/' + row.id, { method: 'DELETE' })
  await load()
}
function filterQuery() {
  const params = new URLSearchParams()
  for (const key of ['keyword', 'type', 'status', 'groupId']) if (query[key]) params.set(key, query[key])
  const qs = params.toString()
  return qs ? '?' + qs : ''
}
function datedName(prefix) {
  const now = new Date()
  const p = (n) => String(n).padStart(2, '0')
  return `${prefix}-${now.getFullYear()}${p(now.getMonth() + 1)}${p(now.getDate())}.xlsx`
}
function requestFilterQuery() {
  const params = new URLSearchParams()
  for (const key of ['keyword', 'status', 'groupId']) if (query[key]) params.set(key, query[key])
  const qs = params.toString()
  return qs ? '?' + qs : ''
}
async function exportExcel() {
  notice.value = ''
  error.value = ''
  if (isDemo()) { notice.value = '演示模式不会下载文件'; return }
  exporting.value = true
  try {
    await download('/api/monitors/export' + filterQuery(), datedName('nmonitor-monitors'))
    notice.value = '已导出当前筛选结果'
  } catch (err) {
    error.value = err.message
  } finally {
    exporting.value = false
  }
}
async function downloadTemplate() {
  notice.value = ''
  error.value = ''
  if (isDemo()) { notice.value = '演示模式不会下载文件'; return }
  exporting.value = true
  try {
    await download('/api/monitors/export?template=1', 'nmonitor-monitors-template.xlsx')
  } catch (err) {
    error.value = err.message
  } finally {
    exporting.value = false
  }
}
async function exportRequests() {
  notice.value = ''
  error.value = ''
  if (isDemo()) { notice.value = '演示模式不会下载文件'; return }
  exporting.value = true
  try {
    await download('/api/monitors/requests/export' + requestFilterQuery(), datedName('nmonitor-requests'))
    notice.value = '已导出网络请求'
  } catch (err) {
    error.value = err.message
  } finally {
    exporting.value = false
  }
}
async function downloadRequestTemplate() {
  notice.value = ''
  error.value = ''
  if (isDemo()) { notice.value = '演示模式不会下载文件'; return }
  exporting.value = true
  try {
    await download('/api/monitors/requests/export?template=1', 'nmonitor-requests-template.xlsx')
  } catch (err) {
    error.value = err.message
  } finally {
    exporting.value = false
  }
}
async function importRequests(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  notice.value = ''
  error.value = ''
  if (!file) return
  if (isDemo()) { notice.value = '演示模式不会写入服务器'; return }
  if (!confirm('按表格新建或更新网络请求？只改名称、分组、类型和地址，不会删除表格里没有的监控。')) return
  const body = new FormData()
  body.append('file', file)
  importing.value = true
  try {
    const data = await api('/api/monitors/requests/import', { method: 'POST', body })
    const totalRows = data.total || 0
    notice.value = totalRows
      ? `已导入 ${totalRows} 条网络请求：新建 ${data.created || 0}，更新 ${data.updated || 0}`
      : '表格里没有可导入的网络请求'
    page.value = 1
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    importing.value = false
  }
}
async function importExcel(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  notice.value = ''
  error.value = ''
  if (!file) return
  if (isDemo()) { notice.value = '演示模式不会写入服务器'; return }
  if (!confirm('按表格新建或更新监控？不会删除表格里没有的监控。从其他环境导入时，请先清空编号列。')) return
  const body = new FormData()
  body.append('file', file)
  importing.value = true
  try {
    const data = await api('/api/monitors/import', { method: 'POST', body })
    const totalRows = data.total || 0
    notice.value = totalRows
      ? `已导入 ${totalRows} 条：新建 ${data.created || 0}，更新 ${data.updated || 0}`
      : '表格里没有可导入的监控'
    page.value = 1
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    importing.value = false
  }
}
onMounted(() => { loadGroups(); load() })
</script>
