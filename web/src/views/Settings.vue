<template>
  <div>
    <div class="page-head"><div><h1>系统配置</h1><p>保留策略、默认检测参数、邮件通知和备份。</p></div></div>
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
    <form class="card section" @submit.prevent="saveMail">
      <h3>邮件服务器</h3>
      <p class="demo-note" style="margin:-6px 0 14px">监控连续失败达到阈值后发送邮件。服务一直异常时，每次检测失败都会再发一封；某一封发送失败会持续重试，直到成功。</p>
      <div class="form-grid">
        <label class="check full"><input v-model="mail.enabled" type="checkbox" /> 启用邮件通知</label>
        <label class="field"><span>SMTP 主机</span><input v-model="mail.host" class="text" placeholder="smtp.example.com" /></label>
        <label class="field"><span>端口</span><input v-model.number="mail.port" class="text" type="number" min="1" max="65535" /></label>
        <label class="field"><span>加密方式</span>
          <select v-model="mail.encryption" class="select" style="width:100%">
            <option value="starttls">STARTTLS</option>
            <option value="ssl">SSL</option>
            <option value="none">不加密</option>
          </select>
        </label>
        <label class="field"><span>用户名</span><input v-model="mail.username" class="text" placeholder="没有可留空" autocomplete="off" /></label>
        <label class="field"><span>密码</span><input v-model="mail.password" class="text" type="password" autocomplete="new-password" :placeholder="mail.passwordSet ? '已配置，留空表示不修改' : '没有可留空'" /></label>
        <label class="field"><span>发件人</span><input v-model="mail.from" class="text" placeholder="nmonitor@example.com" /></label>
        <label class="field full"><span>收件人</span><input v-model="mail.to" class="text" placeholder="多个地址用逗号分隔" /></label>
      </div>
      <p v-if="mailError" class="error">{{ mailError }}</p>
      <p v-if="mailMessage" class="demo-note" style="color:#12915f">{{ mailMessage }}</p>
      <div class="head-actions" style="padding-bottom:12px">
        <button class="btn primary" type="submit">保存邮件配置</button>
        <button class="btn ghost" type="button" @click="testMail">发送测试邮件</button>
      </div>
    </form>
  </div>
</template>
<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api, download } from '../api'
import { isDemo } from '../format'
const form = reactive({ timezone: 'Asia/Shanghai', defaultInterval: 60, defaultTimeout: 5, checkRetentionDays: 7, eventRetentionDays: 365, metricRetentionDays: 90, hourMetricRetentionDays: 730, workerCount: 20, version: '1.0.0' })
const mail = reactive({ enabled: false, host: '', port: 587, username: '', password: '', passwordSet: false, from: '', to: '', encryption: 'starttls' })
const password = reactive({ oldPassword: '', newPassword: '' })
const error = ref('')
const mailError = ref('')
const mailMessage = ref('')
const fileRef = ref()
function applyMail(src, keepPassword = false) {
  if (!src) return
  const typed = mail.password
  Object.assign(mail, src)
  mail.password = keepPassword ? typed : ''
}
async function load() {
  if (isDemo()) return
  const data = await api('/api/settings')
  Object.assign(form, data)
  applyMail(data.mail)
}
async function save() {
  if (isDemo()) { error.value = '演示模式不会写入服务器'; return }
  try {
    const data = await api('/api/settings', { method: 'PUT', body: form })
    Object.assign(form, data)
    applyMail(data.mail, true)
    error.value = ''
  } catch (err) { error.value = err.message }
}
async function saveMail() {
  if (isDemo()) { mailError.value = '演示模式不会写入服务器'; return }
  try {
    const data = await api('/api/settings/mail', { method: 'PUT', body: mail })
    Object.assign(form, data)
    applyMail(data.mail)
    mailError.value = ''
    mailMessage.value = '邮件配置已保存'
  } catch (err) { mailMessage.value = ''; mailError.value = err.message }
}
async function testMail() {
  if (isDemo()) { mailError.value = '演示模式不会连接邮件服务器'; return }
  try {
    await api('/api/settings/mail/test', { method: 'POST', body: mail })
    mailError.value = ''
    mailMessage.value = '测试邮件已发送'
  } catch (err) { mailMessage.value = ''; mailError.value = err.message }
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
