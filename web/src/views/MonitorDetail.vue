<template>
  <div v-if="monitor">
    <div class="page-head">
      <div>
        <h1>{{ monitor.name }}</h1>
        <p>{{ targetOf(monitor) }}</p>
      </div>
      <div class="head-actions">
        <span class="status" :class="monitor.status"><i class="mark" :class="monitor.status === 'DOWN' ? 'red' : 'green'"></i>{{ statusLabel(monitor.status) }}</span>
        <button class="btn ghost" @click="checkNow">立即检测</button>
        <button class="btn ghost" @click="toggle">{{ monitor.enabled ? '暂停' : '启用' }}</button>
        <router-link class="btn primary" :to="'/monitors/' + monitor.id + '/edit'">编辑</router-link>
      </div>
    </div>
    <p v-if="message" class="demo-note" style="margin-bottom:12px">{{ message }}</p>
    <div class="card stats">
      <div class="stat"><div class="label">最近响应</div><div class="value">{{ monitor.lastResponseTime || 0 }}<span style="font-size:14px"> ms</span></div></div>
      <div class="stat"><div class="label">连续失败</div><div class="value">{{ monitor.consecutiveFailures || 0 }}</div></div>
      <div class="stat"><div class="label">24 小时可用率</div><div class="value" style="font-size:28px">{{ avail }}</div></div>
      <div class="stat"><div class="label">最近检测</div><div class="value" style="font-size:18px">{{ relative(monitor.lastCheckAt) }}</div></div>
    </div>
    <div class="card card-pad" style="margin-bottom:14px">
      <div class="card-title"><h2>响应时间</h2>
        <div class="seg"><button v-for="item in ['1h','24h','7d','30d']" :key="item" :class="{ on: range === item }" @click="range = item; loadMetrics()">{{ item }}</button></div>
      </div>
      <Charts v-if="points.length" kind="area" :points="points" />
      <div v-else class="empty">这个时间范围还没有统计</div>
    </div>
    <div class="dash-grid">
      <div class="card table-wrap">
        <h2 style="padding:16px 16px 0">最近检测</h2>
        <table><thead><tr><th>时间</th><th>结果</th><th>耗时</th><th>说明</th></tr></thead>
          <tbody><tr v-for="row in checks" :key="row.id"><td>{{ clock(row.checkedAt) }}</td><td>{{ row.success ? '成功' : '失败' }}</td><td>{{ row.responseTime }} ms</td><td>{{ row.errorMessage }}</td></tr></tbody>
        </table>
      </div>
      <div class="card table-wrap">
        <h2 style="padding:16px 16px 0">历史事件</h2>
        <table><thead><tr><th>时间</th><th>事件</th></tr></thead>
          <tbody><tr v-for="row in events" :key="row.id"><td>{{ clock(row.occurredAt) }}</td><td>{{ eventText[row.eventType] || row.eventType }}</td></tr></tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import Charts from '../components/Charts.vue'
import { api } from '../api'
import { clock, eventText, isDemo, percent, relative, statusLabel, targetOf } from '../format'
import { demoEvents, presentDemo } from '../demo'

const route = useRoute()
const monitor = ref(null)
const checks = ref([])
const events = ref([])
const points = ref([])
const avail = ref('-')
const range = ref('24h')
const message = ref('')

async function load() {
  if (isDemo()) {
    const demo = presentDemo()
    monitor.value = demo.overview.find((item) => String(item.id) === String(route.params.id)) || demo.overview[0]
    points.value = demo.trend.points
    events.value = demoEvents.filter((item) => item.monitorName === monitor.value.name)
    avail.value = monitor.value.hasAvailability ? Number(monitor.value.availability24h).toFixed(2) + '%' : '-'
    message.value = '演示数据，不会请求服务器。'
    return
  }
  monitor.value = await api('/api/monitors/' + route.params.id)
  checks.value = (await api('/api/monitors/' + route.params.id + '/checks?pageSize=8')).items || []
  events.value = (await api('/api/monitors/' + route.params.id + '/events?pageSize=8')).items || []
  const day = await api('/api/monitors/' + route.params.id + '/metrics?range=24h')
  avail.value = percent(day.availability, day.total > 0)
  await loadMetrics()
}
async function loadMetrics() {
  if (isDemo()) return
  const data = await api('/api/monitors/' + route.params.id + '/metrics?range=' + range.value)
  points.value = (data.buckets || []).map((item) => ({ time: item.time, avg: item.avgResponseTime }))
}
async function checkNow() {
  if (isDemo()) { message.value = '演示模式不会发起检测'; return }
  const data = await api('/api/monitors/' + route.params.id + '/check', { method: 'POST' })
  message.value = data.result?.message || '检测完成'
  await load()
}
async function toggle() {
  if (isDemo()) { message.value = '演示模式不会修改状态'; return }
  await api('/api/monitors/' + route.params.id + (monitor.value.enabled ? '/disable' : '/enable'), { method: 'POST' })
  await load()
}
onMounted(load)
</script>
