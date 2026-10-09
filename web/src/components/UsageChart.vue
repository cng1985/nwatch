<template>
  <div class="usage">
    <div class="usage-legend">
      <button
        v-for="item in series"
        :key="item.id"
        type="button"
        :class="{ on: focus === item.id, dim: focus && focus !== item.id }"
        @click="toggle(item.id)"
      >
        <i :style="{ background: item.color }"></i>
        {{ item.label }}
        <b :class="tone(latestOf(item.id))">{{ latestOf(item.id).toFixed(1) }}%</b>
      </button>
    </div>
    <svg class="usage-svg" viewBox="0 0 760 196" role="img" aria-label="使用率趋势">
      <line
        v-for="tick in yTicks"
        :key="tick.value"
        :x1="plot.left"
        :y1="tick.y"
        :x2="plot.right"
        :y2="tick.y"
        stroke="#eef2f5"
      />
      <line
        v-for="guide in guides"
        :key="guide.value"
        :x1="plot.left"
        :y1="guide.y"
        :x2="plot.right"
        :y2="guide.y"
        :stroke="guide.color"
        stroke-opacity="0.45"
        stroke-dasharray="4 4"
      />
      <text
        v-for="tick in yTicks"
        :key="'y' + tick.value"
        :x="plot.left - 8"
        :y="tick.y + 4"
        text-anchor="end"
        font-size="11"
        fill="#8b98a5"
      >{{ tick.label }}</text>
      <polyline
        v-for="item in drawn"
        :key="item.id"
        :points="item.line"
        fill="none"
        :stroke="item.color"
        :stroke-width="focus === item.id ? 2.8 : 2.2"
        :stroke-opacity="lineOpacity(item.id)"
        stroke-linejoin="round"
        stroke-linecap="round"
      />
      <circle
        v-for="item in drawn"
        :key="'dot' + item.id"
        :cx="item.last[0]"
        :cy="item.last[1]"
        r="3.5"
        :fill="item.color"
        :fill-opacity="lineOpacity(item.id)"
      />
      <text
        v-for="(label, index) in xLabels"
        :key="index"
        :x="label.x"
        y="188"
        text-anchor="middle"
        font-size="11"
        fill="#8b98a5"
      >{{ label.text }}</text>
    </svg>
    <div class="trend-foot">
      <div v-for="item in series" :key="item.id">
        <span>{{ item.label }} 平均</span>
        <b>{{ avgOf(item.id).toFixed(1) }}%</b>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'

const props = defineProps({
  points: { type: Array, default: () => [] },
})

const series = [
  { id: 'cpu', label: 'CPU', color: '#12b981', field: 'cpuPercent' },
  { id: 'mem', label: '内存', color: '#3b82c4', field: 'memPercent' },
  { id: 'disk', label: '磁盘', color: '#d97706', field: 'diskPercent' },
]
const focus = ref('')
const plot = { left: 46, right: 736, top: 14, bottom: 164 }

function toggle(id) {
  focus.value = focus.value === id ? '' : id
}
function lineOpacity(id) {
  if (!focus.value || focus.value === id) return 1
  return 0.16
}
function tone(value) {
  if (value >= 90) return 'bad'
  if (value >= 70) return 'warn'
  return ''
}
function yOf(value) {
  const v = Math.max(0, Math.min(100, Number(value) || 0))
  return plot.bottom - (v / 100) * (plot.bottom - plot.top)
}
function xOf(index) {
  const n = props.points.length
  if (n <= 1) return (plot.left + plot.right) / 2
  return plot.left + (index / (n - 1)) * (plot.right - plot.left)
}
function values(field) {
  return props.points.map((point) => Number(point[field]) || 0)
}
function latestOf(id) {
  const field = series.find((item) => item.id === id).field
  const list = values(field)
  return list.length ? list[list.length - 1] : 0
}
function avgOf(id) {
  const field = series.find((item) => item.id === id).field
  const list = values(field)
  if (!list.length) return 0
  return list.reduce((sum, value) => sum + value, 0) / list.length
}

const yTicks = computed(() => [100, 75, 50, 25, 0].map((value) => ({
  value,
  y: yOf(value),
  label: value + '%',
})))
const guides = computed(() => [
  { value: 90, y: yOf(90), color: '#ef4444' },
  { value: 70, y: yOf(70), color: '#f5a524' },
])
const drawn = computed(() => series.map((item) => {
  const list = values(item.field)
  const coords = list.length <= 1
    ? [[plot.left, yOf(list[0])], [plot.right, yOf(list[0])]]
    : list.map((value, index) => [xOf(index), yOf(value)])
  return {
    ...item,
    line: coords.map((pair) => pair.join(',')).join(' '),
    last: coords.at(-1),
  }
}))
const xLabels = computed(() => {
  const n = props.points.length
  if (!n) return []
  const span = new Date(props.points[n - 1].time) - new Date(props.points[0].time)
  const count = Math.min(5, n)
  const indexes = [...new Set(Array.from({ length: count }, (_, i) => Math.round(i * (n - 1) / (count - 1 || 1))))]
  const labels = []
  let lastX = -Infinity
  indexes.forEach((index) => {
    const date = new Date(props.points[index].time)
    const hh = String(date.getHours()).padStart(2, '0')
    const mm = String(date.getMinutes()).padStart(2, '0')
    let text = `${hh}:${mm}`
    if (span > 36 * 3600000) text = `${date.getMonth() + 1}/${date.getDate()}`
    else if (span >= 6 * 3600000) text = `${hh}:00`
    const x = xOf(index)
    if (x - lastX < 72) return
    labels.push({ x, text })
    lastX = x
  })
  return labels
})
</script>

<style scoped>
.usage-legend { display: flex; flex-wrap: wrap; gap: 8px; }
.usage-legend button {
  display: inline-flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 10px; border-radius: 999px;
  border: 1px solid var(--line); background: white; color: #3d4d58; font-size: 12px;
}
.usage-legend button.on { border-color: #b7ebda; background: #f3fbf7; }
.usage-legend button.dim { opacity: 0.45; }
.usage-legend i { width: 8px; height: 8px; border-radius: 50%; }
.usage-legend b { font-variant-numeric: tabular-nums; color: var(--text); }
.usage-legend b.warn { color: #d97706; }
.usage-legend b.bad { color: var(--red); }
.usage-svg { width: 100%; height: 210px; display: block; margin-top: 6px; }
</style>
