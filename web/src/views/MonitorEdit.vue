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
            <label v-if="form.kind !== 'tcp'" class="field full"><span class="req">目标 URL</span>
              <input v-model="form.url" class="text" :placeholder="form.kind === 'tls' ? 'api.example.com' : 'https://erp.example.com/health'" />
            </label>
            <template v-else>
              <label class="field"><span class="req">主机</span><input v-model="form.host" class="text" placeholder="192.168.1.100" /></label>
              <label class="field"><span class="req">端口</span><input v-model.number="form.port" class="text" type="number" /></label>
            </template>
            <label v-if="form.kind === 'http' || form.kind === 'https'" class="field"><span class="req">请求方法</span>
              <select v-model="form.method" class="select" style="width:100%"><option>GET</option><option>POST</option><option>HEAD</option></select>
            </label>
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
            <div class="full">
              <span class="req" style="font-weight:600">通知渠道</span>
              <div class="checks">
                <label v-for="item in notifiers" :key="item.id" class="check"><input v-model="form.notifierIds" type="checkbox" :value="item.id" /> {{ item.name }}</label>
                <span v-if="!notifiers.length" class="demo-note">尚未配置通知渠道，可稍后在通知渠道中添加。</span>
              </div>
            </div>
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
        <div class="kv"><span>检测频率</span><b>每 {{ intervalLabel }}</b></div>
        <div class="kv"><span>超时时间</span><b>{{ form.timeout }} 秒</b></div>
        <div class="kv"><span>异常判定</span><b>连续失败 {{ form.failureThreshold }} 次</b></div>
        <div class="kv"><span>恢复判定</span><b>连续成功 {{ form.recoveryThreshold }} 次</b></div>
        <div class="kv"><span>通知渠道</span><b>{{ form.notifierIds.length }} 个</b></div>
        <div class="tip">
          <b><Icon name="bulb" style="width:16px;height:16px;color:#12b981" /> 配置小贴士</b>
          建议使用专用的 /health 接口作为监控目标。在创建后，您可以立即执行一次检测，确认服务配置。
          <div v-if="demoMode" class="demo-note">演示数据，用于页面设计预览。</div>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Icon from '../components/Icon.vue'
import { api } from '../api'
import { demoGroups, demoNotifiers, presentDemo } from '../demo'
import { isDemo } from '../format'

const route = useRoute()
const router = useRouter()
const editing = computed(() => !!route.params.id)
const demoMode = isDemo()
const groups = ref([])
const notifiers = ref([])
const saving = ref(false)
const error = ref('')
const advanced = ref(false)
const kinds = [
  { id: 'http', label: 'HTTP', icon: 'globe' },
  { id: 'https', label: 'HTTPS', icon: 'lock' },
  { id: 'tcp', label: 'TCP 端口', icon: 'db' },
  { id: 'tls', label: 'HTTPS 证书', icon: 'file' },
]
const intervals = [[15, '15 秒'], [30, '30 秒'], [60, '60 秒'], [300, '5 分钟'], [600, '10 分钟'], [1800, '30 分钟'], [3600, '1 小时'], [43200, '12 小时'], [86400, '24 小时']]
const form = reactive({
  name: '', groupId: null, kind: 'https', url: '', host: '', port: 443, method: 'GET',
  expectedStatusCodes: '200', interval: 60, timeout: 5, failureThreshold: 3, recoveryThreshold: 1,
  bodyContains: '', bodyNotContains: '', followRedirects: true, tlsWarningDays: 14, tlsCriticalDays: 7,
  tlsNotifyDays: '30,14,7,3,1,0', notifierIds: [], enabled: true,
})
const currentKind = computed(() => kinds.find((item) => item.id === form.kind))
const intervalLabel = computed(() => intervals.find((item) => item[0] === form.interval)?.[1] || form.interval + ' 秒')
const previewTarget = computed(() => form.kind === 'tcp' ? (form.host ? `${form.host}:${form.port || ''}` : '未填写地址') : (form.url || '未填写地址'))

function cancel() { router.push('/monitors') }

async function save() {
  error.value = ''
  if (demoMode) { error.value = '演示模式不会写入服务器'; return }
  const type = form.kind === 'tcp' ? 'tcp' : form.kind === 'tls' ? 'tls' : 'http'
  let url = (form.url || '').trim()
  if (form.kind === 'https' && url && !/^https?:\/\//.test(url)) url = 'https://' + url
  if (form.kind === 'http' && url && !/^https?:\/\//.test(url)) url = 'http://' + url
  const payload = {
    ...form, type, url, groupId: form.groupId || null,
    headers: {},
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
    notifiers.value = demoNotifiers
    const row = editing.value ? presentDemo().overview.find((item) => String(item.id) === String(route.params.id)) : null
    if (row) {
      const kind = row.type === 'tcp' ? 'tcp' : row.type === 'tls' ? 'tls' : (row.url || '').startsWith('https') ? 'https' : 'http'
      Object.assign(form, row, { kind, groupId: row.groupId || null, notifierIds: [1, 2] })
    } else if (!editing.value) {
      Object.assign(form, { name: 'ERP API', groupId: 1, kind: 'https', url: 'https://erp.example.com/health', notifierIds: [1, 2] })
    }
    return
  }
  groups.value = await api('/api/groups')
  notifiers.value = await api('/api/notifiers')
  if (!editing.value) return
  const row = await api('/api/monitors/' + route.params.id)
  const kind = row.type === 'tcp' ? 'tcp' : row.type === 'tls' ? 'tls' : (row.url || '').startsWith('https') ? 'https' : 'http'
  Object.assign(form, row, { kind, notifierIds: row.notifierIds || [] })
})
</script>
