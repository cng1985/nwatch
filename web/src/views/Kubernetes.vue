<template>
  <div>
    <div class="page-head">
      <div>
        <h1>Kubernetes</h1>
        <p>查看当前 kubeconfig 能访问的工作负载，滚动重启、调整副本，并查看 Pod 日志。操作身份与 NMonitor 进程相同。</p>
      </div>
      <button class="btn ghost" :disabled="loading" @click="load">刷新</button>
    </div>
    <div v-if="error" class="alert-strip"><Icon name="alert" /> {{ error }}</div>
    <div v-if="notice" class="card card-pad" style="margin-bottom:14px">{{ notice }}</div>
    <div class="toolbar">
      <select v-model="kind" class="select" @change="changeKind">
        <option value="pods">Pod</option>
        <option value="deployments">Deployment</option>
        <option value="statefulsets">StatefulSet</option>
        <option value="daemonsets">DaemonSet</option>
        <option value="services">Service</option>
      </select>
      <select v-model="namespace" class="select" @change="load">
        <option value="">全部命名空间</option>
        <option v-for="item in namespaces" :key="item.name" :value="item.name">{{ item.name }}</option>
      </select>
      <input v-model="keyword" class="search" placeholder="搜索名称、状态、镜像或地址" @keyup.enter="load" />
      <button class="btn ghost" @click="load">搜索</button>
      <span v-if="context" class="demo-note">上下文 {{ context }}</span>
    </div>
    <div class="card table-wrap">
      <table>
        <thead>
          <tr>
            <th>名称</th>
            <th>命名空间</th>
            <th>状态</th>
            <th v-if="kind !== 'services'">就绪</th>
            <th v-if="kind === 'pods'">重启</th>
            <th v-if="kind === 'pods'">节点</th>
            <th v-if="kind === 'services'">地址</th>
            <th v-if="kind === 'services'">端口</th>
            <th>存在时间</th>
            <th v-if="kind !== 'services'" style="width:210px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in items" :key="row.namespace + '/' + row.name" :class="{ 'row-on': logs.key === rowKey(row) }">
            <td>
              <div class="name-cell">
                <div class="name-ico"><Icon name="kube" /></div>
                <div><b>{{ row.name }}</b><small>{{ row.image || kindLabel(kind) }}</small></div>
              </div>
            </td>
            <td>{{ row.namespace || '-' }}</td>
            <td><span class="status" :class="stateClass(row.status)">{{ stateLabel(row.status) }}</span></td>
            <td v-if="kind !== 'services'">{{ row.ready || '-' }}</td>
            <td v-if="kind === 'pods'">{{ row.restarts }}</td>
            <td v-if="kind === 'pods'">{{ row.node || '-' }}</td>
            <td v-if="kind === 'services'">{{ row.clusterIP || '-' }}</td>
            <td v-if="kind === 'services'">{{ row.ports || '-' }}</td>
            <td>{{ row.age || '-' }}</td>
            <td v-if="kind !== 'services'">
              <div class="actions-inline" style="flex-wrap:wrap">
                <button v-if="can('kube.write') && canRestart" :disabled="!!busy" @click="restart(row)">{{ busy === rowKey(row) + ':restart' ? '处理中' : '重启' }}</button>
                <button v-if="can('kube.write') && canScale" :disabled="!!busy" @click="scale(row)">副本</button>
                <button v-if="kind === 'pods'" :disabled="!!busy" @click="openLogs(row)">日志</button>
                <button v-if="can('kube.write') && kind === 'pods'" class="danger" :disabled="!!busy" @click="remove(row)">删除</button>
              </div>
            </td>
          </tr>
          <tr v-if="!items.length"><td :colspan="columns" class="empty">{{ emptyText }}</td></tr>
        </tbody>
      </table>
    </div>
    <div v-if="logs.name" class="card card-pad" style="margin-top:14px">
      <div class="card-title">
        <div>
          <h2>{{ logs.namespace }}/{{ logs.name }}</h2>
          <p>最近 {{ logLines }} 行。日志来自 kubectl logs。</p>
        </div>
        <div class="head-actions">
          <select v-if="logs.containers.length > 1" v-model="logs.container" class="select" @change="loadLogs">
            <option v-for="name in logs.containers" :key="name" :value="name">{{ name }}</option>
          </select>
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
    <p class="demo-note">重启会执行 kubectl rollout restart。删除 Pod 后，由控制器负责重建。副本数限制在 0 到 64。这里不提供进入容器的终端。关闭本功能可设置 kubernetes.enabled 为 false，或使用环境变量 NMONITOR_KUBERNETES_ENABLED=false。上下文用 kubernetes.context 或 NMONITOR_KUBE_CONTEXT，kubeconfig 用 kubernetes.kubeconfig 或 NMONITOR_KUBECONFIG。</p>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import { api } from '../api'
