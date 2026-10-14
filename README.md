# NMonitor

面向 Linux 环境的轻量级服务可用性监控。回答的问题很具体：服务现在能不能访问，证书还有多久过期，出了问题有没有立刻通知到人。

默认只需要一个二进制、一个 SQLite 文件和一份配置：

```text
nmonitor
nmonitor.db
config.yaml
```

不依赖 MySQL、Redis 或消息队列。

## 能力

- HTTP/HTTPS 检查：方法、请求头、状态码、响应内容、超时和响应时间
- HTTPS 证书：到期时间、剩余天数、颁发机构、主机名和证书链
- TCP 端口连通性
- 本机 CPU、内存、磁盘：15 秒采样，1 小时到 7 天趋势；可按使用率阈值告警。进程可按 CPU 或内存排序查看
- 脚本执行：定时检查或在页面上手动执行，记录退出码和输出
- 定时任务：用 5 段 cron 表达式执行脚本，或发起 GET、POST 表单、POST JSON 请求。时间按系统配置里的时区计算，同一个任务不会重叠执行
- 在线终端：登录后在管理台打开本机交互式 Shell。Linux 使用 bash 或 sh，Windows 10 1809 及以上通过 ConPTY 使用 PowerShell 或 cmd
- 系统服务：在管理台列出本机 systemd 服务，启动、停止、重启，并用 journalctl 查看日志
- 运行日志：在管理台查看进程日志，保留 7 天
- 调度器每秒领取到期任务，固定大小的工作池执行，同一个监控不会重叠检查
- 连续失败后才标记为异常，恢复也可要求连续成功
- 钉钉（支持加签）、企业微信、Webhook，通知异步发送并记录结果
- 邮件和所有已启用的通知渠道：连续失败达到阈值后，15 秒通知一次，45 秒再通知一次，之后每 1 分钟通知一次，直到服务恢复。关闭的渠道不发送。某一封邮件发送失败会持续重试，直到成功
- 证书按 30/14/7/3/1/0 天阈值各提醒一次
- 检测历史、分钟/小时/天统计、1 小时到 30 天可用率
- Web 管理台：总览、主机资源、系统服务、监控、脚本执行、定时任务、在线终端、证书、告警、通知、运行日志、分组、用户与角色、系统配置
- 基于角色的访问控制：内置管理员、运维、只读，也可以自定义角色。一个用户可以有多个角色，菜单和接口都按权限控制
- 配置导入导出，SQLite 备份

## 代码结构

后端按功能分包。`cmd/server` 只启动进程，`internal/app` 用 fx 把各包自己的 Module 组装起来。前端是 Vue 3 管理台，`npm run build` 写到 `internal/web/dist`，再由 `internal/web` 嵌入二进制。

```text
cmd/server/main.go        进程入口
internal/app/             组装各功能模块并启动
internal/config/          读取 config.yaml 和环境变量
internal/database/        打开并关闭 SQLite
internal/model/           监控、告警、通知等数据模型
internal/settings/        运行时可改的系统配置
internal/checker/         HTTP、证书、TCP、资源和脚本检测
internal/monitor/         监控请求的校验和规范化
internal/state/           连续失败或成功后切换状态
internal/scheduler/       每秒领取到期检测，工作池执行
internal/metric/          按分钟、小时、天聚合检测结果
internal/dashboard/       总览和趋势查询
internal/alert/           异步通知队列，故障期间定时提醒
internal/notifier/        钉钉、企业微信、Webhook
internal/mailer/          邮件发送和失败重试
internal/cronjob/         cron 表达式、脚本和 HTTP 定时任务
internal/host/            本机 CPU、内存、磁盘和进程采集
internal/shell/           在线终端会话和一次性连接凭证
internal/svcctl/          调用 systemctl 和 journalctl
internal/logview/         写入并查询进程日志
internal/maintenance/     数据库备份和历史清理
internal/auth/            登录和 JWT
internal/rbac/            用户、角色和权限
internal/api/             HTTP 与 WebSocket 接口
internal/transfer/        配置导入导出
internal/web/             嵌入前端构建产物
internal/version/         版本号
web/                      Vue 3 管理台
  src/views/              总览、资源、服务、监控、证书、告警、通知、日志、用户与角色、配置等页面
  src/components/         布局、图表、cron 表达式
  src/api.js              接口调用
  src/router.js           路由和登录拦截
deploy/                   systemd 单元和安装脚本
config.yaml               默认配置
Dockerfile
Makefile
```

