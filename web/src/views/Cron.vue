<template>
  <div>
    <div class="page-head">
      <div>
        <h1>定时任务</h1>
        <p>按 cron 表达式执行脚本，或发起 HTTP/HTTPS 请求。请求可以是 GET，也可以按 JSON、Form、FormData、XML、Text 发送 POST。时间使用系统配置里的时区，同一任务不会重叠执行。</p>
      </div>
      <div class="head-actions">
        <button class="btn ghost" @click="openAllRuns">执行记录</button>
        <button v-if="can('cron.write')" class="btn primary" @click="startCreate"><Icon name="plus" style="width:16px;height:16px" /> 新建任务</button>
      </div>
    </div>

    <div v-if="formOpen && can('cron.write')" class="editor">
      <section class="card section">
        <h3><em>01</em> {{ form.id ? '编辑任务' : '新建任务' }}</h3>
        <div class="form-grid">
          <label class="field"><span class="req">名称</span><input v-model="form.name" class="text" placeholder="清理临时文件" /></label>
          <label class="field"><span>超时</span>
            <select v-model.number="form.timeout" class="select" style="width:100%">
              <option v-for="item in timeouts" :key="item[0]" :value="item[0]">{{ item[1] }}</option>
            </select>
          </label>
          <div class="full"><CronExpr v-model="form.cron" /></div>
          <label class="field full"><span>说明</span><input v-model="form.description" class="text" :placeholder="action === 'script' ? '每天凌晨清理 7 天前的临时文件' : '每天同步一次状态'" /></label>
          <div class="types full">
            <button v-for="item in actions" :key="item.id" type="button" class="type-card" :class="{ on: action === item.id }" @click="setAction(item.id)">
              <Icon v-if="action === item.id" class="tick" name="check" />
              {{ item.label }}
            </button>
          </div>
          <template v-if="action === 'script'">
            <label class="field full"><span class="req">脚本内容</span><textarea v-model="form.command" class="text" rows="6" placeholder="find /tmp -type f -mtime +7 -delete"></textarea></label>
            <label class="field full"><span>工作目录</span><input v-model="form.workDir" class="text" placeholder="/tmp" /></label>
          </template>
          <template v-else>
            <div class="full">
              <HttpRequest
                ref="requestRef"
                v-model:url="form.url"
                v-model:method="form.method"
                v-model:body-type="form.bodyType"
                v-model:headers="form.headers"
                v-model:body="form.body"
                url-label="请求地址"
                placeholder="https://example.com/hook"
                :methods="['GET', 'POST']"
              />
            </div>
          </template>
          <label class="field check"><input v-model="form.enabled" type="checkbox" /> 启用</label>
        </div>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="head-actions" style="margin:4px 0 16px">
          <button class="btn primary" :disabled="saving" @click="save">{{ saving ? '保存中' : '保存' }}</button>
          <button class="btn ghost" @click="formOpen = false">取消</button>
        </div>
      </section>
      <aside class="card preview">
        <h2>下次执行</h2>
        <div class="kv"><span>计划</span><b>{{ preview?.text || '填写表达式后计算' }}</b></div>
        <div class="kv"><span>时区</span><b>{{ preview?.timezone || '系统时区' }}</b></div>
        <div v-for="(item, index) in preview?.next || []" :key="item" class="kv"><span>第 {{ index + 1 }} 次</span><b>{{ zoneClock(item) }}</b></div>
        <p class="demo-note" style="line-height:1.7">表达式是 5 段：分 时 日 月 周。日和周都写了具体值时，满足任意一个就会执行。{{ action === 'script' ? '脚本通过 sh -c 执行，身份与 NMonitor 进程相同。' : '请求从服务器发出。2xx 算成功，POST 不会跟随重定向。' }}</p>
      </aside>
    </div>

    <p v-if="notice" class="error">{{ notice }}</p>
    <div class="toolbar">
      <input v-model="keyword" class="search" placeholder="搜索名称、表达式或地址" @keyup.enter="load" />
      <button class="btn ghost" @click="load">查询</button>
    </div>
    <div class="card table-wrap">
      <table>
        <thead><tr><th>任务</th><th>计划</th><th>状态</th><th>下次执行</th><th>最近执行</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="row in items" :key="row.id" class="clickable" :class="{ 'row-on': selected && selected.id === row.id }" @click="pickJob(row)">
            <td><b>{{ row.name }}</b><small style="display:block;color:#60707b">{{ targetOf(row) }}</small></td>
            <td>{{ row.scheduleText || row.cron }}<small style="display:block;color:#60707b">{{ row.cron }}</small></td>
            <td><span class="status" :class="statusOf(row).cls">{{ statusOf(row).text }}</span></td>
            <td>{{ row.enabled ? zoneClock(row.nextRunAt) : '-' }}</td>
            <td>{{ relative(row.lastRunAt) }}</td>
            <td class="actions-inline" @click.stop>
              <template v-if="can('cron.write')">
                <button :disabled="runningId === row.id" @click="run(row)">{{ runningId === row.id ? '执行中' : '执行' }}</button>
                <button @click="edit(row)">编辑</button>
                <button @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</button>
                <button class="danger" @click="remove(row)">删除</button>
              </template>
            </td>
          </tr>
          <tr v-if="!items.length"><td colspan="6" class="empty">还没有定时任务</td></tr>
        </tbody>
      </table>
    </div>

    <Teleport to="body">
      <div v-if="runsOpen" class="drawer-mask" @click.self="closeRuns">
        <aside class="drawer" role="dialog" aria-modal="true" aria-label="执行记录">
          <div class="drawer-head">
            <div>
              <h2>执行记录</h2>
              <p>{{ output ? runSummary(output) : (selected ? selected.name + ' 的执行记录' : '按任务、结果、触发方式和关键字查询。日期按系统时区，结束日期当天包含在内。') }}</p>
            </div>
            <button class="drawer-close" type="button" aria-label="关闭" @click="closeRuns"><Icon name="close" /></button>
          </div>
          <div class="drawer-body">
            <div class="drawer-filters">
              <select v-model="runQuery.jobId" class="select">
                <option value="">全部任务</option>
                <option v-for="row in jobOptions" :key="row.id" :value="String(row.id)">{{ row.name }}</option>
              </select>
              <select v-model="runQuery.success" class="select">
                <option value="">全部结果</option>
                <option value="true">成功</option>
                <option value="false">失败</option>
              </select>
              <select v-model="runQuery.trigger" class="select">
                <option value="">全部触发</option>
                <option value="schedule">定时</option>
                <option value="manual">手动</option>
              </select>
              <label class="date-field">从<input v-model="runQuery.from" class="text date" type="date" /></label>
              <label class="date-field">到<input v-model="runQuery.to" class="text date" type="date" /></label>
              <input v-model="runQuery.keyword" class="search" placeholder="搜索名称、命令或输出" @keyup.enter="searchRuns" />
              <div class="drawer-actions">
                <button class="btn ghost" @click="searchRuns">查询</button>
                <button class="btn ghost" @click="loadRuns">刷新</button>
              </div>
            </div>
            <table>
              <thead><tr><th>时间</th><th>任务</th><th>触发</th><th>结果</th><th>{{ resultCodeLabel }}</th><th>耗时</th></tr></thead>
              <tbody>
                <tr v-for="row in runs" :key="row.id" class="clickable" :class="{ 'row-on': output && output.id === row.id }" @click="output = row">
                  <td>{{ zoneClock(row.startedAt) }}</td>
                  <td><b>{{ row.name }}</b><small style="display:block;color:#60707b">{{ firstLine(row.command) }}</small></td>
                  <td>{{ row.trigger === 'manual' ? '手动' : '定时' }}</td>
                  <td><span class="status" :class="row.success ? 'UP' : 'DOWN'">{{ row.success ? '成功' : '失败' }}</span></td>
                  <td>{{ row.exitCode }}</td>
                  <td>{{ row.durationMs }} ms</td>
                </tr>
                <tr v-if="!runs.length"><td colspan="6" class="empty">没有符合条件的执行记录</td></tr>
              </tbody>
            </table>
            <div class="pager">
              <button class="btn ghost" :disabled="runPage === 1" @click="turnRuns(-1)">上一页</button>
              <span class="demo-note">{{ runTotal }} 条</span>
              <button class="btn ghost" :disabled="runPage * runSize >= runTotal" @click="turnRuns(1)">下一页</button>
            </div>
            <div v-if="output" class="drawer-output">
              <div class="log-box">{{ output.stdout || '（没有输出）' }}</div>
              <div v-if="output.stderr" class="log-box">{{ output.stderr }}</div>
            </div>
          </div>
        </aside>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import CronExpr from '../components/CronExpr.vue'
