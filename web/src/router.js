import { createRouter, createWebHistory } from 'vue-router'
import Login from './views/Login.vue'
import Layout from './components/Layout.vue'
import Dashboard from './views/Dashboard.vue'
import Resources from './views/Resources.vue'
import Services from './views/Services.vue'
import Scripts from './views/Scripts.vue'
import Cron from './views/Cron.vue'
import Logs from './views/Logs.vue'
import Monitors from './views/Monitors.vue'
import MonitorEdit from './views/MonitorEdit.vue'
import MonitorDetail from './views/MonitorDetail.vue'
import Certificates from './views/Certificates.vue'
import Notifiers from './views/Notifiers.vue'
import NotifyLogs from './views/NotifyLogs.vue'
import Events from './views/Events.vue'
import Groups from './views/Groups.vue'
import Access from './views/Access.vue'
import Settings from './views/Settings.vue'
import { isDemo } from './format'
import { can, loadSession, session } from './session'
import { firstAllowed } from './nav'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login },
    {
      path: '/',
      component: Layout,
      children: [
        { path: '', component: Dashboard, meta: { crumbs: ['工作空间', '系统总览'], nav: '/', perm: 'dashboard.read' } },
        { path: 'resources', component: Resources, meta: { crumbs: ['工作空间', '主机资源'], nav: '/resources', perm: 'host.read' } },
        { path: 'services', component: Services, meta: { crumbs: ['工作空间', '系统服务'], nav: '/services', perm: 'service.read' } },
        { path: 'scripts', component: Scripts, meta: { crumbs: ['工作空间', '脚本执行'], nav: '/scripts', perm: 'script.read' } },
        { path: 'cron', component: Cron, meta: { crumbs: ['工作空间', '定时任务'], nav: '/cron', perm: 'cron.read' } },
        { path: 'shell', component: () => import('./views/Shell.vue'), meta: { crumbs: ['工作空间', '在线终端'], nav: '/shell', perm: 'shell.use' } },
        { path: 'monitors', component: Monitors, meta: { crumbs: ['工作空间', '监控列表'], nav: '/monitors', perm: 'monitor.read' } },
        { path: 'monitors/new', component: MonitorEdit, meta: { crumbs: ['工作空间', '监控列表', '新增监控'], nav: '/monitors', perm: 'monitor.write' } },
        { path: 'monitors/:id/edit', component: MonitorEdit, meta: { crumbs: ['工作空间', '监控列表', '编辑监控'], nav: '/monitors', perm: 'monitor.write' } },
        { path: 'monitors/:id', component: MonitorDetail, meta: { crumbs: ['工作空间', '监控列表', '监控详情'], nav: '/monitors', perm: 'monitor.read' } },
        { path: 'certificates', component: Certificates, meta: { crumbs: ['工作空间', 'HTTPS 证书'], nav: '/certificates', perm: 'cert.read' } },
        { path: 'events', component: Events, meta: { crumbs: ['告警与通知', '告警事件'], nav: '/events', perm: 'event.read' } },
        { path: 'notifiers', component: Notifiers, meta: { crumbs: ['告警与通知', '通知渠道'], nav: '/notifiers', perm: 'notifier.read' } },
        { path: 'notification-logs', component: NotifyLogs, meta: { crumbs: ['告警与通知', '通知日志'], nav: '/notification-logs', perm: 'notifier.read' } },
        { path: 'groups', component: Groups, meta: { crumbs: ['管理', '监控分组'], nav: '/groups', perm: 'group.read' } },
        { path: 'logs', component: Logs, meta: { crumbs: ['管理', '运行日志'], nav: '/logs', perm: 'log.read' } },
        { path: 'access', component: Access, meta: { crumbs: ['管理', '用户与角色'], nav: '/access', perm: 'access.read' } },
        { path: 'settings', component: Settings, meta: { crumbs: ['管理', '系统配置'], nav: '/settings', perm: 'settings.read' } },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const authed = !!localStorage.getItem('nmonitor_token') || isDemo()
  if (to.path !== '/login' && !authed) return '/login'
  if (to.path === '/login' && authed) return '/'
  if (to.path !== '/login' && authed && !session.loaded) {
    try {
      await loadSession()
    } catch {
      return '/login'
    }
  }
  if (to.meta.perm && !can(to.meta.perm)) {
    const next = firstAllowed(can)
    if (!next || next === to.path) return true
    return next
  }
  return true
})

export default router
