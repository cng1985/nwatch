<template>
  <div class="login-page">
    <section class="login-hero">
      <div class="wordmark"><Icon name="wave" /> NMonitor</div>
      <h1>让每一次异常，<br />都被及时看见。</h1>
      <p class="lead">一站式监控服务可用性、响应时间与证书状态。<br />轻量部署，从容掌握每个服务的运行脉搏。</p>
      <div class="preview-card">
        <div class="preview-head"><span><i class="dot" style="margin-right:8px"></i>服务运行状态</span><span class="muted">最近 24 小时</span></div>
        <div v-for="row in samples" :key="row.name" class="preview-row">
          <i class="dot"></i>
          <b>{{ row.name }}</b>
          <span class="chip">{{ row.type }}</span>
          <span class="grow"></span>
          <span class="ok">正常</span>
          <span class="ms">{{ row.ms }} ms</span>
          <span class="spark"><i v-for="(h, i) in row.bars" :key="i" :style="{ height: h + 'px' }"></i></span>
        </div>
      </div>
      <div class="feature-row">
        <span><Icon name="check" /> HTTP / HTTPS / TCP</span>
        <span><Icon name="check" /> 实时异常通知</span>
        <span><Icon name="check" /> 证书到期预警</span>
      </div>
      <div class="login-foot">© 2026 NMonitor &nbsp;|&nbsp; 轻量运行 · 时刻在线</div>
    </section>
    <section class="login-panel">
      <form class="login-card" @submit.prevent="submit">
        <h2>欢迎回来</h2>
        <p class="sub">登录 NMonitor，查看您的服务运行状态。</p>
        <label class="field"><span>用户名</span>
          <div class="input"><Icon name="user" /><input v-model="username" placeholder="请输入用户名" autocomplete="username" /></div>
        </label>
        <label class="field"><span>密码</span>
          <div class="input">
            <Icon name="lock" />
            <input v-model="password" :type="show ? 'text' : 'password'" placeholder="请输入密码" autocomplete="current-password" />
            <button class="icon-btn" type="button" @click="show = !show"><Icon name="eye" /></button>
          </div>
        </label>
        <div class="row-between">
          <label class="check"><input v-model="remember" type="checkbox" /> 记住用户名</label>
          <button class="linkish" type="button" @click="username = 'admin'">管理员账号登录</button>
        </div>
        <p v-if="error" class="error">{{ error }}</p>
        <button class="btn primary block" :disabled="loading">登录控制台 →</button>
        <div class="or">或</div>
        <button class="btn ghost block" type="button" @click="demo"><Icon name="eye" /> 体验演示</button>
        <p class="hint">无需连接服务器，即可浏览页面设计。</p>
      </form>
      <div class="panel-foot">安全连接 · 本地部署</div>
    </section>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '../components/Icon.vue'
import { api } from '../api'

const router = useRouter()
const username = ref(localStorage.getItem('nmonitor_remember_user') || '')
const password = ref('')
const remember = ref(!!localStorage.getItem('nmonitor_remember_user'))
const show = ref(false)
const loading = ref(false)
const error = ref('')
const samples = [
  { name: '官网', type: 'HTTP', ms: 86, bars: [8, 14, 10, 16, 12, 18, 9, 15, 11, 17, 8, 14, 16, 10, 18, 12, 15, 9] },
  { name: 'SCM API', type: 'HTTPS', ms: 42, bars: [10, 16, 9, 15, 12, 18, 8, 14, 11, 17, 13, 16, 10, 18, 12, 15, 9, 14] },
  { name: 'MySQL', type: 'TCP', ms: 12, bars: [7, 12, 8, 15, 10, 16, 9, 13, 8, 17, 11, 14, 9, 16, 12, 15, 8, 13] },
]

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const data = await api('/api/auth/login', { method: 'POST', body: { username: username.value, password: password.value } })
    localStorage.removeItem('nmonitor_demo')
    localStorage.setItem('nmonitor_token', data.token)
    localStorage.setItem('nmonitor_user', data.username)
    localStorage.setItem('nmonitor_default_pw', data.usingDefaultPassword ? '1' : '0')
    if (remember.value) localStorage.setItem('nmonitor_remember_user', username.value)
    else localStorage.removeItem('nmonitor_remember_user')
    router.push('/')
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

function demo() {
  localStorage.removeItem('nmonitor_token')
  localStorage.setItem('nmonitor_demo', '1')
  localStorage.setItem('nmonitor_user', 'admin')
  router.push('/')
}
</script>