## 运行

```bash
make build
export NMONITOR_PASSWORD='your-password'
./nmonitor
```

浏览器打开 `http://127.0.0.1:8080`。未设置密码时，初始账号是 `admin` / `admin`，启动日志会提示尽快修改。

只编译后端时，需要先构建前端，产物会写到 `internal/web/dist` 并嵌入二进制：

```bash
cd web && npm install && npm run build
go build -o nmonitor ./cmd/server
```

## 配置

参见 `config.yaml`。密码不要写进文件，使用环境变量：

```bash
NMONITOR_PASSWORD
NMONITOR_JWT_SECRET
NMONITOR_CONFIG
NMONITOR_USERNAME
NMONITOR_SMTP_ENABLED
NMONITOR_SMTP_HOST
NMONITOR_SMTP_PORT
NMONITOR_SMTP_USERNAME
NMONITOR_SMTP_PASSWORD
NMONITOR_SMTP_FROM
NMONITOR_SMTP_TO
NMONITOR_SMTP_ENCRYPTION
NMONITOR_SHELL_ENABLED
NMONITOR_SERVICE_ENABLED
```

`NMONITOR_SMTP_ENCRYPTION` 可以是 `starttls`、`ssl` 或 `none`。收件人有多个时用逗号分隔。界面里保存的邮件配置会覆盖这里的环境变量。

## systemd

在已经编译好的机器上，用脚本安装并启动服务（需要 root，且系统使用 systemd）：

```bash
make build
sudo env NMONITOR_PASSWORD='your-password' ./deploy/install-service.sh
```

脚本把二进制和 `config.yaml` 放到 `/opt/nmonitor`，创建数据目录和 `nmonitor.env`，写入 systemd 单元，设置开机启动并立即启动。已经存在的 `config.yaml` 和 `nmonitor.env` 不会被覆盖。再次执行会更新二进制和单元；服务若已在运行，会重启以加载新程序。

在终端里直接运行、并且还没有 `nmonitor.env` 时，会提示设置管理员密码。非交互环境下请通过 `NMONITOR_PASSWORD` 传入，否则会先写入示例值 `change-me`。

也可以手工安装：

```bash
sudo mkdir -p /opt/nmonitor/data
sudo cp nmonitor config.yaml /opt/nmonitor/
sudo cp deploy/nmonitor.service /etc/systemd/system/
sudo cp deploy/nmonitor.env.example /opt/nmonitor/nmonitor.env
sudo systemctl enable --now nmonitor
```

## Docker

```bash
docker build -t nmonitor .
docker run --rm -p 8080:8080 \
  -e NMONITOR_PASSWORD=your-password \
  -v nmonitor-data:/opt/nmonitor/data \
  nmonitor
```

数据库路径在容器内是 `/opt/nmonitor/data/nmonitor.db`。

## 接口

除 `POST /api/auth/login`、`GET /health`、`GET /version` 外，接口需要 `Authorization: Bearer <token>`。

主要资源：

```text
GET/POST        /api/monitors
GET/PUT/DELETE  /api/monitors/:id
POST            /api/monitors/:id/check
POST            /api/monitors/:id/enable
POST            /api/monitors/:id/disable
GET             /api/dashboard
GET/POST        /api/notifiers
POST            /api/notifiers/:id/test
GET             /api/events
GET             /api/certificates
GET             /api/host
GET             /api/host/processes
POST            /api/scripts/run
GET             /api/scripts/runs
GET/POST        /api/cron-jobs
GET/PUT/DELETE  /api/cron-jobs/:id
POST            /api/cron-jobs/:id/run
POST            /api/cron-jobs/:id/enable
POST            /api/cron-jobs/:id/disable
GET             /api/cron-jobs/:id/runs
GET             /api/cron-runs
GET             /api/cron/preview
GET             /api/logs
GET             /api/services
POST            /api/services/:name/start
POST            /api/services/:name/stop
POST            /api/services/:name/restart
GET             /api/services/:name/logs
GET             /api/shell
POST            /api/shell/ticket
GET             /api/shell/ws
GET             /api/access/permissions
GET/POST        /api/access/roles
GET/PUT/DELETE  /api/access/roles/:id
GET/POST        /api/access/users
GET/PUT/DELETE  /api/access/users/:id
```

## 系统服务

管理台的「系统服务」调用本机 `systemctl` 列出 service 单元，并执行启动、停止和重启。日志通过 `journalctl -u` 读取，默认 200 行，最多 500 行。执行身份与 NMonitor 进程相同。