import { can } from '../session'
import { isDemo } from '../format'

const kind = ref('pods')
const namespace = ref('')
const namespaces = ref([])
const items = ref([])
const keyword = ref('')
const context = ref('')
const error = ref('')
const notice = ref('')
const loading = ref(false)
const busy = ref('')
const logLines = ref(200)
const follow = ref(false)
const logs = ref({ key: '', namespace: '', name: '', container: '', containers: [], text: '' })
const emptyText = ref('没有匹配的资源')

const demoNamespaces = [
  { name: 'default', status: 'Active' },
  { name: 'prod', status: 'Active' },
]
const demoItems = [
  { kind: 'pods', namespace: 'prod', name: 'web', status: 'Running', ready: '1/1', restarts: 0, node: 'node-a', image: 'nginx:1.25', age: '12d', containers: ['app'] },
  { kind: 'pods', namespace: 'prod', name: 'worker', status: 'CrashLoopBackOff', ready: '0/1', restarts: 8, node: 'node-b', image: 'app:1', age: '2d', containers: ['app', 'sidecar'] },
  { kind: 'deployments', namespace: 'prod', name: 'web', status: 'Available', ready: '2/2', age: '12d', image: 'nginx:1.25' },
  { kind: 'deployments', namespace: 'default', name: 'api', status: 'Progressing', ready: '1/3', age: '4h', image: 'api:2' },
  { kind: 'statefulsets', namespace: 'prod', name: 'db', status: 'Available', ready: '3/3', age: '30d', image: 'postgres:16' },
  { kind: 'daemonsets', namespace: 'kube-system', name: 'agent', status: 'Available', ready: '2/2', age: '40d', image: 'agent:1' },
  { kind: 'services', namespace: 'prod', name: 'web', status: 'ClusterIP', clusterIP: '10.0.0.8', ports: '80/TCP', age: '12d' },
]

const canRestart = computed(() => kind.value === 'deployments' || kind.value === 'statefulsets' || kind.value === 'daemonsets')
const canScale = computed(() => kind.value === 'deployments' || kind.value === 'statefulsets')
const columns = computed(() => {
  if (kind.value === 'pods') return 8
  if (kind.value === 'services') return 6
  return 6
})

function rowKey(row) {
  return `${row.namespace}/${row.name}`
}

function kindLabel(value) {
  return { pods: 'Pod', deployments: 'Deployment', statefulsets: 'StatefulSet', daemonsets: 'DaemonSet', services: 'Service' }[value] || value
}

function stateClass(value) {
  if (['Running', 'Succeeded', 'Available', 'Completed', 'ClusterIP', 'NodePort', 'LoadBalancer'].includes(value)) return 'UP'
  if (['Failed', 'CrashLoopBackOff', 'ImagePullBackOff', 'ErrImagePull', 'Error', 'OOMKilled'].includes(value)) return 'DOWN'
  return 'PAUSED'
}

function stateLabel(value) {
  return {
    Running: '运行中',
    Pending: '等待中',
    Succeeded: '已完成',
    Failed: '失败',
    Unknown: '未知',
    Available: '就绪',
    Progressing: '更新中',
    Stopped: '已缩容',
    CrashLoopBackOff: '反复崩溃',
    ImagePullBackOff: '镜像拉取失败',
    ErrImagePull: '镜像拉取失败',
    ContainerCreating: '创建中',
    PodInitializing: '初始化',
    ClusterIP: '集群内',
    NodePort: '节点端口',
    LoadBalancer: '负载均衡',
    ExternalName: '外部名称',
  }[value] || value || '未知'
}

function matches(row) {
  if (row.kind !== kind.value) return false
  if (namespace.value && row.namespace !== namespace.value) return false
  const q = keyword.value.trim().toLowerCase()
  if (!q) return true
  return `${row.namespace} ${row.name} ${row.status} ${row.image || ''} ${row.node || ''} ${row.ports || ''} ${row.clusterIP || ''}`.toLowerCase().includes(q)
}

function changeKind() {
  logs.value = { key: '', namespace: '', name: '', container: '', containers: [], text: '' }
  load()
}

