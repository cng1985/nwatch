# 接口约定

普通业务 JSON API 成功与应用错误均返回 HTTP 200；网络与代理错误仍可能是真实非 200。前端按外壳校验，不把任意 JSON 视为成功。

## 单条结果

```json
{"code":200,"msg":"success","data":{...}}
```

## 分页结果（原样输出，无 code/msg）

```json
{"data":[...],"total":123,"page":1,"page_size":20}
```

- `page` 默认 1，`page_size` 默认 20、上限 200；超出深分页边界返回参数错误。
- 分页失败为错误 Response：`{"code":400,"msg":"..."}`。

## 错误

```json
{"code":400,"msg":"参数错误说明"}
```

应用码与 HTTP 状态码分开：400/401/403/404/409/413/429/500/503。401 会让前端清除令牌并跳转登录。

## 端点清单

| 分组 | 端点 |
| --- | --- |
| 认证 | POST /api/auth/login（公开）、GET /api/auth/me、PUT /api/account/password |
| 权限 | GET /api/access/permissions、/api/access/users、/api/access/users/:id、/api/access/roles、/api/access/roles/:id |
| 总览 | GET /api/dashboard |
| 监控 | GET/POST /api/monitors、GET/PUT/DELETE /api/monitors/:id、POST .../:id/enable|disable|check、GET .../:id/checks|metrics|events、GET /api/events、GET /api/certificates |
| 分组 | GET/POST /api/groups、PUT/DELETE /api/groups/:id |
| 主机 | GET /api/host、/api/host/processes、/api/monitors/processes |
| 脚本 | POST /api/scripts/run、GET /api/scripts/runs、/api/scripts/runs/:id |
| 通知 | GET/POST /api/notifiers、PUT/DELETE /api/notifiers/:id、POST .../:id/test、GET /api/notification-logs |
| 定时任务 | GET/POST /api/cron-jobs、GET/PUT/DELETE /api/cron-jobs/:id、POST .../:id/enable|disable|run、GET .../:id/runs、GET /api/cron-runs、GET /api/cron/preview |
| 运维 | GET /api/services[.../:name/logs]、POST /api/services/:name/start|stop|restart、/api/docker/*、/api/kubernetes/*、GET /api/shell、/api/shell/commands、POST /api/shell/ticket、GET /api/shell/ws（票据） |
| 系统 | GET/PUT /api/settings、PUT /api/settings/mail、POST /api/settings/mail/test、GET/POST /api/backup、GET /api/export、POST /api/import、GET /api/logs |
| 健康 | GET /health/live、/health/ready、/health、/version（公开） |

表格导入导出为文件传输：GET /api/monitors/export、/api/monitors/requests/export 返回 xlsx 字节流；POST .../import 为 multipart 上传（单文件 ≤8MB）。
未知 API 返回 `{"code":404,"msg":"接口不存在"}`，不回退 HTML；SPA 回退只用于页面路由。
