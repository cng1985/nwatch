export const navGroups = [
  {
    label: '工作空间',
    items: [
      { to: '/', label: '系统总览', icon: 'home', perm: 'dashboard.read' },
      { to: '/resources', label: '主机资源', icon: 'cpu', perm: 'host.read' },
      { to: '/services', label: '系统服务', icon: 'service', perm: 'service.read' },
      { to: '/monitors', label: '监控列表', icon: 'list', perm: 'monitor.read' },
      { to: '/scripts', label: '脚本执行', icon: 'script', perm: 'script.read' },
      { to: '/cron', label: '定时任务', icon: 'clock', perm: 'cron.read' },
      { to: '/shell', label: '在线终端', icon: 'terminal', perm: 'shell.use' },
      { to: '/certificates', label: 'HTTPS 证书', icon: 'cert', perm: 'cert.read' },
    ],
  },
  {
    label: '告警与通知',
    items: [
      { to: '/events', label: '告警事件', icon: 'bell', perm: 'event.read', badge: true },
      { to: '/notifiers', label: '通知渠道', icon: 'chat', perm: 'notifier.read' },
      { to: '/notification-logs', label: '通知日志', icon: 'ticket', perm: 'notifier.read' },
    ],
  },
  {
    label: '管理',
    items: [
      { to: '/groups', label: '监控分组', icon: 'folder', perm: 'group.read' },
      { to: '/logs', label: '运行日志', icon: 'file', perm: 'log.read' },
      { to: '/access', label: '用户与角色', icon: 'user', perm: 'access.read' },
      { to: '/settings', label: '系统配置', icon: 'settings', perm: 'settings.read' },
    ],
  },
]

export function firstAllowed(can) {
  for (const group of navGroups) {
    for (const item of group.items) {
      if (can(item.perm)) return item.to
    }
  }
  return ''
}
