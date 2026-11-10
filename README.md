# NWatch

面向 Linux 环境的轻量级服务可用性监控。回答的问题很具体：服务现在能不能访问，证书还有多久过期，出了问题有没有立刻通知到人。

默认只需要一个二进制、一个 SQLite 文件和一份配置：

```text
nwatch
configs/
data/app.db
```

不依赖 MySQL、Redis 或消息队列。

## 能力

- HTTP/HTTPS 检查：方法、请求头、状态码、响应内容、超时和响应时间
- HTTPS 证书：到期时间、剩余天数、颁发机构、主机名和证书链
- TCP 端口连通性
- 本机 CPU、内存、磁盘：15 秒采样，1 小时到 7 天趋势；可按使用率阈值告警。进程可按 CPU 或内存排序查看
- 进程监控：从本机进程中选择程序。进程停止后按现有通知渠道提醒；也可设置 CPU、内存阈值，超过后同样告警。进程重启后按进程名或命令行继续识别，不依赖 PID
- 脚本执行：定时检查或在页面上手动执行，记录退出码和输出
- 定时任务：用 5 段 cron 表达式执行脚本，或发起 HTTP/HTTPS 请求。POST 支持 JSON、Form、FormData、XML 和 Text。时间按系统配置里的时区计算，同一个任务不会重叠执行
- 在线终端：登录后在管理台打开本机交互式 Shell。Linux 使用 bash 或 sh，Windows 10 1809 及以上通过 ConPTY 使用 PowerShell 或 cmd
- 系统服务：在管理台列出本机 systemd 服务，启动、停止、重启，并用 journalctl 查看日志
- Docker：列出本机容器和镜像，启动、停止、重启容器，并查看日志
- Kubernetes：列出 Pod、Deployment、StatefulSet、DaemonSet 和 Service，滚动重启、调整副本，删除 Pod，并查看 Pod 日志
- 运行日志：在管理台查看进程日志，保留 7 天
- 调度器每秒领取到期任务，固定大小的工作池执行，同一个监控不会重叠检查
- 连续失败后才标记为异常，恢复也可要求连续成功
- 钉钉（支持加签）、企业微信、Webhook，通知异步发送并记录结果
- 邮件和所有已启用的通知渠道：连续失败达到阈值后，15 秒通知一次，45 秒再通知一次，之后每 1 分钟通知一次，直到服务恢复。关闭的渠道不发送。某一封邮件发送失败会持续重试，直到成功
- 证书按 30/14/7/3/1/0 天阈值各提醒一次
- 检测历史、分钟/小时/天统计、1 小时到 30 天可用率
- Web 管理台：总览、主机资源、系统服务、Docker、Kubernetes、监控、脚本执行、定时任务、在线终端、证书、告警、通知、运行日志、分组、用户与角色、系统配置
- 基于角色的访问控制：内置管理员、运维、只读，也可以自定义角色。一个用户可以有多个角色，菜单和接口都按权限控制
- 配置导入导出，SQLite 备份
- 监控列表导出、导入 Excel，便于批量修改和迁移

## 代码结构

按《Go 项目结构与开发标准规范》（v3.4）组织：platform 承载横切能力，modules 按业务分组，app 是组合根。业务调用链为 Controller → Resource → Repository/适配器，公开方法统一 `y=f(x)` 外壳（Response/PageResponse）。

```text
cmd/server/main.go        进程入口，仅调用 app.Run()
internal/app/             组合根：启动、模块组合、受控迁移与幂等初始化
internal/platform/
  config/                 configs/ 三文件加载、profile 覆盖、环境变量与校验
  database/               SQLite（默认）与 mysql/postgres/sqlserver 连接
  response/ errors/       统一响应外壳与错误分类、应用码
  middleware/ routing/    认证、授权、恢复、请求标识；路由注册契约
  server/                 Gin 引擎、健康端点与 HTTP 生命周期
  logging/ auth/ settings/ domain/ httpreq/ version/
internal/modules/
  migrations/             受控迁移模型清单
  monitoring/             监控分组：monitor、group、checker、scheduler、state、
                          metric、alert、dashboard、host、maintenance、script、shared
  notify/                 通知分组：notifier（渠道）、mailer（邮件）
  jobs/                   任务分组：cronjob
  ops/                    运维分组：shell、service、docker、kube
  admin/                  系统管理分组：access、settings、backup、logview
configs/                  application.yaml、application-dev.yaml、application-pro.yaml
web/                      Vue 3 管理台（src/）+ 构建产物（dist/）与 Go 嵌入
scripts/                  build-platforms.cmd、build-platforms.ps1、install-linux.sh
deploy/                   环境变量示例
tests/                    跨模块集成测试
docs/                     架构、接口、数据库、部署、验收与决策记录
```

