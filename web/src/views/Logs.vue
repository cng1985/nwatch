<template>
  <div>
    <div class="page-head">
      <div>
        <h1>运行日志</h1>
        <p>查看 NMonitor 进程日志。服务重启后仍保留最近 7 天。</p>
      </div>
      <button class="btn ghost" @click="load">刷新</button>
    </div>
    <div class="toolbar">
      <select v-model="level" class="select" @change="load">
        <option value="">全部级别</option>
        <option value="DEBUG">DEBUG</option>
        <option value="INFO">INFO</option>
        <option value="WARN">WARN</option>
        <option value="ERROR">ERROR</option>
      </select>
      <input v-model="keyword" class="search" placeholder="搜索日志内容" @keyup.enter="load" />
      <label class="demo-note" style="display:flex;align-items:center;gap:6px"><input v-model="auto" type="checkbox" /> 自动刷新</label>
    </div>
    <div class="card table-wrap">
      <table>
        <thead><tr><th style="width:180px">时间</th><th style="width:80px">级别</th><th>内容</th></tr></thead>
        <tbody>
          <tr v-for="row in items" :key="row.id">
            <td>{{ clock(row.loggedAt) }}</td>
            <td><span class="level" :class="row.level">{{ row.level }}</span></td>
            <td>{{ row.message }}<small v-if="row.attrs" style="display:block;color:#7e8c98">{{ row.attrs }}</small></td>
          </tr>
          <tr v-if="!items.length"><td colspan="3" class="empty">这个条件下没有日志</td></tr>
        </tbody>
      </table>
      <div class="pager">
        <button class="btn ghost" :disabled="page === 1" @click="page--; load()">上一页</button>
        <span class="demo-note">{{ total }} 条</span>
        <button class="btn ghost" :disabled="page * 50 >= total" @click="page++; load()">下一页</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { api } from '../api'
import { clock, isDemo } from '../format'

const items = ref([])
const total = ref(0)
const page = ref(1)
const level = ref('')
const keyword = ref('')
const auto = ref(true)

async function load() {
  if (isDemo()) {
    items.value = [
      { id: 1, level: 'INFO', message: 'NMonitor 启动', attrs: 'version=1.0.0', loggedAt: new Date().toISOString() },
      { id: 2, level: 'WARN', message: '正在使用默认管理员密码', attrs: '', loggedAt: new Date(Date.now() - 60000).toISOString() },
    ]
    total.value = items.value.length
    return
  }
  const params = new URLSearchParams({ page: String(page.value), pageSize: '50' })
  if (level.value) params.set('level', level.value)
  if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
  const data = await api('/api/logs?' + params.toString())
  items.value = data.items || []
  total.value = data.total || 0
}

watch(keyword, () => { page.value = 1 })
let timer
onMounted(() => {
  load()
  timer = setInterval(() => { if (auto.value) load() }, 5000)
})
onUnmounted(() => clearInterval(timer))
</script>
