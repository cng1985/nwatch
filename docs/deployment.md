# 部署说明

## 源码运行

```bash
make build        # 前端 + Go 构建
make run          # APP_PROFILE=dev 运行
go test ./...     # 全量测试
```

未指定 profile 时只加载 configs/application.yaml；`--profile dev` 或 APP_PROFILE=dev 叠加 application-dev.yaml。

## 配置

- configs/ 三文件：application.yaml（基础）、application-dev.yaml、application-pro.yaml。
- 覆盖顺序：基础 < profile < 环境变量（APP_*，NMONITOR_* 为已登记别名）< 命令行。
- 启动定位参数：--base-dir/APP_BASE_DIR、--config-dir/APP_CONFIG_DIR、--data-dir/APP_DATA_DIR、--profile/APP_PROFILE。
- 生产凭据不写入配置文件；用 deploy/nwatch.env.example 建立环境文件。

## 发布打包（Windows 入口 / 跨平台目标）

```powershell
scripts\build-platforms.cmd                        # 默认 windows/amd64 linux/amd64 linux/arm64
scripts\build-platforms.cmd linux/amd64            # 指定目标
pwsh scripts/build-platforms.ps1 -SkipWeb          # 跳过前端（需已有 web/dist）
```

产物进入 release/&lt;os&gt;-&lt;arch&gt;/：二进制（含 --version）、configs/ 模板、Linux 附 install-linux.sh、RELEASE.txt（版本/提交/时间）。任何阶段失败返回非零。

## Linux 安装与更新

```bash
sudo env APP_ADMIN_PASSWORD='your-password' ./scripts/install-linux.sh
sudo ./scripts/install-linux.sh --binary release/linux-amd64/nwatch --no-start
```

- 布局：程序 /opt/nwatch，配置 /opt/nwatch/configs，数据 /var/lib/nwatch，专用用户 nwatch，生产 profile=pro。
- 更新先停服务再替换二进制，保留配置与数据；daemon-reload 后 restart。
- 查看服务：`systemctl status nwatch`、`journalctl -u nwatch`。
- 数据库迁移后能否回切旧二进制未验证，更新前先备份。

## Docker

```bash
docker build -t nwatch .
docker run -e APP_PROFILE=pro -e APP_ADMIN_PASSWORD=... -p 8080:8080 -v nwatch-data:/var/lib/nwatch nwatch
```

## 健康与备份

- GET /health/live、/health/ready（HTTP 200/503）。
- 管理台"系统设置"页提供手动备份与下载（SQLite 在线一致备份），备份目录默认 data/backups，保留 7 天。