import HttpRequest from '../components/HttpRequest.vue'
import Icon from '../components/Icon.vue'
import { requestLabel } from '../components/httpRequest.js'
import { api } from '../api'
import { can } from '../session'
import { clock, isDemo, relative } from '../format'

const timeouts = [[10, '10 秒'], [30, '30 秒'], [60, '1 分钟'], [120, '2 分钟'], [300, '5 分钟'], [600, '10 分钟'], [1800, '30 分钟'], [3600, '1 小时']]
const actions = [
  { id: 'script', label: '脚本' },
  { id: 'http', label: 'HTTP' },
]

const timezone = ref('Asia/Shanghai')
const items = ref([])
const jobOptions = ref([])
const keyword = ref('')
const formOpen = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const preview = ref(null)
const selected = ref(null)
const runsOpen = ref(false)
const runs = ref([])
const output = ref(null)
const runPage = ref(1)
const runTotal = ref(0)
const runSize = 20
const runQuery = reactive({ jobId: '', success: '', trigger: '', from: '', to: '', keyword: '' })
const runFilter = reactive({ jobId: '', success: '', trigger: '', from: '', to: '', keyword: '' })
const resultCodeLabel = computed(() => {
  if (!runFilter.jobId) return '结果码'
  const job = jobOptions.value.find((row) => String(row.id) === String(runFilter.jobId)) || selected.value
  return job && job.kind === 'http' ? '状态码' : '退出码'
})
const runningId = ref(0)
const requestRef = ref(null)
const form = reactive(blank())
const action = computed(() => (form.kind === 'http' ? 'http' : 'script'))
let timer = 0
let previewTimer = 0