## 构建

需要本机已安装 Go 和 Node.js。脚本先构建前端（写入 `web/dist` 并嵌入二进制），再分平台编译并打包到 `release/<系统>-<架构>/`：

```powershell
scripts\build-platforms.cmd                        # Windows：默认三个目标
scripts\build-platforms.cmd linux/amd64            # 指定目标
pwsh scripts/build-platforms.ps1 -SkipWeb          # 已有 web/dist 时跳过前端
```

也可以用 Makefile，产物是当前目录下的 `nwatch`：

```bash
make build
```

## 运行

```bash
make build
export APP_ADMIN_PASSWORD='your-password'
APP_PROFILE=dev ./nwatch
```

浏览器打开 `http://127.0.0.1:8080`。未设置密码时，初始账号是 `admin` / `admin`，启动日志会提示尽快修改。

只编译后端时，需要先构建前端：

```bash
cd web && npm install && npm run build
go build -o nwatch ./cmd/server
```

## 配置

统一放在 `configs/`：`application.yaml` 是基础配置，`application-dev.yaml` / `application-pro.yaml` 是 profile 差异配置，同一时间只加载基础 + 一个 profile。覆盖顺序：基础配置 < profile 配置 < 环境变量 < 命令行。

启动定位参数：`--base-dir`、`--config-dir`、`--data-dir`、`--profile`（或 `APP_BASE_DIR`、`APP_CONFIG_DIR`、`APP_DATA_DIR`、`APP_PROFILE`）。SQLite 相对路径相对 data-dir 解析。`--version` 输出版本后退出。

密码不要写进文件，使用环境变量（旧的 `NMONITOR_*` 名称作为已登记别名继续可用）：

```bash
APP_ADMIN_PASSWORD
APP_JWT_SECRET
APP_DATABASE_DSN
APP_SMTP_HOST / APP_SMTP_PORT / APP_SMTP_USERNAME / APP_SMTP_PASSWORD
APP_SMTP_FROM / APP_SMTP_TO / APP_SMTP_ENCRYPTION / APP_SMTP_ENABLED
APP_SHELL_ENABLED / APP_SERVICE_ENABLED / APP_DOCKER_ENABLED
APP_KUBERNETES_ENABLED / APP_KUBE_CONTEXT / APP_KUBECONFIG
```

`APP_SMTP_ENCRYPTION` 可以是 `starttls`、`ssl` 或 `none`。收件人有多个时用逗号分隔。界面里保存的邮件配置会覆盖这里的环境变量。完整字段字典见 `docs/deployment.md`。

## systemd

在已经编译好的机器上，用脚本安装并启动服务（需要 root，且系统使用 systemd）：

```bash
sudo env APP_ADMIN_PASSWORD='your-password' ./scripts/install-linux.sh
```

脚本把二进制放到 `/opt/nwatch`、配置放到 `/opt/nwatch/configs`、数据放到 `/var/lib/nwatch`，创建专用用户 `nwatch` 与 systemd 单元（生产 profile=pro），设置开机启动并立即启动。已经存在的配置与环境文件不会被覆盖。再次执行会先停服务、更新二进制后重启；查看状态用 `systemctl status nwatch`，日志用 `journalctl -u nwatch`。

## Docker

```bash
docker build -t nwatch .
docker run --rm -p 8080:8080 \
  -e APP_PROFILE=pro -e APP_ADMIN_PASSWORD=your-password \
  -v nwatch-data:/var/lib/nwatch \
  nwatch
```

数据库路径在容器内是 `/var/lib/nwatch/app.db`。

## 接口

除 `POST /api/auth/login`、`GET /health/live`、`GET /health/ready`、`GET /health`、`GET /version` 外，接口需要 `Authorization: Bearer <token>`。

普通业务 JSON API 成功与应用错误均返回 HTTP 200：单条成功是 `{"code":200,"msg":"success","data":...}`，分页原样输出 `{"data":[...],"total","page","page_size"}`，错误是 `{"code":400,"msg":"..."}`。网络层错误仍可能是真实非 200。完整端点清单见 `docs/api.md`。

## 系统服务

管理台的「系统服务」调用本机 `systemctl` 列出 service 单元，并执行启动、停止和重启。日志通过 `journalctl -u` 读取，默认 200 行，最多 500 行。执行身份与进程相同。

服务名只接受 systemd 的 service 单元，不能包含空格、路径或 shell 元字符。`systemd.service` 和 `init.service` 不能在这里操作。命令不经过 shell。启动、停止、重启和失败原因会写进运行日志。

