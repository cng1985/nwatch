<template>
  <div>
    <div class="page-head">
      <div>
        <h1>{{ editing ? '编辑监控' : '新增监控' }}</h1>
        <p>为您的服务配置健康检查与异常通知。</p>
      </div>
      <div class="head-actions">
        <button class="btn ghost" @click="cancel">取消</button>
        <button class="btn primary" :disabled="saving" @click="save">{{ editing ? '保存修改' : '创建监控' }}</button>
      </div>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="editor">
      <div>
        <section class="card section">
          <h3><em>01</em> 基本信息</h3>
          <div class="form-grid">
            <label class="field"><span class="req">监控名称</span><input v-model="form.name" class="text" placeholder="ERP API" /></label>
            <label class="field"><span class="req">监控分组</span>
              <select v-model="form.groupId" class="select" style="width:100%">
                <option :value="null">请选择分组</option>
                <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
              </select>
            </label>
            <div class="full">
              <span class="req" style="display:block;margin-bottom:8px;font-weight:600">监控类型</span>
              <div class="types">
                <button v-for="item in kinds" :key="item.id" type="button" class="type-card" :class="{ on: form.kind === item.id }" @click="form.kind = item.id">
                  <Icon v-if="form.kind === item.id" class="tick" name="check" />
                  <Icon :name="item.icon" />
                  {{ item.label }}
                </button>
              </div>
            </div>
          </div>
        </section>
        <section class="card section">
          <h3><em>02</em> 检测配置</h3>
          <div class="form-grid">
            <div v-if="form.kind === 'http' || form.kind === 'https'" class="full">
              <HttpRequest
                ref="requestRef"
                v-model:url="form.url"
                v-model:method="form.method"
                v-model:body-type="form.bodyType"
                v-model:headers="form.headers"
                v-model:body="form.body"
                url-label="目标 URL"
                :placeholder="form.kind === 'https' ? 'https://erp.example.com/health' : 'http://erp.example.com/health'"
                :methods="['GET', 'POST', 'HEAD']"
              />
            </div>
            <label v-else-if="form.kind === 'tls'" class="field full"><span class="req">目标 URL</span>
              <input v-model="form.url" class="text" placeholder="api.example.com" />
            </label>
            <template v-else-if="form.kind === 'tcp'">
              <label class="field"><span class="req">主机</span><input v-model="form.host" class="text" placeholder="192.168.1.100" /></label>
              <label class="field"><span class="req">端口</span><input v-model.number="form.port" class="text" type="number" /></label>
            </template>
            <template v-else-if="form.kind === 'cpu' || form.kind === 'memory'">
              <label class="field"><span class="req">使用率阈值</span><div class="unit"><input v-model.number="form.threshold" class="text" type="number" min="1" max="100" /><em>%</em></div></label>
              <p class="full demo-note">使用率超过阈值时记为一次失败，连续失败达到下方次数后告警。</p>
            </template>
            <template v-else-if="form.kind === 'disk'">
              <label class="field"><span class="req">磁盘路径</span><input v-model="form.host" class="text" :placeholder="'/ 或 C:\\'" /></label>
              <label class="field"><span class="req">使用率阈值</span><div class="unit"><input v-model.number="form.threshold" class="text" type="number" min="1" max="100" /><em>%</em></div></label>
              <p class="full demo-note">按路径所在文件系统计算用量。Linux 根分区填 /，Windows 可填 C:\。留空时使用系统默认盘。</p>
            </template>
            <template v-else-if="form.kind === 'process'">
              <div class="full proc-picker">
                <div v-if="form.host" class="proc-selected">
                  <div>
                    <b>{{ form.host }}</b>
                    <small>{{ form.match === 'command' && form.command ? form.command : '只按进程名匹配，进程重启后仍可识别' }}</small>
                  </div>
                  <button type="button" class="link" @click="clearProcess">清除</button>
                </div>
                <div class="proc-tools">
                  <button type="button" class="btn ghost" @click="openPicker">选择进程</button>
                  <span class="demo-note" style="margin:0">从当前系统进程中选择。PID 会变化，停止和重启都按进程名识别。</span>
                </div>
              </div>
              <label class="field"><span class="req">进程名</span><input v-model="form.host" class="text" placeholder="nginx" /></label>
              <label class="field"><span>匹配方式</span>
                <select v-model="form.match" class="select" style="width:100%">
                  <option value="name">只按进程名</option>
                  <option value="command">进程名和命令行</option>
                </select>
              </label>
              <label v-if="form.match === 'command'" class="field full"><span class="req">命令行包含</span><input v-model="form.command" class="text" placeholder="nginx: master process" /></label>
              <label class="field"><span>CPU 阈值</span><div class="unit"><input v-model.number="form.threshold" class="text" type="number" min="0" max="10000" step="0.1" /><em>%</em></div></label>
              <label class="field"><span>内存阈值</span><div class="unit"><input v-model.number="form.memThreshold" class="text" type="number" min="0" step="1" /><em>MB</em></div></label>
              <p class="full demo-note">进程未运行时记为失败，连续失败达到下方次数后通知。CPU 按单个核心计算，可以超过 100%。多个同名进程时，CPU 取最高的一个，内存取合计。填 0 表示不按该项告警。</p>
            </template>
            <template v-else-if="form.kind === 'script'">
              <label class="field full"><span class="req">脚本内容</span><textarea v-model="form.command" class="text" rows="6" placeholder="systemctl is-active nginx"></textarea></label>
              <label class="field"><span>工作目录</span><input v-model="form.workDir" class="text" placeholder="/opt/nmonitor" /></label>
              <label class="field"><span>输出包含</span><input v-model="form.bodyContains" class="text" placeholder="active" /></label>
              <p class="full demo-note">脚本在服务器上以 NMonitor 进程用户执行。退出码 0 视为成功。</p>
            </template>
            <label v-if="form.kind === 'http' || form.kind === 'https'" class="field"><span class="req">期望状态码</span>
              <input v-model="form.expectedStatusCodes" class="text" placeholder="200" />
            </label>
            <label class="field"><span class="req">检测频率</span>
              <select v-model.number="form.interval" class="select" style="width:100%">
                <option v-for="item in intervals" :key="item[0]" :value="item[0]">{{ item[1] }}</option>
              </select>
            </label>
            <label class="field"><span class="req">超时时间</span>
              <select v-model.number="form.timeout" class="select" style="width:100%">
                <option :value="3">3 秒</option><option :value="5">5 秒</option><option :value="10">10 秒</option><option :value="15">15 秒</option><option :value="30">30 秒</option>
              </select>
            </label>
            <button class="fold full" type="button" @click="advanced = !advanced">{{ advanced ? '▾' : '▸' }} 高级检测配置</button>
            <template v-if="advanced">
              <label v-if="form.kind === 'http' || form.kind === 'https'" class="field"><span>响应包含</span><input v-model="form.bodyContains" class="text" placeholder="UP" /></label>
              <label v-if="form.kind === 'http' || form.kind === 'https'" class="field"><span>响应不含</span><input v-model="form.bodyNotContains" class="text" /></label>
              <label v-if="form.kind === 'http' || form.kind === 'https'" class="field full check"><input v-model="form.followRedirects" type="checkbox" /> 跟随重定向</label>
              <label v-if="form.kind === 'tls'" class="field"><span>预警天数</span><input v-model.number="form.tlsWarningDays" class="text" type="number" /></label>
              <label v-if="form.kind === 'tls'" class="field"><span>严重天数</span><input v-model.number="form.tlsCriticalDays" class="text" type="number" /></label>
              <label v-if="form.kind === 'tls'" class="field full"><span>通知阈值</span><input v-model="form.tlsNotifyDays" class="text" placeholder="30,14,7,3,1,0" /></label>
            </template>
          </div>
        </section>
        <section class="card section">
          <h3><em>03</em> 状态与通知</h3>
          <div class="form-grid">
            <label class="field"><span class="req">连续失败次数</span><div class="unit"><input v-model.number="form.failureThreshold" class="text" type="number" min="1" /><em>次</em></div></label>
            <label class="field"><span class="req">连续恢复次数</span><div class="unit"><input v-model.number="form.recoveryThreshold" class="text" type="number" min="1" /><em>次</em></div></label>
            <p class="full demo-note" style="margin-top:-6px">连续失败达到阈值后触发告警，避免短暂波动造成误报。</p>
            <p class="full demo-note" style="margin-top:-6px">告警会发到所有已启用的通知渠道，包括钉钉、企业微信、Webhook，以及系统配置里启用的邮件。关闭的渠道不发送。</p>
          </div>
        </section>
      </div>
      <aside class="card preview">
        <h2>配置预览</h2>
        <div class="preview-target">
          <span class="name-ico brand"><Icon :name="currentKind.icon" /></span>
          <div><b>{{ form.name || '未命名监控' }}</b><small style="color:#7e8c98">{{ previewTarget }}</small></div>
        </div>
        <div class="kv"><span>监控类型</span><b>{{ currentKind.label }}</b></div>
        <div v-if="form.kind === 'http' || form.kind === 'https'" class="kv"><span>请求</span><b>{{ requestPreview }}</b></div>
        <div class="kv"><span>检测频率</span><b>每 {{ intervalLabel }}</b></div>
        <div class="kv"><span>超时时间</span><b>{{ form.timeout }} 秒</b></div>
        <div class="kv"><span>异常判定</span><b>连续失败 {{ form.failureThreshold }} 次</b></div>
        <div class="kv"><span>恢复判定</span><b>连续成功 {{ form.recoveryThreshold }} 次</b></div>
        <div class="kv"><span>通知渠道</span><b>全部已启用</b></div>
        <div v-if="form.kind === 'process'" class="kv"><span>CPU / 内存</span><b>{{ processLimitText }}</b></div>
        <div class="tip">
          <b><Icon name="bulb" style="width:16px;height:16px;color:#12b981" /> 配置小贴士</b>
          {{ form.kind === 'process' ? '进程停止后按连续失败次数通知。想在进程一停止就提醒，把连续失败次数设为 1。创建后可以立即检测一次。' : '建议使用专用的 /health 接口作为监控目标。在创建后，您可以立即执行一次检测，确认服务配置。' }}
          <div v-if="demoMode" class="demo-note">演示数据，用于页面设计预览。</div>
        </div>
      </aside>
    </div>
    <div v-if="procOpen" class="modal-mask" @click.self="procOpen = false">
      <div class="modal wide">
        <h3>选择进程</h3>
        <div class="proc-tools">
          <input v-model="procQuery" class="search" placeholder="搜索进程名、命令或 PID" />
          <button type="button" class="btn ghost" @click="loadProcesses">刷新</button>
        </div>
        <p v-if="procError" class="error">{{ procError }}</p>
        <div class="proc-scroll">
          <table>
            <thead><tr><th>进程</th><th>PID</th><th>用户</th><th>CPU</th><th>内存</th><th>状态</th></tr></thead>
            <tbody>
              <tr v-for="row in filteredProcs" :key="row.pid" :class="{ on: row.name === form.host && row.command === form.command }" @click="pickProcess(row)">
                <td><div class="proc-cmd"><b>{{ row.name || '-' }}</b><small v-if="row.command && row.command !== row.name">{{ row.command }}</small></div></td>
                <td class="pct">{{ row.pid }}</td>
                <td>{{ row.user || '-' }}</td>
                <td class="pct">{{ Number(row.cpuPercent || 0).toFixed(1) }}%</td>
                <td class="pct">{{ bytes(row.memBytes) }}</td>
                <td>{{ row.stateText || row.state || '-' }}</td>
              </tr>
              <tr v-if="!filteredProcs.length"><td colspan="6" class="empty">{{ procLoading ? '读取中' : (procError || '没有匹配的进程') }}</td></tr>
            </tbody>
          </table>
        </div>
        <div class="head-actions" style="margin-top:12px;justify-content:flex-end">
          <button type="button" class="btn ghost" @click="procOpen = false">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import HttpRequest from '../components/HttpRequest.vue'
