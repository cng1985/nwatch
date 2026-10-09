<template>
  <div>
    <div class="page-head">
      <div>
        <h1>系统总览</h1>
        <p>所有服务状态，一目了然。</p>
      </div>
      <div class="head-actions">
        <select v-model="groupId" class="select">
          <option value="">全部分组</option>
          <option v-for="g in groups" :key="g.id" :value="String(g.id)">{{ g.name }}</option>
        </select>
        <router-link class="btn primary" to="/monitors/new"><Icon name="plus" style="width:16px;height:16px" /> 新增监控</router-link>
      </div>
    </div>

    <div v-if="data.openAlerts" class="alert-strip">
      <Icon name="alert" />
      <div><b>{{ data.openAlerts }} 个服务需要关注</b> {{ joinNames(data.problems) }}异常</div>
      <router-link to="/events">查看告警 →</router-link>
    </div>

    <div class="meters" v-if="host.cpuPercent !== undefined">
      <router-link class="card meter" to="/resources">
        <div class="label"><Icon name="cpu" /> CPU</div>
        <div class="value">{{ host.cpuPercent.toFixed(1) }}<small>%</small></div>
        <div class="meta">负载 {{ host.load1?.toFixed?.(1) || host.load1 || 0 }}</div>
        <span class="bar" :class="tone(host.cpuPercent)"><i :style="{ width: host.cpuPercent + '%' }"></i></span>
      </router-link>
      <router-link class="card meter" to="/resources">
        <div class="label"><Icon name="mem" /> 内存</div>
        <div class="value">{{ host.memPercent.toFixed(1) }}<small>%</small></div>
        <div class="meta">{{ bytes(host.memUsed) }} / {{ bytes(host.memTotal) }}</div>
        <span class="bar" :class="tone(host.memPercent)"><i :style="{ width: host.memPercent + '%' }"></i></span>
      </router-link>
      <router-link class="card meter" to="/resources">
        <div class="label"><Icon name="disk" /> 磁盘</div>
        <div class="value">{{ host.diskPercent.toFixed(1) }}<small>%</small></div>
        <div class="meta">{{ host.diskPath || '/' }}</div>
        <span class="bar" :class="tone(host.diskPercent)"><i :style="{ width: host.diskPercent + '%' }"></i></span>
      </router-link>
    </div>

    <div class="card stats">
      <div class="stat"><div class="label">监控总数</div><div class="value">{{ data.total || 0 }}<span v-if="data.addedSinceYesterday" class="delta">+{{ data.addedSinceYesterday }} 较昨日</span></div></div>
      <div class="stat"><div class="label"><i class="mark green"></i>正常运行</div><div class="value">{{ data.up || 0 }}</div></div>
      <div class="stat"><div class="label"><i class="mark red"></i>服务异常</div><div class="value">{{ data.down || 0 }}</div></div>
      <div class="stat"><div class="label"><i class="mark gray"></i>已暂停</div><div class="value">{{ data.paused || 0 }}</div></div>
    </div>

    <div class="dash-grid">
      <div class="card card-pad">
        <div class="card-title">
          <div><h2>响应时间趋势</h2><p>全部服务的平均响应时间</p></div>
          <div class="seg">
            <button v-for="item in ranges" :key="item" :class="{ on: range === item.id }" @click="changeRange(item.id)">{{ item.label }}</button>
          </div>
        </div>
        <Charts v-if="(data.trend?.points || []).length" kind="area" :points="data.trend.points" />
        <div v-else class="empty">开始检测后，这里会显示响应时间趋势</div>
        <div class="trend-foot">
          <div><span>平均</span><b>{{ data.trend?.avg || 0 }} ms <i :class="deltaClass">{{ deltaText }}</i></b></div>
          <div><span>最快</span><b>{{ data.trend?.min || 0 }} ms</b></div>
          <div><span>最慢</span><b>{{ data.trend?.max || 0 }} ms</b></div>
        </div>
      </div>
      <div class="card card-pad">
        <div class="card-title"><div><h2>24 小时可用率</h2><p>统计最近 24 小时的检测结果</p></div></div>
        <Charts kind="ring" :percent="data.availability24h || 0" />
        <div class="ring-legend">
          <span><i class="mark green"></i> 成功检测</span><b>{{ commas(data.success24h) }}</b>
        </div>
        <div class="ring-legend" style="border:0;padding-top:8px">
          <span><i class="mark red"></i> 失败检测</span><b>{{ commas(data.failure24h) }}</b>
        </div>
      </div>
    </div>

    <div class="card" style="margin-top:14px">
      <div class="card-title" style="padding:16px 16px 0">
        <h2>监控概览</h2>
        <router-link to="/monitors" style="color:#12b981;font-weight:650">查看全部</router-link>
      </div>
      <div class="table-wrap">
        <table>
          <thead><tr><th>监控名称</th><th>状态</th><th>响应时间</th><th>24h 可用率</th><th>最近检测</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>
                <router-link class="name-cell" :to="'/monitors/' + row.id">
                  <span class="name-ico"><Icon :name="iconOf(row)" /></span>
                  <span><b>{{ row.name }}</b><small>{{ targetOf(row) }}</small></span>
                </router-link>
              </td>
              <td><span class="status" :class="row.status"><i class="mark" :class="row.status === 'DOWN' ? 'red' : row.status === 'UP' ? 'green' : 'gray'"></i>{{ statusLabel(row.status) }}</span></td>
              <td>{{ row.lastResponseTime ? row.lastResponseTime + ' ms' : '-' }}</td>
              <td>
                <span class="bar" :class="{ bad: row.status === 'DOWN' }"><i :style="{ width: (row.hasAvailability ? row.availability24h : 0) + '%' }"></i></span>
                <span class="pct">{{ row.hasAvailability ? row.availability24h.toFixed(row.availability24h % 1 ? 2 : 0) + '%' : '-' }}</span>
              </td>
              <td>{{ relative(row.lastCheckAt) }}</td>
            </tr>
            <tr v-if="!rows.length"><td colspan="5" class="empty">还没有监控，点击右上角新增。</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import Icon from '../components/Icon.vue'
