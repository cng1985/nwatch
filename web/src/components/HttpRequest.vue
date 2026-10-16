<template>
  <div class="http-request">
    <div class="field">
      <span>请求 Body 类型</span>
      <div class="seg" role="group" aria-label="请求 Body 类型">
        <button v-for="item in bodyTypes" :key="item.id" type="button" :class="{ on: bodyType === item.id }" @click="pickType(item.id)">{{ item.label }}</button>
      </div>
      <p class="demo-note">{{ bodyHint }}</p>
    </div>

    <label class="field">
      <span class="req">{{ urlLabel }}</span>
      <input v-model="url" class="text" :placeholder="placeholder" />
    </label>

    <div class="form-grid">
      <label class="field">
        <span class="req">请求方法</span>
        <select class="select" style="width:100%" :value="method || 'GET'" @change="onMethod">
          <option v-for="item in methods" :key="item" :value="item">{{ item }}</option>
        </select>
      </label>
    </div>

    <div class="field">
      <span>请求头</span>
      <div class="req-box">
        <div class="req-box-title">请求头</div>
        <div class="req-tabs" role="tablist">
          <button type="button" :class="{ on: headerMode === 'table' }" @click="setHeaderMode('table')">表格</button>
          <button type="button" :class="{ on: headerMode === 'text' }" @click="setHeaderMode('text')">文本</button>
        </div>
        <template v-if="headerMode === 'table'">
          <div class="kv-head"><span>KEY</span><span>VALUE</span><span></span></div>
          <div v-for="(row, index) in headerRows" :key="index" class="kv-row">
            <input v-model="row.key" class="text" placeholder="键" @input="onHeaderRow" />
            <input v-model="row.value" class="text" placeholder="值" @input="onHeaderRow" />
            <button type="button" class="icon-btn" aria-label="删除请求头" @click="removeHeader(index)"><Icon name="close" /></button>
          </div>
          <div class="req-add-row">
            <button type="button" class="req-add" @click="addHeader"><Icon name="plus" /> 添加请求头</button>
          </div>
        </template>
        <div v-else class="req-pad">
          <textarea v-model="headerText" class="text req-code" rows="5" placeholder="Authorization: Bearer token" @input="onHeaderText"></textarea>
          <p v-if="headerError" class="error">{{ headerError }}</p>
        </div>
      </div>
    </div>

    <div v-if="showBody" class="field">
      <span>请求 Body</span>
      <div class="req-box">
        <div class="req-box-title">请求体</div>
        <div class="req-tabs" role="tablist">
          <button type="button" :class="{ on: bodyView === 'edit' }" @click="setBodyView('edit')">{{ editLabel }}</button>
          <button type="button" :class="{ on: bodyView === 'preview' }" @click="setBodyView('preview')">预览</button>
          <button v-if="bodyType !== 'text'" type="button" :class="{ on: bodyView === 'text' }" @click="setBodyView('text')">文本</button>
        </div>
        <div v-if="bodyView === 'preview'" class="req-pad">
          <pre class="req-preview">{{ preview || '（空）' }}</pre>
        </div>
        <template v-else-if="bodyView === 'edit' && isPairs">
          <div class="kv-head"><span>KEY</span><span>VALUE</span><span></span></div>
          <div v-for="(row, index) in pairs" :key="index" class="kv-row">
            <input v-model="row.key" class="text" placeholder="键" @input="onPairs" />
            <input v-model="row.value" class="text" placeholder="值" @input="onPairs" />
            <button type="button" class="icon-btn" aria-label="删除字段" @click="removePair(index)"><Icon name="close" /></button>
          </div>
          <div class="req-add-row">
            <button type="button" class="req-add" @click="addPair"><Icon name="plus" /> 添加字段</button>
          </div>
        </template>
        <div v-else class="req-pad">
          <textarea v-model="bodyText" class="text req-code" rows="8" :placeholder="editorPlaceholder" @input="onBodyText"></textarea>
        </div>
        <p v-if="note" class="req-note">{{ note }}</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'
import { bodyTypes, headersToText, parsePairs, pairsToText, previewBody, previewNote, textToHeaders } from './httpRequest.js'

const url = defineModel('url', { type: String, default: '' })
const method = defineModel('method', { type: String, default: 'GET' })
const bodyType = defineModel('bodyType', { type: String, default: '' })
const headers = defineModel('headers', { type: Object, default: () => ({}) })
const body = defineModel('body', { type: String, default: '' })

const props = defineProps({
  urlLabel: { type: String, default: '请求地址' },
  placeholder: { type: String, default: 'https://example.com/hook' },
  methods: { type: Array, default: () => ['GET', 'POST'] },
})

const headerMode = ref('table')
const headerError = ref('')
const headerRows = ref(rowsFrom(headers.value))
const headerText = ref(headersToText(headers.value))
const bodyView = ref('edit')
const bodyText = ref(body.value || '')
const pairs = ref(filledPairs(body.value))
let headerSignature = headersToText(headers.value)
let bodySignature = body.value || ''