import Icon from '../components/Icon.vue'
import { requestLabel } from '../components/httpRequest.js'
import { api } from '../api'
import { demoGroups, presentDemo } from '../demo'
import { bytes, isDemo } from '../format'

const route = useRoute()
const router = useRouter()
const editing = computed(() => !!route.params.id)
const demoMode = isDemo()
const groups = ref([])
const saving = ref(false)
const error = ref('')
const advanced = ref(false)
const requestRef = ref(null)
const kinds = [
  { id: 'http', label: 'HTTP', icon: 'globe' },
  { id: 'https', label: 'HTTPS', icon: 'lock' },
  { id: 'tcp', label: 'TCP 端口', icon: 'db' },
  { id: 'tls', label: 'HTTPS 证书', icon: 'file' },
  { id: 'cpu', label: 'CPU', icon: 'cpu' },
  { id: 'memory', label: '内存', icon: 'mem' },
  { id: 'disk', label: '磁盘', icon: 'disk' },
  { id: 'script', label: '脚本', icon: 'script' },
  { id: 'process', label: '进程', icon: 'process' },
]
const demoProcesses = [
  { pid: 1842, user: 'www', name: 'java', command: 'java -jar erp.jar', state: 'S', stateText: '睡眠', cpuPercent: 86.4, memBytes: 1.8e9 },
  { pid: 922, user: 'root', name: 'dockerd', command: 'dockerd --host=unix:///var/run/docker.sock', state: 'S', stateText: '睡眠', cpuPercent: 12.1, memBytes: 420e6 },
  { pid: 4102, user: 'postgres', name: 'postgres', command: 'postgres: checkpointer', state: 'S', stateText: '睡眠', cpuPercent: 3.4, memBytes: 960e6 },
]
const intervals = [[15, '15 秒'], [30, '30 秒'], [60, '60 秒'], [300, '5 分钟'], [600, '10 分钟'], [1800, '30 分钟'], [3600, '1 小时'], [43200, '12 小时'], [86400, '24 小时']]
const form = reactive({
  name: '', groupId: null, kind: 'https', url: '', host: '', port: 443, method: 'GET',
  expectedStatusCodes: '200', interval: 60, timeout: 5, failureThreshold: 3, recoveryThreshold: 1,
  bodyContains: '', bodyNotContains: '', followRedirects: true, tlsWarningDays: 14, tlsCriticalDays: 7,
  headers: {}, body: '', bodyType: '',
  tlsNotifyDays: '30,14,7,3,1,0', notifierIds: [], enabled: true,
  threshold: 90, memThreshold: 0, match: 'name', command: '', workDir: '',
})
const procOpen = ref(false)
const procLoading = ref(false)
const procError = ref('')
const procQuery = ref('')
const processes = ref([])
const filteredProcs = computed(() => {
  const q = procQuery.value.trim().toLowerCase()
  if (!q) return processes.value
  return processes.value.filter((row) => `${row.pid} ${row.user || ''} ${row.name || ''} ${row.command || ''}`.toLowerCase().includes(q))
})
const currentKind = computed(() => kinds.find((item) => item.id === form.kind))
const intervalLabel = computed(() => intervals.find((item) => item[0] === form.interval)?.[1] || form.interval + ' 秒')
const requestPreview = computed(() => requestLabel(form.method, form.bodyType))
const previewTarget = computed(() => {
  if (form.kind === 'tcp') return form.host ? `${form.host}:${form.port || ''}` : '未填写地址'
  if (form.kind === 'cpu') return `CPU 超过 ${form.threshold || 90}%`
  if (form.kind === 'memory') return `内存超过 ${form.threshold || 90}%`
  if (form.kind === 'disk') return `${form.host || '默认磁盘'} 超过 ${form.threshold || 90}%`
  if (form.kind === 'script') return (form.command || '').split('\n')[0] || '未填写脚本'
  if (form.kind === 'process') {
    const name = form.host || '未选择进程'
    return form.match === 'command' && form.command ? `${name} · ${form.command}` : name
  }
  return form.url || '未填写地址'
})
const processLimitText = computed(() => {
  const cpu = Number(form.threshold) > 0 ? `CPU ${form.threshold}%` : 'CPU 不限制'
  const mem = Number(form.memThreshold) > 0 ? `内存 ${form.memThreshold} MB` : '内存不限制'
  return `${cpu}，${mem}`
})

