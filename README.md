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
- 调度器每秒领取到期任务，固定大小的工作池执行，同一个监控不会重叠检查
- 连续失败后才标记为异常，恢复也可要求连续成功
- 钉钉（支持加签）、企业微信、Webhook，通知异步发送并记录结果
- 证书按 30/14/7/3/1/0 天阈值各提醒一次
- 检测历史、分钟/小时/天统计、1 小时到 30 天可用率
- Web 管理台：总览、监控、证书、告警、通知、分组、系统配置
- 管理员登录，默认密码可通过环境变量覆盖
- 配置导入导出，SQLite 备份

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
```

## systemd

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
```

## 测试

```bash
go test ./...
```