没有 `systemctl` 的环境仍可打开页面，接口会返回不可用原因。可以在配置的 `service` 段设置 `enabled: false`，或用 `APP_SERVICE_ENABLED=false` 关闭。用 root 安装的 systemd 服务可以直接操作；普通用户可能因权限不足只能查看，或在操作时看到 systemctl 返回的拒绝信息。

## Docker 管理

管理台的「Docker」调用本机 `docker` 客户端列出容器和镜像。容器可以启动、停止、重启，日志通过 `docker logs` 读取，默认 200 行，最多 500 行。执行身份与进程相同，命令不经过 shell。

容器名只接受字母、数字、下划线、点和横线，不能以横线开头，也不能包含空格或路径。这里不进入容器，也不删除容器或镜像。

没有 `docker` 或连不上守护进程时，页面会提示不可用。可以在配置的 `docker` 段设置 `enabled: false`，或用 `APP_DOCKER_ENABLED=false` 关闭。程序跑在容器里时，需要镜像内有 docker 客户端，并且能访问宿主机的 Docker 套接字，才会看到宿主机上的容器。

## Kubernetes

管理台的「Kubernetes」调用本机 `kubectl`。可以按命名空间查看 Pod、Deployment、StatefulSet、DaemonSet 和 Service。Deployment、StatefulSet 和 DaemonSet 可以滚动重启，前两类还可以把副本数改成 0 到 64。删除 Pod 后由控制器重建。Pod 日志通过 `kubectl logs` 读取，默认 200 行，最多 500 行。执行身份与进程相同，命令不经过 shell。

名称只接受 Kubernetes 的 DNS 名称。这里不提供进入容器的终端。

`kubernetes.context` 和 `kubernetes.kubeconfig` 留空时使用当前环境的默认 kubeconfig。也可以用 `APP_KUBE_CONTEXT` 和 `APP_KUBECONFIG` 覆盖。没有 `kubectl`，或 kubeconfig 不能访问集群时，页面会提示原因。可以设置 `kubernetes.enabled: false`，或用 `APP_KUBERNETES_ENABLED=false` 关闭。

## 进程

管理台的「主机资源」列出本机进程，可以按 CPU 或内存从高到低排序。CPU 按单个核心计算，多线程进程可以超过 100%。内存按常驻内存计算，并显示占物理内存的比例。列表约 15 秒刷新一次，默认显示占用最高的 80 个，最多 300 个。

`GET /api/host/processes` 接受 `sort=cpu` 或 `sort=memory`，以及 `limit`。

## 在线终端

管理台的「在线终端」通过 WebSocket 连接本机 Shell，执行身份与进程相同。Linux 上启动 bash 或 sh；Windows 10 1809 及以上通过 ConPTY 启动 PowerShell，找不到时使用 cmd。页面先申请 30 秒内有效、只能使用一次的连接凭证，再升级连接，长期登录令牌不会出现在地址栏里。

默认空闲 15 分钟断开，单次最长 4 小时，同时最多 8 个会话。页面可以同时打开多个终端，每个终端是一次独立会话。可以在配置的 `shell` 段调整，或用 `APP_SHELL_ENABLED=false` 关闭。跨站页面不能连接。

会话的建立和断开会写进运行日志。用户在终端里提交的命令会同时写入运行日志和命令记录，内容包括登录用户、会话编号、来源地址和命令本身。命令记录保留 30 天，可在终端页面按用户和命令查询，有运行日志权限的账号也可以查询。方向键翻出的历史命令和 Tab 补全，按屏幕上实际提交的那一行记录。屏幕上是密码或口令提示时，随后输入的内容不会记录。直接敲下的按键、退格和粘贴会按最终提交的命令记录，而不是每个按键。

## 定时任务

管理台的「定时任务」按 cron 表达式执行，例如 `0 2 * * *` 表示每天 02:00。新建和编辑时可以用 cron 组件按分、时、日、月、周选择任意、间隔、范围或指定值，也可以直接填写表达式。也支持 `@hourly`、`@daily`、`@weekly`、`@monthly`、`@yearly`。月份可以用 `JAN` 到 `DEC`，星期可以用 `SUN` 到 `SAT`，`0` 和 `7` 都是周日。

任务可以是脚本，也可以是网络请求。网络请求支持 GET，以及 POST 的 JSON、Form、FormData、XML 和 Text。表单和 FormData 按 `key=value` 填写，发送时分别使用 `application/x-www-form-urlencoded` 和 `multipart/form-data`。JSON、XML、Text 会带上对应的 Content-Type。状态码在 200 到 299 之间算成功。

