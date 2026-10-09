<template>
  <svg v-if="kind === 'area'" class="chart" viewBox="0 0 680 220" role="img">
    <defs>
      <linearGradient id="areaFill" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" stop-color="#12b981" stop-opacity="0.28" />
        <stop offset="100%" stop-color="#12b981" stop-opacity="0.02" />
      </linearGradient>
    </defs>
    <line v-for="tick in ticks" :key="tick.y" x1="46" :y1="tick.y" x2="664" :y2="tick.y" stroke="#eef2f5" />
    <text v-for="tick in ticks" :key="'t' + tick.y" x="4" :y="tick.y + 4" font-size="11" fill="#8b98a5">{{ tick.text }}</text>
    <polygon :points="area" fill="url(#areaFill)" />
    <polyline :points="line" fill="none" stroke="#12b981" stroke-width="2.4" stroke-linejoin="round" stroke-linecap="round" />
    <text v-for="label in labels" :key="label.x + label.text" :x="label.x" y="210" font-size="11" fill="#8b98a5">{{ label.text }}</text>
  </svg>
  <div v-else class="ring-wrap">
    <svg width="188" height="188" viewBox="0 0 188 188">
      <circle cx="94" cy="94" r="72" fill="none" stroke="#e8eef2" stroke-width="16" />
      <circle cx="94" cy="94" r="72" fill="none" stroke="#12b981" stroke-width="16" stroke-linecap="round"
        :stroke-dasharray="dash" transform="rotate(-90 94 94)" />
      <circle v-if="fail > 0.4" cx="94" cy="94" r="72" fill="none" stroke="#ef4444" stroke-width="16" stroke-linecap="round"
        :stroke-dasharray="failDash" :stroke-dashoffset="failOffset" transform="rotate(-90 94 94)" />
      <text x="94" y="100" text-anchor="middle" font-size="28" font-weight="700" fill="#17232c">{{ shown }}</text>
    </svg>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  kind: { type: String, default: 'area' },
  points: { type: Array, default: () => [] },
  percent: { type: Number, default: 0 },
})
const ceiling = computed(() => {
  const peak = Math.max(1, ...props.points.map((p) => p.avg || 0))
  const steps = [100, 200, 300, 500, 800, 1000, 2000, 5000]
  return steps.find((step) => step >= peak * 1.15) || Math.ceil(peak / 1000) * 1000
})
const ticks = computed(() => [1, 2 / 3, 1 / 3].map((ratio) => ({
  y: 168 - ratio * 132,
  text: Math.round(ceiling.value * ratio) + ' ms',
})))
const coords = computed(() => props.points.map((p, i) => {
  const x = props.points.length <= 1 ? 355 : 52 + (i / (props.points.length - 1)) * 600
  const y = 168 - ((p.avg || 0) / ceiling.value) * 132
  return [x, y]
}))
const line = computed(() => coords.value.map((c) => c.join(',')).join(' '))
const area = computed(() => {
  if (!coords.value.length) return ''
  const first = coords.value[0][0]
  const last = coords.value.at(-1)[0]
  return `${first},168 ${line.value} ${last},168`
})
const labels = computed(() => {
  const n = props.points.length
  if (!n) return []
  const span = new Date(props.points[n - 1].time) - new Date(props.points[0].time)
  const onHour = props.points.every((point) => new Date(point.time).getMinutes() === 0)
  if (onHour && span < 36 * 3600000 && n >= 12) {
    return props.points.flatMap((point, i) => {
      const hour = new Date(point.time).getHours()
      if (hour % 4 !== 0) return []
      return [{ x: coords.value[i][0] - 14, text: `${String(hour).padStart(2, '0')}:00` }]
    })
  }
  const count = Math.min(6, n)
  const indexes = Array.from({ length: count }, (_, i) => Math.round(i * (n - 1) / (count - 1 || 1)))
  return [...new Set(indexes)].map((i) => {
    const date = new Date(props.points[i].time)
    const hh = String(date.getHours()).padStart(2, '0')
    const text = span > 36 * 3600000 ? `${date.getMonth() + 1}/${date.getDate()}` : `${hh}:00`
    return { x: coords.value[i][0] - 14, text }
  })
})
const shown = computed(() => props.percent ? props.percent.toFixed(2) + '%' : '-')
const circ = 2 * Math.PI * 72
const dash = computed(() => {
  const part = Math.max(0, Math.min(100, props.percent)) / 100 * circ
  return `${part} ${circ}`
})
const fail = computed(() => Math.max(0, 100 - Math.min(100, props.percent)))
const failDash = computed(() => `${fail.value / 100 * circ} ${circ}`)
const failOffset = computed(() => `${-(Math.min(100, props.percent) / 100 * circ)}`)
</script>

<style scoped>
.chart { width: 100%; height: 220px; }
</style>