function blank() {
  return {
    id: 0, name: '', cron: '0 2 * * *', kind: 'script', command: '', workDir: '',
    url: '', method: '', bodyType: '', headers: {}, body: '', timeout: 60, description: '', enabled: true,
  }
}

function zoneClock(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  try {
    const parts = new Intl.DateTimeFormat('en-GB', {
      timeZone: timezone.value || 'UTC',
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
      hourCycle: 'h23',
    }).formatToParts(date)
    const pick = (type) => parts.find((part) => part.type === type)?.value || '00'
    return `${pick('year')}-${pick('month')}-${pick('day')} ${pick('hour')}:${pick('minute')}:${pick('second')}`
  } catch {
    return clock(value)
  }
}

function firstLine(value) {
  return (value || '').split('\n').map((line) => line.trim()).find(Boolean) || '-'
}

function targetOf(row) {
  if (row.targetText) return row.targetText
  if (row.kind === 'http') return `${requestLabel(row.method, row.bodyType)} ${row.url || ''}`.trim()
  return firstLine(row.command)
}

function setAction(next) {
  if (next === 'script') {
    form.kind = 'script'
    form.method = ''
    form.bodyType = ''
    form.body = ''
    form.url = ''
    form.headers = {}
    return
  }
  form.kind = 'http'
  if (!form.method) form.method = 'GET'
}

function runSummary(row) {
  if (row.errorMessage) return row.errorMessage
  return row.success ? '执行成功' : '执行失败'
}

function statusOf(row) {
  if (!row.enabled) return { cls: 'PAUSED', text: '已停用' }
  return {
    SUCCESS: { cls: 'UP', text: '成功' },
    FAILED: { cls: 'DOWN', text: '失败' },
    TIMEOUT: { cls: 'DOWN', text: '超时' },
    RUNNING: { cls: 'UNKNOWN', text: '执行中' },
  }[row.lastStatus] || { cls: 'UNKNOWN', text: '尚未执行' }
}