服务名只接受 systemd 的 service 单元，不能包含空格、路径或 shell 元字符。`systemd.service` 和 `init.service` 不能在这里操作。命令不经过 shell。启动、停止、重启和失败原因会写进运行日志。

没有 `systemctl` 的环境仍可打开页面，接口会返回不可用原因。可以在 `config.yaml` 的 `service` 段设置 `enabled: false`，或用 `NMONITOR_SERVICE_ENABLED=false` 关闭。用 root 安装的 systemd 服务可以直接操作；普通用户可能因权限不足只能查看，或在操作时看到 systemctl 返回的拒绝信息。

## 进程

管理台的「主机资源」列出本机进程，可以按 CPU 或内存从高到低排序。CPU 按单个核心计算，多线程进程可以超过 100%。内存按常驻内存计算，并显示占物理内存的比例。列表约 15 秒刷新一次，默认显示占用最高的 80 个，最多 300 个。

`GET /api/host/processes` 接受 `sort=cpu` 或 `sort=memory`，以及 `limit`。

## 在线终端

管理台的「在线终端」通过 WebSocket 连接本机 Shell，执行身份与 NMonitor 进程相同。Linux 上启动 bash 或 sh；Windows 10 1809 及以上通过 ConPTY 启动 PowerShell，找不到时使用 cmd。页面先申请 30 秒内有效、只能使用一次的连接凭证，再升级连接，长期登录令牌不会出现在地址栏里。

默认空闲 15 分钟断开，单次最长 4 小时，同时最多 8 个会话。可以在 `config.yaml` 的 `shell` 段调整，或用 `NMONITOR_SHELL_ENABLED=false` 关闭。跨站页面不能连接。会话的建立和断开会写进运行日志，按键内容不会记录。

## 定时任务

管理台的「定时任务」按 cron 表达式执行，例如 `0 2 * * *` 表示每天 02:00。新建和编辑时可以用 cron 组件按分、时、日、月、周选择任意、间隔、范围或指定值，也可以直接填写表达式。也支持 `@hourly`、`@daily`、`@weekly`、`@monthly`、`@yearly`。月份可以用 `JAN` 到 `DEC`，星期可以用 `SUN` 到 `SAT`，`0` 和 `7` 都是周日。

任务可以是脚本，也可以是网络请求。网络请求支持 GET、POST 表单和 POST JSON。表单按 `key=value` 发送，JSON 会带上 `Content-Type: application/json`。状态码在 200 到 299 之间算成功。

执行时间使用系统配置中的时区。超时最长 1 小时，执行记录保留 30 天。任务还在跑时，下一次到点会被跳过，同一个任务不会并行执行。配置导入导出会带上这些任务。

执行记录在页面右侧抽屉里查询，可按任务、结果、触发方式、时间和关键字筛选。`GET /api/cron-runs` 支持 `jobId`、`success`（`true` 或 `false`）、`trigger`（`schedule` 或 `manual`）、`keyword`、`from`、`to`。日期写成 `YYYY-MM-DD`，按系统时区的自然日，结束日期当天包含在内。单个任务的记录接口接受除 `jobId` 以外的同样条件。

## 用户与角色

管理台的「用户与角色」用来分配谁能看、谁能改。首次启动会创建三个内置角色，并把原来的管理员账号放进「管理员」：

- 管理员：全部权限，包含系统配置、在线终端和账号管理
- 运维：管理监控、分组、通知、脚本、定时任务和系统服务，可以查看系统配置。不能修改配置、打开在线终端或管理账号
- 只读：查看监控、主机、告警、通知和运行日志

内置角色不能删除，管理员角色始终拥有全部权限。运维和只读的权限可以调整。也可以新建角色，勾选需要的权限。一个用户可以同时拥有多个角色，能做的事是这些角色的并集。

至少保留一个启用的、可以管理用户与角色的账号。不能删除或停用当前登录账号。用户名以字母开头，2 到 32 位。修改密码后，已经发出的登录令牌在过期前仍然有效。

原来的 `NMONITOR_USERNAME` 和 `NMONITOR_PASSWORD` 只用于创建第一个管理员。如果系统配置里已经保存过密码，会继续使用那份密码。之后在页面上修改这个账号的密码，启动提示也会跟着更新。

没有对应权限时，菜单里不显示入口，直接调用接口会返回 403。

## 测试

```bash
go test ./...
```
