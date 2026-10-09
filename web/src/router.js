import { createRouter, createWebHistory } from 'vue-router'
import Login from './views/Login.vue'
import Layout from './components/Layout.vue'
import Dashboard from './views/Dashboard.vue'
import Monitors from './views/Monitors.vue'
import MonitorEdit from './views/MonitorEdit.vue'
import MonitorDetail from './views/MonitorDetail.vue'
import Certificates from './views/Certificates.vue'
import Notifiers from './views/Notifiers.vue'
import NotifyLogs from './views/NotifyLogs.vue'
import Events from './views/Events.vue'
import Groups from './views/Groups.vue'
import Settings from './views/Settings.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login },
    {
      path: '/',
      component: Layout,
      children: [
        { path: '', component: Dashboard, meta: { crumbs: ['工作空间', '系统总览'], nav: '/' } },
        { path: 'monitors', component: Monitors, meta: { crumbs: ['工作空间', '监控列表'], nav: '/monitors' } },
        { path: 'monitors/new', component: MonitorEdit, meta: { crumbs: ['工作空间', '监控列表', '新增监控'], nav: '/monitors' } },
        { path: 'monitors/:id/edit', component: MonitorEdit, meta: { crumbs: ['工作空间', '监控列表', '编辑监控'], nav: '/monitors' } },
        { path: 'monitors/:id', component: MonitorDetail, meta: { crumbs: ['工作空间', '监控列表', '监控详情'], nav: '/monitors' } },
        { path: 'certificates', component: Certificates, meta: { crumbs: ['工作空间', 'HTTPS 证书'], nav: '/certificates' } },
        { path: 'events', component: Events, meta: { crumbs: ['告警与通知', '告警事件'], nav: '/events' } },
        { path: 'notifiers', component: Notifiers, meta: { crumbs: ['告警与通知', '通知渠道'], nav: '/notifiers' } },
        { path: 'notification-logs', component: NotifyLogs, meta: { crumbs: ['告警与通知', '通知日志'], nav: '/notification-logs' } },
        { path: 'groups', component: Groups, meta: { crumbs: ['管理', '监控分组'], nav: '/groups' } },
        { path: 'settings', component: Settings, meta: { crumbs: ['管理', '系统配置'], nav: '/settings' } },
      ],
    },
  ],
})

router.beforeEach((to) => {
  const authed = !!localStorage.getItem('nmonitor_token') || localStorage.getItem('nmonitor_demo') === '1'
  if (to.path !== '/login' && !authed) return '/login'
  if (to.path === '/login' && authed) return '/'
  return true
})

export default router