function demoItems() {
  const next = new Date(Date.now() + 3600 * 1000).toISOString()
  return [
    { id: 1, name: '清理临时文件', cron: '0 2 * * *', kind: 'script', scheduleText: '每天 02:00', command: 'find /tmp -type f -mtime +7 -delete', enabled: true, lastStatus: 'SUCCESS', lastRunAt: new Date(Date.now() - 3600 * 1000).toISOString(), nextRunAt: next, lastMessage: '执行成功', timeout: 60 },
    { id: 2, name: '同步状态', cron: '0 3 * * *', kind: 'http', method: 'POST', bodyType: 'json', url: 'https://example.com/hook', scheduleText: '每天 03:00', targetText: 'POST JSON https://example.com/hook', enabled: false, lastStatus: 'FAILED', lastRunAt: new Date(Date.now() - 86400 * 1000).toISOString(), nextRunAt: next, lastMessage: 'HTTP 502', timeout: 30 },
  ]
}

async function load() {
  try {
    if (isDemo()) {
      jobOptions.value = demoItems()
      items.value = jobOptions.value.filter((row) => !keyword.value || `${row.name} ${row.cron} ${row.command || ''} ${row.url || ''}`.includes(keyword.value))
      return
    }
    const [listed, options] = await Promise.all([
      api('/api/cron-jobs?pageSize=50&keyword=' + encodeURIComponent(keyword.value)),
      api('/api/cron-jobs?pageSize=200'),
    ])
    items.value = listed.items || []
    jobOptions.value = options.items || []
    if (selected.value) {
      const fresh = jobOptions.value.find((item) => item.id === selected.value.id)
      if (fresh) selected.value = fresh
    }
  } catch (err) {
    notice.value = err.message
  }
}

function startCreate() {
  Object.assign(form, blank())
  error.value = ''
  formOpen.value = true
  refreshPreview()
}

function edit(row) {
  Object.assign(form, blank(), row, { id: row.id, enabled: !!row.enabled, headers: { ...(row.headers || {}) }, body: row.body || '', bodyType: row.bodyType || '', kind: row.kind || 'script' })
  error.value = ''
  formOpen.value = true
  refreshPreview()
}

async function refreshPreview() {
  const expr = form.cron.trim()
  if (!expr) {
    preview.value = null
    return
  }
  if (isDemo()) {
    preview.value = { text: '每天 02:00', timezone: 'Asia/Shanghai', next: [new Date(Date.now() + 3600 * 1000).toISOString()] }
    return
  }
  try {
    preview.value = await api('/api/cron/preview?count=5&expr=' + encodeURIComponent(expr))
  } catch {
    preview.value = null
  }
}

