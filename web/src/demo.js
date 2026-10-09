const wave = [96, 102, 118, 140, 188, 230, 210, 176, 150, 132, 128, 146, 210, 168, 142, 130, 118, 124, 112, 108, 102, 98, 94, 90]

function atHour(index) {
  const start = new Date()
  start.setHours(0, 0, 0, 0)
  return new Date(start.getTime() + index * 3600000).toISOString()
}

function ago(seconds) {
  return new Date(Date.now() - seconds * 1000).toISOString()
}

export const demoDashboard = {
  total: 32,
  up: 28,
  down: 3,
  paused: 1,
  addedSinceYesterday: 2,
  availability24h: 99.93,
  hasAvailability: true,
  success24h: 45327,
  failure24h: 32,
  openAlerts: 3,
  problems: [
    { id: 'erp', name: 'ERP API' },
    { id: 'pay', name: '支付网关' },
    { id: 'file', name: '文件服务' },
  ],
  overview: [
    { id: 'erp', name: 'ERP API', type: 'http', url: 'https://erp.example.com/health', status: 'DOWN', groupId: 1, lastResponseTime: 5000, availability24h: 98.62, hasAvailability: true, ago: 10 },
    { id: 'pay', name: '支付网关', type: 'http', url: 'https://pay.example.com/health', status: 'DOWN', groupId: 1, lastResponseTime: 5000, availability24h: 99.12, hasAvailability: true, ago: 12 },
    { id: 'web', name: '官网', type: 'http', url: 'https://www.example.com', status: 'UP', groupId: 1, lastResponseTime: 86, availability24h: 100, hasAvailability: true, ago: 8 },
    { id: 'scm', name: 'SCM API', type: 'http', url: 'https://scm.example.com/health', status: 'UP', groupId: 1, lastResponseTime: 42, availability24h: 100, hasAvailability: true, ago: 15 },
    { id: 'mysql', name: 'MySQL 生产库', type: 'tcp', host: '192.168.1.100', port: 3306, status: 'UP', groupId: 3, lastResponseTime: 12, availability24h: 100, hasAvailability: true, ago: 6 },
  ],
  trend: {
    range: '24h',
    avg: 128,
    min: 24,
    max: 486,
    delta: -12,
    success: 45327,
    failure: 32,
    points: wave.map((avg, i) => ({ time: atHour(i), avg })),
  },
}

export const demoNotifiers = [
  { id: 1, name: '运维钉钉群', type: 'dingtalk' },
  { id: 2, name: '企业微信', type: 'wecom' },
  { id: 3, name: '自定义 Webhook', type: 'webhook' },
]

export const demoGroups = [
  { id: 1, name: '生产环境', description: '线上服务', sort: 1, monitorCount: 18 },
  { id: 2, name: '测试环境', description: '预发布与测试', sort: 2, monitorCount: 9 },
  { id: 3, name: '数据库', description: '端口与连接', sort: 3, monitorCount: 5 },
]

export const demoEvents = [
  { id: 1, occurredAt: new Date().toISOString(), monitorName: 'ERP API', eventType: 'MONITOR_DOWN', oldStatus: 'UP', newStatus: 'DOWN', target: 'https://erp.example.com/health', message: 'connection timeout', recoveredAt: null },
  { id: 2, occurredAt: new Date(Date.now() - 3600000).toISOString(), monitorName: '支付网关', eventType: 'MONITOR_DOWN', oldStatus: 'UP', newStatus: 'DOWN', target: 'https://pay.example.com/health', message: 'connection timeout', recoveredAt: null },
]

export const demoCerts = [
  { id: 'cert', name: 'api.example.com', url: 'api.example.com', certNotAfter: '2026-10-15T00:00:00Z', certDaysRemaining: 6, certIssuer: "Let's Encrypt", tlsStatus: 'CRITICAL' },
]

export function presentDemo() {
  return {
    ...demoDashboard,
    overview: demoDashboard.overview.map((row) => ({ ...row, lastCheckAt: ago(row.ago) })),
    trend: { ...demoDashboard.trend, points: demoDashboard.trend.points.map((point, i) => ({ ...point, time: atHour(i) })) },
  }
}

export function demoTrend(range) {
  if (range === '7d') {
    return { ...demoDashboard.trend, range, avg: 136, min: 28, max: 420, delta: -6, points: [110, 140, 180, 150, 120, 130, 100].map((avg, i) => ({ time: new Date(Date.now() - (6 - i) * 86400000).toISOString(), avg })) }
  }
  if (range === '30d') {
    return { ...demoDashboard.trend, range, avg: 142, min: 22, max: 510, delta: 3.2, points: Array.from({ length: 15 }, (_, i) => ({ time: new Date(Date.now() - (14 - i) * 2 * 86400000).toISOString(), avg: 90 + ((i * 37) % 80) })) }
  }
  return demoDashboard.trend
}
