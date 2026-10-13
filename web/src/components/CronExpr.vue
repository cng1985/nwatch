<template>
  <div class="cron-expr">
    <div class="cron-presets">
      <button v-for="item in cronPresets" :key="item.expr" type="button" :class="{ on: canonicalExpr(modelValue) === item.expr }" @click="emitValue(item.expr)">{{ item.label }}</button>
    </div>
    <label class="field">
      <span class="req">Cron 表达式</span>
      <input :value="modelValue" class="text" placeholder="0 2 * * *" spellcheck="false" @input="emitValue($event.target.value)" />
    </label>
    <p v-if="!parsed" class="cron-hint">表达式是 5 段：分 时 日 月 周。也可以使用 @hourly、@daily、@weekly、@monthly、@yearly。</p>
    <template v-else>
      <div class="cron-tabs">
        <button v-for="field in cronFields" :key="field.key" type="button" :class="{ on: active === field.key }" @click="active = field.key">
          {{ field.label }}
          <small>{{ parsed[indexOf(field.key)].raw }}</small>
        </button>
      </div>
      <div class="cron-panel">
        <div class="cron-modes">
          <button v-for="item in modes" :key="item.id" type="button" :class="{ on: current.mode === item.id }" @click="setMode(item.id)">{{ item.label }}</button>
        </div>
        <p class="cron-hint">{{ hint }}</p>
        <div v-if="current.mode === 'step' || current.mode === 'range'" class="cron-line">
          <span>从</span>
          <select class="cron-select" :value="current.start" @change="changeBound('start', $event.target.value)">
            <option v-for="value in numbers" :key="value" :value="value">{{ valueLabel(activeField, value) }}</option>
          </select>
          <template v-if="current.mode === 'range'">
            <span>到</span>
            <select class="cron-select" :value="current.end" @change="changeBound('end', $event.target.value)">
              <option v-for="value in numbers" :key="value" :value="value">{{ valueLabel(activeField, value) }}</option>
            </select>
          </template>
          <template v-else>
            <span>每</span>
            <input class="cron-num" type="number" :min="1" :max="activeField.max" :value="current.step" @change="changeStep($event.target.value)" />
            <span>{{ activeField.unit }}</span>
          </template>
        </div>
        <div v-else-if="current.mode === 'pick' || current.mode === 'custom'" class="cron-grid" :class="active">
          <button v-for="value in numbers" :key="value" type="button" :class="{ on: current.values.includes(value) }" @click="toggle(value)">{{ valueLabel(activeField, value) }}</button>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { canonicalExpr, cronFields, cronPresets, fieldByKey, parseExpression, replaceField, valueLabel } from './cronExpr.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue'])

const modes = [
  { id: 'every', label: '任意' },
  { id: 'step', label: '间隔' },
  { id: 'range', label: '范围' },
  { id: 'pick', label: '指定' },
]
const active = ref('minute')
const parsed = computed(() => parseExpression(props.modelValue))
const activeField = computed(() => fieldByKey(active.value))
const current = computed(() => parsed.value?.[indexOf(active.value)] || { mode: 'custom', raw: '', start: 0, end: 0, step: 1, values: [] })
const numbers = computed(() => {
  const field = activeField.value
  return Array.from({ length: field.max - field.min + 1 }, (_, index) => field.min + index)
})
const hint = computed(() => {
  const field = activeField.value
  const state = current.value
  if (state.mode === 'custom') return `「${field.label}」当前是 ${state.raw}，选择上面的方式会改写这一段`
  if (state.mode === 'every') return `不限制${field.unit}`
  if (state.mode === 'range') return `从 ${valueLabel(field, state.start)} 到 ${valueLabel(field, state.end)}`
  if (state.mode === 'step' && state.end < field.max) return `从 ${valueLabel(field, state.start)} 到 ${valueLabel(field, state.end)}，每 ${state.step} ${field.unit}`
  if (state.mode === 'step') return `从 ${valueLabel(field, state.start)} 开始，每 ${state.step} ${field.unit}`
  if (!state.values.length) return '至少选择一个'
  return `指定 ${state.values.map((value) => valueLabel(field, value)).join('、')}`
})

function indexOf(key) {
  return cronFields.findIndex((field) => field.key === key)
}

function emitValue(value) {
  if (value !== props.modelValue) emit('update:modelValue', value)
}

function write(state) {
  emitValue(replaceField(props.modelValue, active.value, state))
}