async function save() {
  error.value = ''
  if (isDemo()) {
    error.value = '演示模式不会写入服务器'
    return
  }
  if (form.kind === 'http' && requestRef.value) {
    const message = requestRef.value.validate()
    if (message) {
      error.value = message
      return
    }
  }
  saving.value = true
  try {
    const body = {
      name: form.name, cron: form.cron, kind: form.kind || 'script', command: form.command, workDir: form.workDir,
      url: form.url, method: form.method, bodyType: form.kind === 'http' ? form.bodyType : '',
      headers: form.kind === 'http' ? (form.headers || {}) : {},
      body: form.kind === 'http' ? form.body : '',
      timeout: form.timeout, description: form.description, enabled: form.enabled,
    }
    if (form.id) await api('/api/cron-jobs/' + form.id, { method: 'PUT', body })
    else await api('/api/cron-jobs', { method: 'POST', body })
    formOpen.value = false
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

async function run(row) {
  if (isDemo()) {
    notice.value = '演示模式不会执行任务'
    return
  }
  notice.value = ''
  runningId.value = row.id
  try {
    output.value = await api('/api/cron-jobs/' + row.id + '/run', { method: 'POST' })
    selected.value = row
    runQuery.jobId = String(row.id)
    runsOpen.value = true
    await searchRuns()
    await load()
  } catch (err) {
    notice.value = err.message
  } finally {
    runningId.value = 0
  }
}

async function toggle(row) {
  if (isDemo()) return
  notice.value = ''
  try {
    await api('/api/cron-jobs/' + row.id + (row.enabled ? '/disable' : '/enable'), { method: 'POST' })
    await load()
  } catch (err) {
    notice.value = err.message
  }
}

async function remove(row) {
  if (isDemo() || !confirm('删除定时任务「' + row.name + '」？执行记录会一并删除。')) return
  notice.value = ''
  try {
    await api('/api/cron-jobs/' + row.id, { method: 'DELETE' })
    if (selected.value && selected.value.id === row.id) {
      selected.value = null
      output.value = null
    }
    if (runQuery.jobId === String(row.id)) runQuery.jobId = ''
    if (runFilter.jobId === String(row.id)) runFilter.jobId = ''
    await load()
    await loadRuns()
  } catch (err) {
    notice.value = err.message
  }
}

function demoRuns() {
  return [
    { id: 1, jobId: 1, name: '清理临时文件', command: 'find /tmp -type f -mtime +7 -delete', trigger: 'schedule', success: true, exitCode: 0, durationMs: 80, startedAt: new Date(Date.now() - 3600 * 1000).toISOString(), stdout: 'done', stderr: '', errorMessage: '' },
    { id: 2, jobId: 2, name: '同步状态', command: 'POST JSON https://example.com/hook', trigger: 'manual', success: false, exitCode: 502, durationMs: 40, startedAt: new Date(Date.now() - 86400 * 1000).toISOString(), stdout: 'bad gateway', stderr: '', errorMessage: 'HTTP 502' },
  ]
}

function openAllRuns() {
  selected.value = null
  runQuery.jobId = ''
  runsOpen.value = true
  return searchRuns()
}

function closeRuns() {
  runsOpen.value = false
}

function onKey(event) {
  if (event.key === 'Escape' && runsOpen.value) closeRuns()
}

function pickJob(row) {
  selected.value = row
  runQuery.jobId = String(row.id)
  runsOpen.value = true
  return searchRuns()
}

function searchRuns() {
  Object.assign(runFilter, runQuery)
  runPage.value = 1
  selected.value = jobOptions.value.find((row) => String(row.id) === String(runQuery.jobId)) || selected.value
  if (!runQuery.jobId) selected.value = null
  return loadRuns()
}

function turnRuns(delta) {
  const next = runPage.value + delta
  if (next < 1 || (next - 1) * runSize >= runTotal.value) return
  runPage.value = next
  return loadRuns()
}

async function loadRuns() {
  try {
    if (isDemo()) {
      let rows = demoRuns()
      if (runFilter.jobId) rows = rows.filter((row) => String(row.jobId) === String(runFilter.jobId))
      if (runFilter.success) rows = rows.filter((row) => String(row.success) === runFilter.success)
      if (runFilter.trigger) rows = rows.filter((row) => row.trigger === runFilter.trigger)
      const kw = runFilter.keyword.trim()
      if (kw) rows = rows.filter((row) => `${row.name} ${row.command} ${row.stdout} ${row.stderr} ${row.errorMessage || ''}`.includes(kw))
      if (runFilter.from) rows = rows.filter((row) => String(row.startedAt).slice(0, 10) >= runFilter.from)
      if (runFilter.to) rows = rows.filter((row) => String(row.startedAt).slice(0, 10) <= runFilter.to)
      runTotal.value = rows.length
      runs.value = rows
      if (output.value && !runs.value.some((row) => row.id === output.value.id)) output.value = null
      return
    }
    const params = new URLSearchParams({ page: String(runPage.value), pageSize: String(runSize) })
    for (const key of ['jobId', 'success', 'trigger', 'from', 'to', 'keyword']) {
      const value = String(runFilter[key] || '').trim()
      if (value) params.set(key, value)
    }
    const data = await api('/api/cron-runs?' + params.toString())
    runs.value = data.items || []
    runTotal.value = data.total || 0
    if (output.value && !runs.value.some((row) => row.id === output.value.id)) output.value = null
  } catch (err) {
    notice.value = err.message
  }
}

watch(() => form.cron, () => {
  clearTimeout(previewTimer)
  previewTimer = setTimeout(refreshPreview, 250)
})

onMounted(async () => {
  if (!isDemo()) {
    try {
      const settings = await api('/api/settings')
      if (settings.timezone) timezone.value = settings.timezone
    } catch { /* 列表仍可用，时间先按默认时区显示 */ }
  }
  await load()
  window.addEventListener('keydown', onKey)
  timer = setInterval(() => { load(); if (runsOpen.value) loadRuns() }, 5000)
})
onUnmounted(() => {
  clearInterval(timer)
  window.removeEventListener('keydown', onKey)
  document.body.style.overflow = ''
})

watch(runsOpen, (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
})
</script>
