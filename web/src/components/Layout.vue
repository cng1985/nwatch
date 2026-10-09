<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="side-brand">
        <div class="logo-sq"><Icon name="wave" /></div>
        <div><strong>NMonitor</strong><small>轻量级服务监控</small></div>
      </div>
      <div v-for="group in groups" :key="group.label">
        <div class="nav-label">{{ group.label }}</div>
        <router-link v-for="item in group.items" :key="item.to" :to="item.to" class="nav-item" :class="{ active: route.meta.nav === item.to }">
          <Icon :name="item.icon" /> {{ item.label }}
          <i v-if="item.badge && alerts" class="badge">{{ alerts }}</i>
        </router-link>
      </div>
      <div class="side-foot">
        <div class="run-state"><i class="dot"></i> {{ healthy ? '系统运行正常' : '系统状态未知' }}</div>
        <div class="ver">v1.0.0</div>
        <div class="user-row">
          <div class="avatar"><Icon name="user" /></div>
          <div><b>管理员</b><small>{{ username }}</small></div>
          <button title="退出" @click="logout"><Icon name="logout" /></button>
        </div>
      </div>
    </aside>
    <section class="main">
      <header class="topbar">
        <div class="crumbs">
          <span v-for="(item, index) in crumbs" :key="item">
            <b v-if="index === crumbs.length - 1">{{ item }}</b>
            <template v-else>{{ item }} / </template>
          </span>
        </div>
        <div class="top-tools">
          <span>{{ todayLabel() }}</span>
          <router-link to="/events" class="bell"><Icon name="bell" /><i v-if="alerts">{{ alerts }}</i></router-link>
        </div>
      </header>
      <div class="content"><router-view /></div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Icon from './Icon.vue'
import { api } from '../api'
import { isDemo, todayLabel } from '../format'
import { demoDashboard } from '../demo'

const route = useRoute()
const router = useRouter()
const username = ref(localStorage.getItem('nmonitor_user') || 'admin')
const alerts = ref(isDemo() ? demoDashboard.openAlerts : 0)
const healthy = ref(true)
const crumbs = computed(() => route.meta.crumbs || ['工作空间'])
const groups = [
  { label: '工作空间', items: [
    { to: '/', label: '系统总览', icon: 'home' },
    { to: '/monitors', label: '监控列表', icon: 'list' },
    { to: '/certificates', label: 'HTTPS 证书', icon: 'cert' },
  ] },
  { label: '告警与通知', items: [
    { to: '/events', label: '告警事件', icon: 'bell', badge: true },
    { to: '/notifiers', label: '通知渠道', icon: 'chat' },
    { to: '/notification-logs', label: '通知日志', icon: 'ticket' },
  ] },
  { label: '管理', items: [
    { to: '/groups', label: '监控分组', icon: 'folder' },
    { to: '/settings', label: '系统配置', icon: 'settings' },
  ] },
]

onMounted(async () => {
  if (isDemo()) return
  try {
    const me = await api('/api/auth/me')
    username.value = me.username
    const dash = await api('/api/dashboard')
    alerts.value = dash.openAlerts || 0
    const health = await fetch('/health').then((res) => res.json())
    healthy.value = health.status === 'UP'
  } catch { /* 忽略侧栏刷新失败 */ }
})

function logout() {
  localStorage.removeItem('nmonitor_token')
  localStorage.removeItem('nmonitor_demo')
  router.push('/login')
}
</script>
