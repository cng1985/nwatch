<template>
  <div>
    <div class="page-head">
      <div>
        <h1>主机资源</h1>
        <p>{{ subtitle }}</p>
      </div>
      <div class="seg">
        <button v-for="item in ranges" :key="item.id" :class="{ on: range === item.id }" @click="range = item.id; load()">{{ item.label }}</button>
      </div>
    </div>
    <div class="meters">
      <div class="card meter">
        <div class="label"><Icon name="cpu" /> CPU</div>
        <div class="value">{{ current.cpuPercent.toFixed(1) }}<small>%</small></div>
        <div class="meta">{{ current.cpuCount || 0 }} 核 · 负载 {{ current.load1.toFixed(2) }} / {{ current.load5.toFixed(2) }} / {{ current.load15.toFixed(2) }}</div>
        <span class="bar" :class="tone(current.cpuPercent)"><i :style="{ width: current.cpuPercent + '%' }"></i></span>
      </div>
      <div class="card meter">
        <div class="label"><Icon name="mem" /> 内存</div>
        <div class="value">{{ current.memPercent.toFixed(1) }}<small>%</small></div>
        <div class="meta">{{ bytes(current.memUsed) }} / {{ bytes(current.memTotal) }}<template v-if="current.swapTotal"> · 交换 {{ current.swapPercent.toFixed(1) }}%</template></div>
        <span class="bar" :class="tone(current.memPercent)"><i :style="{ width: current.memPercent + '%' }"></i></span>
      </div>
      <div class="card meter">
        <div class="label"><Icon name="disk" /> 根分区</div>
        <div class="value">{{ root.percent.toFixed(1) }}<small>%</small></div>
        <div class="meta">{{ bytes(root.used) }} / {{ bytes(root.total) }} · {{ root.path || '/' }}</div>
        <span class="bar" :class="tone(root.percent)"><i :style="{ width: root.percent + '%' }"></i></span>
      </div>
    </div>
    <div class="card card-pad" style="margin-bottom:14px">
      <div class="card-title">
        <div><h2>使用率趋势</h2><p>三条曲线共用时间轴，虚线标出 70% 和 90%。采样约 15 秒一次，保留 7 天。</p></div>
      </div>
      <UsageChart v-if="history.length" :points="history" />
      <div v-else class="empty">还没有采样</div>
    </div>
    <div class="card table-wrap">
      <h2 style="padding:16px 16px 0">磁盘分区</h2>
      <table>
        <thead><tr><th>挂载点</th><th>文件系统</th><th>已用</th><th>容量</th><th>使用率</th></tr></thead>
        <tbody>
          <tr v-for="disk in current.disks" :key="disk.path">
            <td>{{ disk.path }}</td>
            <td>{{ disk.fsType || '-' }}</td>
            <td>{{ bytes(disk.used) }}</td>
            <td>{{ bytes(disk.total) }}</td>
            <td><span class="bar" :class="tone(disk.percent)"><i :style="{ width: disk.percent + '%' }"></i></span><span class="pct">{{ disk.percent.toFixed(1) }}%</span></td>
          </tr>
          <tr v-if="!current.disks.length"><td colspan="5" class="empty">没有读到磁盘</td></tr>
        </tbody>
      </table>
    </div>
    <p class="demo-note">需要告警时，到监控列表新增 CPU、内存或磁盘监控，超过阈值后走现有通知渠道。</p>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import Icon from '../components/Icon.vue'
import UsageChart from '../components/UsageChart.vue'
import { api } from '../api'
import { bytes, duration, isDemo } from '../format'

const ranges = [{ id: '1h', label: '1 小时' }, { id: '6h', label: '6 小时' }, { id: '24h', label: '24 小时' }, { id: '7d', label: '7 天' }]
const range = ref('1h')
const history = ref([])
const current = reactive({
  hostname: '', cpuCount: 0, cpuPercent: 0, memPercent: 0, memUsed: 0, memTotal: 0,
  swapTotal: 0, swapPercent: 0, load1: 0, load5: 0, load15: 0, uptime: 0, disks: [],
})
const root = computed(() => current.disks.find((item) => item.path === '/') || current.disks[0] || { percent: 0, used: 0, total: 0, path: '/' })
const subtitle = computed(() => {
  const name = current.hostname || '本机'
  return `${name} · 已运行 ${duration(Math.floor(current.uptime || 0))}`
})
function tone(value) {
  if (value >= 90) return 'bad'
  if (value >= 70) return 'warn'
  return ''
}

async function load() {
  if (isDemo()) {
    Object.assign(current, {
      hostname: 'demo', cpuCount: 4, cpuPercent: 18.4, memPercent: 46.2, memUsed: 7.4e9, memTotal: 16e9,
      swapTotal: 0, swapPercent: 0, load1: 0.42, load5: 0.38, load15: 0.31, uptime: 86400 * 3 + 3600,
      disks: [
        { path: '/', fsType: 'ext4', used: 48e9, total: 80e9, percent: 61 },
        { path: '/data', fsType: 'xfs', used: 120e9, total: 400e9, percent: 30 },
      ],
    })
    const now = Date.now()
    history.value = Array.from({ length: 24 }, (_, i) => ({
      time: new Date(now - (23 - i) * 150000).toISOString(),
      cpuPercent: 16 + Math.sin(i / 3) * 6,
      memPercent: 72 + (i % 4),
      diskPercent: 88 + (i % 3) * 0.4,
    }))
    return
  }
  const data = await api('/api/host?range=' + range.value)
  Object.assign(current, data.current || {})
  current.disks = data.current?.disks || []
  history.value = data.history || []
}

let timer
onMounted(() => {
  load()
  timer = setInterval(load, 15000)
})
onUnmounted(() => clearInterval(timer))
</script>