const showBody = computed(() => method.value === 'POST')
const bodyHint = computed(() => {
  const rest = props.methods.filter((item) => item !== 'POST')
  if (!rest.length) return '选择类型后按 POST 发送。'
  return `选择类型后按 POST 发送。${rest.join('、')} 不带请求体。`
})
const isPairs = computed(() => bodyType.value === 'form' || bodyType.value === 'formdata')
const editLabel = computed(() => ({ json: 'JSON', form: '表格', formdata: '表格', xml: 'XML', text: '文本' }[bodyType.value] || '文本'))
const preview = computed(() => previewBody(bodyType.value, body.value))
const note = computed(() => previewNote(bodyType.value, body.value))
const editorPlaceholder = computed(() => {
  if (bodyType.value === 'json') return '{\n  // 在此输入 JSON格式的请求体\n}'
  if (bodyType.value === 'xml') return '<?xml version="1.0" encoding="UTF-8"?>\n<root></root>'
  if (isPairs.value) return 'name=nmonitor\nok=1'
  return '在此输入文本请求体'
})

watch(() => headersToText(headers.value), () => {
  const next = headersToText(headers.value)
  if (next === headerSignature) return
  headerSignature = next
  headerText.value = next
  headerRows.value = rowsFrom(headers.value)
  headerError.value = ''
})

watch(() => body.value || '', (value) => {
  if (value === bodySignature) return
  bodySignature = value
  bodyText.value = value
  pairs.value = filledPairs(value)
})

function rowsFrom(value) {
  const rows = Object.entries(value || {}).map(([key, item]) => ({ key, value: item }))
  if (!rows.length) rows.push({ key: '', value: '' })
  return rows
}

function filledPairs(value) {
  const parsed = parsePairs(value)
  return parsed.length ? parsed : [{ key: '', value: '' }]
}

function pushHeaders(next) {
  headerSignature = headersToText(next)
  headers.value = next
}

function pushBody(next) {
  const value = next || ''
  bodySignature = value
  if (bodyText.value !== value) bodyText.value = value
  body.value = value
}

function onHeaderRow() {
  headerError.value = ''
  const next = {}
  for (const row of headerRows.value) {
    const key = row.key.trim()
    if (!key) continue
    next[key] = row.value
  }
  pushHeaders(next)
}

function onHeaderText() {
  try {
    pushHeaders(textToHeaders(headerText.value))
    headerError.value = ''
  } catch (err) {
    headerError.value = err.message
  }
}

function setHeaderMode(mode) {
  if (mode === headerMode.value) return
  if (headerMode.value === 'text' && headerError.value) return
  if (mode === 'text') headerText.value = headersToText(headers.value)
  else headerRows.value = rowsFrom(headers.value)
  headerMode.value = mode
}

function addHeader() {
  const last = headerRows.value[headerRows.value.length - 1]
  if (last && !last.key.trim() && !last.value.trim()) return
  headerRows.value.push({ key: '', value: '' })
}

function removeHeader(index) {
  headerRows.value.splice(index, 1)
  if (!headerRows.value.length) headerRows.value.push({ key: '', value: '' })
  onHeaderRow()
}

function onPairs() {
  pushBody(pairsToText(pairs.value))
}

function onBodyText() {
  pushBody(bodyText.value)
  if (isPairs.value) pairs.value = filledPairs(bodyText.value)
}

function addPair() {
  const last = pairs.value[pairs.value.length - 1]
  if (last && !last.key.trim() && !last.value.trim()) return
  pairs.value.push({ key: '', value: '' })
}

function removePair(index) {
  pairs.value.splice(index, 1)
  if (!pairs.value.length) pairs.value.push({ key: '', value: '' })
  onPairs()
}

function setBodyView(view) {
  if (view === 'text' || (view === 'edit' && !isPairs.value)) bodyText.value = body.value || ''
  if (view === 'edit' && isPairs.value) pairs.value = filledPairs(body.value)
  bodyView.value = view
}

function pickType(id) {
  const prev = bodyType.value
  const table = (item) => item === 'form' || item === 'formdata'
  if (prev && prev !== id && !(table(prev) && table(id))) {
    pushBody('')
    pairs.value = [{ key: '', value: '' }]
  }
  bodyType.value = id
  if (method.value !== 'POST') method.value = 'POST'
  bodyView.value = 'edit'
}

function onMethod(event) {
  const next = event.target.value
  method.value = next
  if (next === 'GET' || next === 'HEAD') {
    bodyType.value = ''
    pushBody('')
    pairs.value = [{ key: '', value: '' }]
    return
  }
  if (!bodyType.value) bodyType.value = 'json'
}

onMounted(() => {
  if (method.value === 'POST' && !bodyType.value) bodyType.value = body.value ? 'text' : 'json'
})

function validate() {
  if (headerMode.value === 'text') onHeaderText()
  return headerError.value
}

defineExpose({ validate })
</script>