执行时间使用系统配置中的时区。超时最长 1 小时，执行记录保留 30 天。任务还在跑时，下一次到点会被跳过，同一个任务不会并行执行。配置导入导出会带上这些任务。

执行记录在页面右侧抽屉里查询，可按任务、结果、触发方式、时间和关键字筛选。`GET /api/cron-runs` 支持 `jobId`、`success`（`true` 或 `false`）、`trigger`（`schedule` 或 `manual`）、`keyword`、`from`、`to`。日期写成 `YYYY-MM-DD`，按系统时区的自然日，结束日期当天包含在内。单个任务的记录接口接受除 `jobId` 以外的同样条件。

## 监控导入导出

监控列表可以导出为 Excel，改完后再导入。

- 导出的是当前筛选结果，包含检测配置，也带上状态、最近结果和可用率
- 导入只新建和更新，不会删除表格里没有的监控
- 填了编号且本机存在这条监控时，按编号更新，名称也可以改
- 编号留空时按名称查找：只有一条同名监控就更新，没有就新建，有多条则必须补编号
- 从别的环境导入时先清空编号列，避免改到本机同编号的监控
- 分组不存在时会自动创建。通知渠道必须已经存在，多个名称用逗号或顿号分隔
- 导出的表格里有「说明」页，列出每一列怎么填

`GET /api/monitors/export` 下载 xlsx，加上 `template=1` 只下载表头。`POST /api/monitors/import` 使用 multipart 字段 `file`。

## 网络请求导入导出

监控列表可以单独导出、导入网络请求。表格只有四列：名称、分组、类型、地址。

- 只包含 HTTP/HTTPS 监控。导出时带上当前搜索词和状态
- 类型填 `http` 或 `https`。地址没写协议时按类型补上；两边不一致时以类型为准
- 按名称更新。没有同名监控就新建，使用默认检测间隔，请求方法为 GET
- 更新时不改检测间隔、超时、请求方法、请求头、请求体和通知渠道
- 分组不存在时自动创建，留空表示不分组。不会删除表格里没有的监控
- 同名监控有多条，或同名的不是网络请求时，整次导入不会写入
- 带编号、请求方法等列的完整监控表请走上面的监控导入

`GET /api/monitors/requests/export` 下载 xlsx，加上 `template=1` 只下载表头。`POST /api/monitors/requests/import` 使用 multipart 字段 `file`。

## 用户与角色

管理台的「用户与角色」用来分配谁能看、谁能改。首次启动会创建三个内置角色，并把原来的管理员账号放进「管理员」：

- 管理员：全部权限，包含系统配置、在线终端和账号管理
- 运维：管理监控、分组、通知、脚本、定时任务、系统服务、Docker 和 Kubernetes，可以查看系统配置。不能修改配置、打开在线终端或管理账号
- 只读：查看监控、主机、告警、通知、运行日志、Docker 和 Kubernetes

内置角色不能删除，管理员角色始终拥有全部权限。运维和只读的权限可以调整。也可以新建角色，勾选需要的权限。一个用户可以同时拥有多个角色，能做的事是这些角色的并集。

至少保留一个启用的、可以管理用户与角色的账号。不能删除或停用当前登录账号。用户名以字母开头，2 到 32 位。修改密码后，已经发出的登录令牌在过期前仍然有效。

`security.username` 和 `security.password`（或环境变量 `APP_ADMIN_USERNAME` / `APP_ADMIN_PASSWORD`）只用于创建第一个管理员。如果系统配置里已经保存过密码，会继续使用那份密码。之后在页面上修改这个账号的密码，启动提示也会跟着更新。

没有对应权限时，菜单里不显示入口，直接调用接口返回 `{"code":403,...}`。

管理员角色会在启动时同步全部权限。已经存在的运维、只读角色如果还是原来的默认权限，启动时会补上 Docker 和 Kubernetes；权限已经被改过的角色保持原样，需要在「用户与角色」里另行勾选。

## 数据库支持

默认 SQLite（`data/app.db`，WAL）。`database.driver` 支持 `mysql`、`postgres`、`sqlserver`，连接串用 `database.dsn` / `APP_DATABASE_DSN` 注入。sqlite 已在本项目验证；其余三库的迁移、约束与并发用例尚未验证（见 `docs/decisions.md` D9）。

## 测试

```bash
gofmt -l internal tests cmd web | grep . && exit 1 || true
go vet ./...
go test ./...
cd web && npm run build
```

集成测试位于 `tests/`，通过完整装配的业务模块走 HTTP 验证行为。
