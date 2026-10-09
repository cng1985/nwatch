<template>
  <div>
    <div class="page-head"><div><h1>系统配置</h1><p>保留策略、默认检测参数和备份。</p></div></div>
    <div class="dash-grid">
      <form class="card section" @submit.prevent="save">
        <h3>运行参数</h3>
        <div class="stack">
          <label class="field"><span>时区</span><input v-model="form.timezone" class="text" /></label>
          <label class="field"><span>默认检测频率（秒）</span><input v-model.number="form.defaultInterval" class="text" type="number" /></label>
          <label class="field"><span>默认超时（秒）</span><input v-model.number="form.defaultTimeout" class="text" type="number" /></label>
          <label class="field"><span>检测记录保留（天）</span><input v-model.number="form.checkRetentionDays" class="text" type="number" /></label>
          <label class="field"><span>事件保留（天）</span><input v-model.number="form.eventRetentionDays" class="text" type="number" /></label>
          <label class="field"><span>分钟统计保留（天）</span><input v-model.number="form.metricRetentionDays" class="text" type="number" /></label>
          <label class="field"><span>小时统计保留（天）</span><input v-model.number="form.hourMetricRetentionDays" class="text" type="number" /></label>
          <p class="demo-note">工作线程 {{ form.workerCount || 20 }} · 版本 {{ form.version || '1.0.0' }} · {{ form.database }}</p>
          <p v-if="error" class="error">{{ error }}</p>
          <button class="btn primary" style="width:120px">保存配置</button>
        </div>
      </form>
      <div>
        <form class="card section" @submit.prevent="changePassword">
          <h3>修改密码</h3>
          <div class="stack">
            <input v-model="password.oldPassword" class="text" type="password" placeholder="原密码" />
            <input v-model="password.newPassword" class="text" type="password" placeholder="新密码，至少 6 位" />
            <button class="btn primary" style="width:120px">更新密码</button>
          </div>
        </form>
        <div class="card section">
          <h3>备份与迁移</h3>
          <div class="head-actions">
            <button class="btn ghost" type="button" @click="backup">下载数据库备份</button>
            <button class="btn ghost" type="button" @click="exportConfig(false)">导出配置</button>
            <button class="btn ghost" type="button" @click="fileRef.click()">导入配置</button>
            <input ref="fileRef" hidden type="file" accept="application/json" @change="importConfig" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api, download } from '../api'
import { isDemo } from '../format'
const form = reactive({ timezone: 'Asia/Shanghai', defaultInterval: 60, defaultTimeout: 5, checkRetentionDays: 7, eventRetentionDays: 365, metricRetentionDays: 90, hourMetricRetentionDays: 730, workerCount: 20, version: '1.0.0' })
const password = reactive({ oldPassword: '', newPassword: '' })
const error = ref('')
const fileRef = ref()
async function load() { if (!isDemo()) Object.assign(form, await api('/api/settings')) }
async function save() {
  if (isDemo()) { error.value = '演示模式不会写入服务器'; return }
  try { Object.assign(form, await api('/api/settings', { method: 'PUT', body: form })); error.value = '' } catch (err) { error.value = err.message }
}
async function changePassword() {
  if (isDemo()) return
  try { await api('/api/account/password', { method: 'PUT', body: password }); password.oldPassword = ''; password.newPassword = '' } catch (err) { error.value = err.message }
}
async function backup() { if (!isDemo()) await download('/api/backup', 'nmonitor-backup.db') }
async function exportConfig(includeSecrets) {
  if (isDemo()) return
  const data = await api('/api/export?includeSecrets=' + includeSecrets)
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = 'nmonitor-config.json'
  link.click()
}
async function importConfig(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file || isDemo()) return
  await api('/api/import', { method: 'POST', body: JSON.parse(await file.text()) })
}
onMounted(load)
</script>
