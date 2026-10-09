<template>
  <div>
    <div class="page-head">
      <div><h1>通知渠道</h1><p>钉钉、企业微信和 Webhook 彼此独立，发送失败不会影响监控状态。</p></div>
      <button class="btn primary" @click="open()">新增渠道</button>
    </div>
    <div class="card table-wrap">
      <table>
        <thead><tr><th>名称</th><th>类型</th><th>Webhook</th><th>密钥</th><th></th></tr></thead>
        <tbody>
          <tr v-for="row in items" :key="row.id">
            <td><b>{{ row.name }}</b></td>
            <td>{{ { dingtalk: '钉钉', wecom: '企业微信', webhook: 'Webhook' }[row.type] }}</td>
            <td>{{ row.webhookUrl || '演示渠道' }}</td>
            <td>{{ row.secretSet ? '已配置' : '-' }}</td>
            <td class="actions-inline"><button @click="test(row)">测试</button><button @click="open(row)">编辑</button><button class="danger" @click="remove(row)">删除</button></td>
          </tr>
          <tr v-if="!items.length"><td colspan="5" class="empty">还没有通知渠道</td></tr>
        </tbody>
      </table>
    </div>
    <div v-if="visible" class="modal-mask" @click.self="visible = false">
      <form class="modal stack" @submit.prevent="save">
        <h3>{{ form.id ? '编辑渠道' : '新增渠道' }}</h3>
        <input v-model="form.name" class="text" placeholder="名称" />
        <select v-model="form.type" class="select"><option value="dingtalk">钉钉</option><option value="wecom">企业微信</option><option value="webhook">Webhook</option></select>
        <input v-model="form.webhookUrl" class="text" placeholder="https:// Webhook 地址" />
        <input v-if="form.type === 'dingtalk'" v-model="form.secret" class="text" placeholder="加签密钥，留空表示不修改" />
        <p v-if="error" class="error">{{ error }}</p>
        <div class="head-actions"><button class="btn ghost" type="button" @click="visible = false">取消</button><button class="btn primary" type="submit">保存</button></div>
      </form>
    </div>
  </div>
</template>
<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import { demoNotifiers } from '../demo'
import { isDemo } from '../format'
const items = ref([])
const visible = ref(false)
const error = ref('')
const form = reactive({ id: 0, name: '', type: 'dingtalk', webhookUrl: '', secret: '', enabled: true })
async function load() { items.value = isDemo() ? demoNotifiers : await api('/api/notifiers') }
function open(row) { Object.assign(form, { id: 0, name: '', type: 'dingtalk', webhookUrl: '', secret: '', enabled: true }, row || {}); form.webhookUrl = row ? '' : form.webhookUrl; error.value = ''; visible.value = true }
async function save() {
  if (isDemo()) { error.value = '演示模式不会写入服务器'; return }
  try {
    const body = { ...form }
    if (!body.webhookUrl) delete body.webhookUrl
    if (form.id) await api('/api/notifiers/' + form.id, { method: 'PUT', body })
    else await api('/api/notifiers', { method: 'POST', body })
    visible.value = false
    await load()
  } catch (err) { error.value = err.message }
}
async function test(row) {
  if (isDemo()) return
  try { await api('/api/notifiers/' + row.id + '/test', { method: 'POST' }); alert('测试通知已发送') } catch (err) { alert(err.message) }
}
async function remove(row) {
  if (isDemo() || !confirm('删除渠道「' + row.name + '」？')) return
  await api('/api/notifiers/' + row.id, { method: 'DELETE' })
  await load()
}
onMounted(load)
</script>
