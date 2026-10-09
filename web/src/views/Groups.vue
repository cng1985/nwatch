<template>
  <div>
    <div class="page-head"><div><h1>监控分组</h1><p>按环境或系统把监控归类。</p></div><button class="btn primary" @click="open()">新增分组</button></div>
    <div class="card table-wrap">
      <table>
        <thead><tr><th>名称</th><th>说明</th><th>排序</th><th>监控数</th><th></th></tr></thead>
        <tbody>
          <tr v-for="row in items" :key="row.id"><td><b>{{ row.name }}</b></td><td>{{ row.description }}</td><td>{{ row.sort }}</td><td>{{ row.monitorCount ?? '-' }}</td><td class="actions-inline"><button @click="open(row)">编辑</button><button class="danger" @click="remove(row)">删除</button></td></tr>
          <tr v-if="!items.length"><td colspan="5" class="empty">还没有分组</td></tr>
        </tbody>
      </table>
    </div>
    <div v-if="visible" class="modal-mask" @click.self="visible = false">
      <form class="modal stack" @submit.prevent="save">
        <h3>{{ form.id ? '编辑分组' : '新增分组' }}</h3>
        <input v-model="form.name" class="text" placeholder="名称" />
        <input v-model="form.description" class="text" placeholder="说明" />
        <input v-model.number="form.sort" class="text" type="number" placeholder="排序" />
        <p v-if="error" class="error">{{ error }}</p>
        <div class="head-actions"><button class="btn ghost" type="button" @click="visible = false">取消</button><button class="btn primary">保存</button></div>
      </form>
    </div>
  </div>
</template>
<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import { demoGroups } from '../demo'
import { isDemo } from '../format'
const items = ref([])
const visible = ref(false)
const error = ref('')
const form = reactive({ id: 0, name: '', description: '', sort: 0 })
async function load() { items.value = isDemo() ? demoGroups : await api('/api/groups') }
function open(row) { Object.assign(form, { id: 0, name: '', description: '', sort: 0 }, row || {}); error.value = ''; visible.value = true }
async function save() {
  if (isDemo()) { error.value = '演示模式不会写入服务器'; return }
  try {
    if (form.id) await api('/api/groups/' + form.id, { method: 'PUT', body: form })
    else await api('/api/groups', { method: 'POST', body: form })
    visible.value = false
    await load()
  } catch (err) { error.value = err.message }
}
async function remove(row) {
  if (isDemo() || !confirm('删除分组「' + row.name + '」？')) return
  await api('/api/groups/' + row.id, { method: 'DELETE' })
  await load()
}
onMounted(load)
</script>