function setMode(mode) {
  const field = activeField.value
  const state = current.value
  if (mode === state.mode && mode !== 'custom') return
  if (mode === 'every') {
    write({ mode: 'every' })
    return
  }
  if (mode === 'step') {
    const start = state.mode === 'pick' ? state.values[0] : state.start
    const end = state.mode === 'range' ? state.end : field.max
    write({ mode: 'step', start: start ?? field.min, end, step: field.key === 'minute' ? 5 : 1, values: [] })
    return
  }
  if (mode === 'range') {
    const defaults = { minute: [0, 30], hour: [9, 18], dom: [1, 15], month: [1, 6], dow: [1, 5] }
    let start = defaults[field.key][0]
    let end = defaults[field.key][1]
    if (state.mode === 'pick' && state.values.length) {
      start = state.values[0]
      end = state.values[state.values.length - 1]
    } else if (state.mode === 'range' || state.mode === 'step') {
      start = state.start
      end = state.end
    }
    if (start === end) end = Math.min(field.max, start + 1)
    write({ mode: 'range', start, end, step: 1, values: [] })
    return
  }
  let values = [field.key === 'dow' ? 1 : field.min]
  if (state.mode === 'pick' && state.values.length) values = state.values
  else if (state.mode === 'range' || state.mode === 'step') {
    values = []
    for (let value = state.start; value <= state.end; value += state.step || 1) values.push(value)
    if (values.length > 16) values = [state.start]
  }
  write({ mode: 'pick', values, start: values[0], end: values[values.length - 1], step: 1 })
}

function changeBound(bound, raw) {
  const field = activeField.value
  const state = { ...current.value, values: current.value.values.slice() }
  let value = Number(raw)
  if (bound === 'start') {
    state.start = value
    if (state.end < value) state.end = value
  } else {
    state.end = value
    if (state.start > value) state.start = value
  }
  state.start = Math.min(field.max, Math.max(field.min, state.start))
  state.end = Math.min(field.max, Math.max(field.min, state.end))
  write(state)
}

function changeStep(raw) {
  const step = Math.max(1, Math.trunc(Number(raw) || 1))
  write({ ...current.value, mode: 'step', step, values: [] })
}

function toggle(value) {
  const selected = current.value.mode === 'pick' ? current.value.values.slice() : []
  const index = selected.indexOf(value)
  if (index >= 0) selected.splice(index, 1)
  else selected.push(value)
  if (!selected.length) return
  write({ mode: 'pick', values: selected, start: Math.min(...selected), end: Math.max(...selected), step: 1 })
}
</script>

<style scoped>
.cron-expr { display: grid; gap: 10px; }
.cron-presets, .cron-modes { display: flex; flex-wrap: wrap; gap: 8px; }
.cron-presets button, .cron-modes button, .cron-tabs button, .cron-grid button {
  border: 1px solid var(--line);
  background: white;
  color: #3d4d58;
}
.cron-presets button, .cron-modes button {
  height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 13px;
}
.cron-presets button.on, .cron-modes button.on, .cron-tabs button.on {
  border-color: var(--green);
  background: #f3fbf7;
  color: #0f9f6e;
}
.cron-tabs { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 8px; }
.cron-tabs button { border-radius: 10px; padding: 8px 6px; font-size: 13px; }
.cron-tabs small { display: block; margin-top: 2px; color: var(--muted); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cron-tabs button.on small { color: #0f9f6e; }
.cron-panel { display: grid; gap: 10px; }
.cron-hint { color: var(--muted); font-size: 12px; line-height: 1.6; }
.cron-line { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; color: #3d4d58; font-size: 13px; }
.cron-select, .cron-num {
  height: 36px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: white;
  padding: 0 10px;
}
.cron-num { width: 76px; }
.cron-grid { display: grid; grid-template-columns: repeat(10, minmax(0, 1fr)); gap: 6px; }
.cron-grid.hour { grid-template-columns: repeat(8, minmax(0, 1fr)); }
.cron-grid.dom, .cron-grid.dow { grid-template-columns: repeat(7, minmax(0, 1fr)); }
.cron-grid.month { grid-template-columns: repeat(6, minmax(0, 1fr)); }
.cron-grid button { height: 32px; border-radius: 8px; font-size: 12px; font-variant-numeric: tabular-nums; }
.cron-grid button.on { background: #0f9f6e; border-color: #0f9f6e; color: white; }
</style>
