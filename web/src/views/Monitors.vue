<template>
  <div>
    <div class="page-head">
      <div><h1>监控列表</h1><p>按状态查看全部监控，异常会排在最上方。</p></div>
      <router-link class="btn primary" to="/monitors/new"><Icon name="plus" style="width:16px;height:16px" /> 新增监控</router-link>
    </div>
    <div class="toolbar">
      <input v-model="query.keyword" class="search" placeholder="搜索名称或地址" @keyup.enter="load" />
      <select v-model="query.type" class="select" @change="load"><option value="">全部类型</option><option value="http">HTTP</option><option value="tls">证书</option><option value="tcp">TCP</option></select>
      <select v-model="query.status" class="select" @change="load"><option value="">全部状态</option><option value="UP">正常运行</option><option value="DOWN">服务异常</option><option value="PAUSED">已暂停</option><option value="UNKNOWN">等待检测</option></select>
      <button class="btn ghost" @click="load">查询</button>
    </div>
    <div class="card table-wrap">
      <table>
        <thead><tr><th>监控名称</th><th>状态</th><th>响应时间</th><th>24h 可用率</th><th>最近检测</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="row in items" :key="row.id">
            <td><router-link class="name-cell" :to="'/monitors/' + row.id"><span class="name-ico"><Icon :name="row.type === 'tcp' ? 'db' : row.type === 'tls' ? 'cert' : 'globe'" /></span><span><b>{{ row.name }}</b><small>{{ targetOf(row) }}</small></span></router-link></td>
            <td><span class="status" :class="row.status"><i class="mark" :class="row.status === 'DOWN' ? 'red' : row.status === 'UP' ? 'green' : 'gray'"></i>{{ statusLabel(row.status) }}</span></td>
            <td>{{ row.lastResponseTime ? row.lastResponseTime + ' ms' : '-' }}</td>
            <td><span class="bar" :class="{ bad: row.status === 'DOWN' }"><i :style="{ width: (row.hasAvailability ? row.availability24h : 0) + '%' }"></i></span><span class="pct">{{ row.hasAvailability ? Number(row.availability24h).toFixed(2) + '%' : '-' }}</span></td>
            <td>{{ relative(row.lastCheckAt) }}</td>
            <td class="actions-inline">
              <button @click="check(row)">检测</button>
              <router-link class="link" :to="'/monitors/' + row.id + '/edit'">编辑</router-link>
              <button class="danger" @click="remove(row)">删除</button>
            </td>
          </tr>
          <tr v-if="!items.length"><td colspan="6" class="empty">还没有监控</td></tr>
        </tbody>
      </table>
      <div class="pager">
        <button class="btn ghost" :disabled="page === 1" @click="page--; load()">上一页</button>
        <span class="demo-note">{{ total }} 项</span>
        <button class="btn ghost" :disabled="page * 20 >= total" @click="page++; load()">下一页</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import Icon from '../components/Icon.vue'
import { api } from '../api'
import { presentDemo } from '../demo'
import { isDemo, relative, statusLabel, targetOf } from '../format'

const items = ref([])
const total = ref(0)
const page = ref(1)
const query = reactive({ keyword: '', type: '', status: '' })

async function load() {
  if (isDemo()) {
    items.value = presentDemo().overview
    total.value = items.value.length
    return
  }
  const params = new URLSearchParams({ page: String(page.value), pageSize: '20' })
  for (const key of ['keyword', 'type', 'status']) if (query[key]) params.set(key, query[key])
  const data = await api('/api/monitors?' + params.toString())
  items.value = data.items || []
  total.value = data.total || 0
}
async function check(row) {
  if (isDemo()) return
  await api('/api/monitors/' + row.id + '/check', { method: 'POST' })
  await load()
}
async function remove(row) {
  if (isDemo() || !confirm('删除监控「' + row.name + '」？')) return
  await api('/api/monitors/' + row.id, { method: 'DELETE' })
  await load()
}
onMounted(load)
</script>