async function load() {
  error.value = ''
  if (isDemo()) {
    notice.value = ''
    context.value = 'demo'
    namespaces.value = demoNamespaces
    items.value = demoItems.filter(matches)
    emptyText.value = '没有匹配的资源'
    return
  }
  loading.value = true
  try {
    const ns = await api('/api/kubernetes/namespaces')
    namespaces.value = ns.items || []
    if (namespace.value && !namespaces.value.some((item) => item.name === namespace.value)) namespace.value = ''
    const params = new URLSearchParams({ kind: kind.value })
    if (namespace.value) params.set('namespace', namespace.value)
    if (keyword.value.trim()) params.set('q', keyword.value.trim())
    const data = await api('/api/kubernetes/resources?' + params.toString())
    items.value = data.items || []
    context.value = data.context || ns.context || ''
    notice.value = data.reason || ns.reason || ''
    if (!data.enabled) {
      notice.value = data.reason || 'Kubernetes 管理已关闭'
      emptyText.value = 'Kubernetes 管理已关闭'
    } else if (!data.available) {
      notice.value = data.reason || '当前环境没有 kubectl'
      emptyText.value = '当前环境无法读取 Kubernetes'
    } else {
      emptyText.value = '没有匹配的资源'
    }
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function restart(row) {
  if (!window.confirm(`确定滚动重启 ${row.namespace}/${row.name}？`)) return
  error.value = ''
  if (isDemo()) {
    row.status = 'Progressing'
    return
  }
  busy.value = rowKey(row) + ':restart'
  try {
    await api(`/api/kubernetes/workloads/${kind.value}/${encodeURIComponent(row.namespace)}/${encodeURIComponent(row.name)}/restart`, { method: 'POST' })
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function scale(row) {
  const current = String(row.ready || '').split('/')[1] || ''
  const value = window.prompt(`设置 ${row.namespace}/${row.name} 的副本数（0 到 64）`, current)
  if (value == null) return
  const replicas = Number(value)
  if (!Number.isInteger(replicas) || replicas < 0 || replicas > 64) {
    error.value = '副本数需要是 0 到 64 的整数'
    return
  }
  error.value = ''
  if (isDemo()) {
    const ready = String(row.ready || '0/0').split('/')[0]
    row.ready = `${ready}/${replicas}`
    row.status = replicas === 0 ? 'Stopped' : 'Progressing'
    return
  }
  busy.value = rowKey(row) + ':scale'
  try {
    await api(`/api/kubernetes/workloads/${kind.value}/${encodeURIComponent(row.namespace)}/${encodeURIComponent(row.name)}/scale`, { method: 'POST', body: { replicas } })
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function remove(row) {
  if (!window.confirm(`删除 ${row.namespace}/${row.name} 后，控制器会重新创建。确定删除？`)) return
  error.value = ''
  if (isDemo()) {
    items.value = items.value.filter((item) => rowKey(item) !== rowKey(row))
    if (logs.value.key === rowKey(row)) logs.value = { key: '', namespace: '', name: '', container: '', containers: [], text: '' }
    return
  }
  busy.value = rowKey(row) + ':delete'
  try {
    await api(`/api/kubernetes/pods/${encodeURIComponent(row.namespace)}/${encodeURIComponent(row.name)}/delete`, { method: 'POST' })
    if (logs.value.key === rowKey(row)) logs.value = { key: '', namespace: '', name: '', container: '', containers: [], text: '' }
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = ''
  }
}

async function openLogs(row) {
  const containers = row.containers || []
  logs.value = {
    key: rowKey(row),
    namespace: row.namespace,
    name: row.name,
    container: containers[0] || '',
    containers,
    text: logs.value.key === rowKey(row) ? logs.value.text : '',
  }
  await loadLogs()
}

async function loadLogs() {
  if (!logs.value.name) return
  if (isDemo()) {
    logs.value = { ...logs.value, text: `2026-10-14T08:00:00Z demo ${logs.value.namespace}/${logs.value.name}: 示例日志\n` }
    return
  }
  try {
    const params = new URLSearchParams({ lines: String(logLines.value) })
    if (logs.value.container) params.set('container', logs.value.container)
    const data = await api(`/api/kubernetes/pods/${encodeURIComponent(logs.value.namespace)}/${encodeURIComponent(logs.value.name)}/logs?${params.toString()}`)
    logs.value = { ...logs.value, text: data.text || '没有日志' }
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
