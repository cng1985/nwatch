<template>
  <div>
    <div class="page-head">
      <div>
        <h1>定时任务</h1>
        <p>按 cron 表达式在服务器上执行命令。时间使用系统配置里的时区，同一任务不会重叠执行。</p>
      </div>
      <button class="btn primary" @click="startCreate"><Icon name="plus" style="width:16px;height:16px" /> 新建任务</button>
    </div>

    <div v-if="formOpen" class="editor">
      <section class="card section">
        <h3><em>01</em> {{ form.id ? '编辑任务' : '新建任务' }}</h3>
        <div class="form-grid">
          <label class="field"><span class="req">名称</span><input v-model="form.name" class="text" placeholder="清理临时文件" /></label>
          <label class="field"><span>超时</span>
            <select v-model.number="form.timeout" class="select" style="width:100%">
              <option v-for="item in timeouts" :key="item[0]" :value="item[0]">{{ item[1] }}</option>
            </select>
          </label>
          <label class="field"><span>常用计划</span>
            <select class="select" style="width:100%" :value="presetValue" @change="applyPreset($event.target.value)">
              <option value="">自定义</option>
              <option v-for="item in presets" :key="item[0]" :value="item[0]">{{ item[1] }}</option>
            </select>
          </label>
          <label class="field"><span class="req">Cron 表达式</span><input v-model="form.cron" class="text" placeholder="0 2 * * *" /></label>
          <label class="field full"><span>说明</span><input v-model="form.description" class="text" placeholder="每天凌晨清理 7 天前的临时文件" /></label>
          <label class="field full"><span class="req">脚本内容</span><textarea v-model="form.command" class="text" rows="6" placeholder="find /tmp -type f -mtime +7 -delete"></textarea></label>
          <label class="field full"><span>工作目录</span><input v-model="form.workDir" class="text" placeholder="/tmp" /></label>
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
        <p class="demo-note" style="line-height:1.7">表达式是 5 段：分 时 日 月 周。日和周都写了具体值时，满足任意一个就会执行。命令通过 sh -c 执行，身份与 NMonitor 进程相同。</p>
      </aside>
    </div>

    <p v-if="notice" class="error">{{ notice }}</p>
    <div class="toolbar">
      <input v-model="keyword" class="search" placeholder="搜索名称、表达式或命令" @keyup.enter="load" />
      <button class="btn ghost" @click="load">查询</button>
    </div>
    <div class="card table-wrap">
      <table>
        <thead><tr><th>任务</th><th>计划</th><th>状态</th><th>下次执行</th><th>最近执行</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="row in items" :key="row.id" class="clickable" :class="{ 'row-on': selected && selected.id === row.id }" @click="openRuns(row)">
            <td><b>{{ row.name }}</b><small style="display:block;color:#60707b">{{ firstLine(row.command) }}</small></td>
            <td>{{ row.scheduleText || row.cron }}<small style="display:block;color:#60707b">{{ row.cron }}</small></td>
            <td><span class="status" :class="statusOf(row).cls">{{ statusOf(row).text }}</span></td>
            <td>{{ row.enabled ? zoneClock(row.nextRunAt) : '-' }}</td>
            <td>{{ relative(row.lastRunAt) }}</td>
            <td class="actions-inline" @click.stop>
              <button :disabled="runningId === row.id" @click="run(row)">{{ runningId === row.id ? '执行中' : '执行' }}</button>
              <button @click="edit(row)">编辑</button>
              <button @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</button>
              <button class="danger" @click="remove(row)">删除</button>
            </td>
          </tr>
          <tr v-if="!items.length"><td colspan="6" class="empty">还没有定时任务</td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="selected" class="card card-pad" style="margin-top:14px">
      <div class="card-title">
        <div>
          <h2>{{ selected.name }}</h2>
          <p>{{ selected.lastMessage || (runs.length ? '以下是执行记录' : '还没有执行记录') }}</p>
        </div>
        <button class="btn ghost" @click="openRuns(selected)">刷新记录</button>
      </div>
      <table>
        <thead><tr><th>时间</th><th>触发</th><th>结果</th><th>退出码</th><th>耗时</th></tr></thead>
        <tbody>
          <tr v-for="row in runs" :key="row.id" class="clickable" :class="{ 'row-on': output && output.id === row.id }" @click="output = row">
            <td>{{ zoneClock(row.startedAt) }}</td>
            <td>{{ row.trigger === 'manual' ? '手动' : '定时' }}</td>
            <td><span class="status" :class="row.success ? 'UP' : 'DOWN'">{{ row.success ? '成功' : '失败' }}</span></td>
            <td>{{ row.exitCode }}</td>
            <td>{{ row.durationMs }} ms</td>
          </tr>
          <tr v-if="!runs.length"><td colspan="5" class="empty">还没有执行记录</td></tr>
        </tbody>
      </table>
      <div v-if="output" style="margin-top:14px">
        <div class="log-box">{{ output.stdout || '（没有标准输出）' }}</div>
        <div v-if="output.stderr" class="log-box">{{ output.stderr }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import Icon from '../components/Icon.vue'
import { api } from '../api'
import { clock, isDemo, relative } from '../format'

const presets = [
  ['* * * * *', '每分钟'],
  ['*/5 * * * *', '每 5 分钟'],
  ['*/15 * * * *', '每 15 分钟'],
  ['0 * * * *', '每小时'],
  ['0 2 * * *', '每天 02:00'],
  ['0 9 * * 1', '每周一 09:00'],
  ['0 0 1 * *', '每月 1 日 00:00'],
]
const timeouts = [[10, '10 秒'], [30, '30 秒'], [60, '1 分钟'], [120, '2 分钟'], [300, '5 分钟'], [600, '10 分钟'], [1800, '30 分钟'], [3600, '1 小时']]

const timezone = ref('Asia/Shanghai')
const items = ref([])
const keyword = ref('')
const formOpen = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const preview = ref(null)
const selected = ref(null)
const runs = ref([])
const output = ref(null)
const runningId = ref(0)
const form = reactive(blank())
const presetValue = computed(() => presets.some((item) => item[0] === form.cron) ? form.cron : '')
let timer = 0
let previewTimer = 0

function blank() {
  return { id: 0, name: '', cron: '0 2 * * *', command: '', workDir: '', timeout: 60, description: '', enabled: true }
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
    { id: 1, name: '清理临时文件', cron: '0 2 * * *', scheduleText: '每天 02:00', command: 'find /tmp -type f -mtime +7 -delete', enabled: true, lastStatus: 'SUCCESS', lastRunAt: new Date(Date.now() - 3600 * 1000).toISOString(), nextRunAt: next, lastMessage: '执行成功', timeout: 60 },
    { id: 2, name: '数据库备份', cron: '0 3 * * *', scheduleText: '每天 03:00', command: 'echo backup', enabled: false, lastStatus: 'FAILED', lastRunAt: new Date(Date.now() - 86400 * 1000).toISOString(), nextRunAt: next, lastMessage: '退出码 1', timeout: 300 },
  ]
}