function cancel() { router.push('/monitors') }

function clearProcess() {
  form.host = ''
  form.command = ''
  form.match = 'name'
}

async function loadProcesses() {
  procLoading.value = true
  procError.value = ''
  try {
    if (demoMode) {
      processes.value = demoProcesses
      return
    }
    const data = await api('/api/monitors/processes?sort=cpu&limit=300')
    processes.value = data.items || []
  } catch (err) {
    procError.value = err.message
    processes.value = []
  } finally {
    procLoading.value = false
  }
}

function openPicker() {
  procOpen.value = true
  procQuery.value = ''
  loadProcesses()
}

function pickProcess(row) {
  form.host = row.name || ''
  form.command = row.command || ''
  form.match = form.command && form.command !== form.host ? 'command' : 'name'
  if (!form.name) form.name = form.host || '进程监控'
  procOpen.value = false
}

function applyProcessQuery() {
  if (editing.value || route.query.type !== 'process') return
  form.kind = 'process'
  form.host = String(route.query.name || '')
  form.command = String(route.query.command || '')
  form.match = form.command && form.command !== form.host ? 'command' : 'name'
  if (!form.name || form.name === 'ERP API') form.name = form.host || form.name
}

async function save() {
  error.value = ''
  if (demoMode) { error.value = '演示模式不会写入服务器'; return }
  const httpKind = form.kind === 'http' || form.kind === 'https'
  if (httpKind && requestRef.value) {
    const message = requestRef.value.validate()
    if (message) { error.value = message; return }
  }
  if (form.kind === 'process' && !String(form.host || '').trim()) {
    error.value = '请选择或填写要监控的进程'
    return
  }
  if (form.kind === 'process' && form.match === 'command' && !String(form.command || '').trim()) {
    error.value = '按命令行匹配时需要填写命令'
    return
  }
  const passthrough = ['tcp', 'tls', 'cpu', 'memory', 'disk', 'script', 'process']
  const type = passthrough.includes(form.kind) ? form.kind : 'http'
  let url = (form.url || '').trim()
  if (form.kind === 'https' && url && !/^https?:\/\//.test(url)) url = 'https://' + url
  if (form.kind === 'http' && url && !/^https?:\/\//.test(url)) url = 'http://' + url
  const payload = {
    ...form, type, url, groupId: form.groupId || null,
    host: form.host,
    headers: httpKind ? (form.headers || {}) : {},
    body: httpKind ? (form.body || '') : '',
    bodyType: httpKind ? (form.bodyType || '') : '',
    method: httpKind ? (form.method || 'GET') : '',
    threshold: Number(form.threshold) || 0,
    memThreshold: form.kind === 'process' ? (Number(form.memThreshold) || 0) : 0,
    match: form.kind === 'process' ? (form.match || 'name') : '',
    command: form.kind === 'process' || form.kind === 'script' ? (form.command || '') : '',
  }
  saving.value = true
  try {
    const saved = editing.value
      ? await api('/api/monitors/' + route.params.id, { method: 'PUT', body: payload })
      : await api('/api/monitors', { method: 'POST', body: payload })
    router.push('/monitors/' + saved.id)
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  if (demoMode) {
    groups.value = demoGroups
    const row = editing.value ? presentDemo().overview.find((item) => String(item.id) === String(route.params.id)) : null
    if (row) {
      const kind = kindOf(row)
      Object.assign(form, row, { kind, groupId: row.groupId || null, notifierIds: [1, 2], headers: row.headers || {}, body: row.body || '', bodyType: row.bodyType || '' })
    } else if (!editing.value) {
      Object.assign(form, { name: 'ERP API', groupId: 1, kind: 'https', url: 'https://erp.example.com/health', notifierIds: [1, 2] })
      applyProcessQuery()
    }
    return
  }
  groups.value = await api('/api/groups')
  if (!editing.value) {
    applyProcessQuery()
    return
  }
  const row = await api('/api/monitors/' + route.params.id)
  const kind = kindOf(row)
  Object.assign(form, row, { kind, notifierIds: row.notifierIds || [], headers: row.headers || {}, body: row.body || '', bodyType: row.bodyType || '' })
})

function kindOf(row) {
  if (['tcp', 'tls', 'cpu', 'memory', 'disk', 'script', 'process'].includes(row.type)) return row.type
  return (row.url || '').startsWith('https') ? 'https' : 'http'
}
</script>