import Charts from '../components/Charts.vue'
import { api } from '../api'
import { demoGroups, demoTrend, presentDemo } from '../demo'
import { bytes, iconOf, isDemo, joinNames, relative, statusLabel, targetOf } from '../format'

const data = reactive({ trend: { points: [] }, problems: [], overview: [] })
const host = reactive({})
const groups = ref([])
const groupId = ref('')
const range = ref('24h')
const ranges = [{ id: '24h', label: '24 小时' }, { id: '7d', label: '7 天' }, { id: '30d', label: '30 天' }]
const rows = computed(() => {
  const list = data.overview || []
  if (!groupId.value) return list
  return list.filter((item) => String(item.groupId || '') === groupId.value)
})
const deltaText = computed(() => {
  const delta = data.trend?.delta
  if (delta === null || delta === undefined) return ''
  const arrow = delta <= 0 ? '↓' : '↑'
  return arrow + Math.abs(delta).toFixed(0) + '%'
})
const deltaClass = computed(() => (data.trend?.delta || 0) <= 0 ? 'down-good' : 'up-bad')
function commas(value) { return Number(value || 0).toLocaleString('en-US') }

function tone(value) {
  if (value >= 90) return 'bad'
  if (value >= 70) return 'warn'
  return ''
}
function applyHost(current) {
  const disks = current?.disks || []
  const root = disks.find((item) => item.path === '/') || disks[0] || {}
  Object.assign(host, {
    cpuPercent: current.cpuPercent || 0,
    memPercent: current.memPercent || 0,
    memUsed: current.memUsed || 0,
    memTotal: current.memTotal || 0,
    load1: current.load1 || 0,
    diskPercent: root.percent || 0,
    diskPath: root.path || '/',
  })
}

async function load() {
  if (isDemo()) {
    const demo = presentDemo()
    Object.assign(data, { ...demo, trend: range.value === '24h' ? demo.trend : demoTrend(range.value) })
    groups.value = demoGroups
    applyHost({ cpuPercent: 18.4, memPercent: 46.2, memUsed: 7.4e9, memTotal: 16e9, load1: 0.42, disks: [{ path: '/', percent: 61 }] })
    return
  }
  const dash = await api('/api/dashboard?range=' + range.value)
  Object.assign(data, dash)
  groups.value = await api('/api/groups')
  try {
    const resources = await api('/api/host?range=1h')
    applyHost(resources.current || {})
  } catch { /* 主机采样失败时总览仍可用 */ }
}
function changeRange(next) { range.value = next; load() }
onMounted(load)
</script>