async function load() {
  try {
    if (isDemo()) {
      items.value = demoItems().filter((row) => !keyword.value || `${row.name} ${row.cron} ${row.command}`.includes(keyword.value))
      return
    }
    const data = await api('/api/cron-jobs?pageSize=50&keyword=' + encodeURIComponent(keyword.value))
    items.value = data.items || []
    if (selected.value) {
      const fresh = items.value.find((item) => item.id === selected.value.id)
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
  Object.assign(form, blank(), row, { id: row.id, enabled: !!row.enabled })
  error.value = ''
  formOpen.value = true
  refreshPreview()
}

function applyPreset(value) {
  if (value) form.cron = value
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
  saving.value = true
  try {
    const body = { name: form.name, cron: form.cron, command: form.command, workDir: form.workDir, timeout: form.timeout, description: form.description, enabled: form.enabled }
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
    await openRuns(row)
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
      runs.value = []
      output.value = null
    }
    await load()
  } catch (err) {
    notice.value = err.message
  }
}

async function openRuns(row) {
  selected.value = row
  if (isDemo()) {
    runs.value = [{ id: 1, startedAt: row.lastRunAt, trigger: 'schedule', success: row.lastStatus === 'SUCCESS', exitCode: row.lastStatus === 'SUCCESS' ? 0 : 1, durationMs: 80, stdout: row.lastStatus === 'SUCCESS' ? 'done' : '', stderr: '' }]
    return
  }
  const data = await api('/api/cron-jobs/' + row.id + '/runs?pageSize=20')
  runs.value = data.items || []
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
  timer = setInterval(load, 5000)
})
onUnmounted(() => clearInterval(timer))
</script>
