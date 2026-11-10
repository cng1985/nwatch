# 架构说明

nwatch 是一个模块化单体：Go + Gin + GORM + Fx 后端与 Vue 3 前端（Go 嵌入），提供监控、告警、定时任务与运维管理台。

## 目录结构

```text
cmd/server/            入口，仅调用 app.Run()
internal/
├── app/               组合根：app.go（启动）、module.go（组合）、bootstrap.go（迁移与初始化）
├── modules/
│   ├── migrations/    受控迁移模型清单
│   ├── monitoring/    监控分组：shared、monitor、group、checker、scheduler、state、
│   │                  metric、alert、dashboard、host、maintenance、script
│   ├── notify/        通知分组：notifier（渠道）、mailer（邮件）
│   ├── jobs/          任务分组：cronjob
│   ├── ops/           运维分组：shell、service、docker、kube
│   └── admin/         系统管理分组：access、settings、backup、logview
└── platform/
    ├── config/        配置加载、覆盖与验证（configs/ 三文件 + profile + 环境变量）
    ├── database/      四库连接、连接池与生命周期
    ├── domain/        可信 Scope 与分页边界
    ├── response/      泛型结果与 Gin 直接输出函数
    ├── errors/        错误分类、应用码与公开消息
    ├── middleware/    认证、授权、恢复、请求标识与请求体限制
    ├── routing/       公共路由注册契约
    ├── server/        Gin 引擎、健康端点与 HTTP 生命周期
    ├── logging/       结构化日志与可查询日志存储
    ├── auth/          身份、会话、角色和权限（含用户/角色实体）
    ├── settings/      运行时可变系统设置
    ├── httpreq/       受控 HTTP 请求体构造
    └── version/       版本信息
configs/                application.yaml、application-dev.yaml、application-pro.yaml
web/                    Vue 前端源码 + dist 构建产物 + embed.go/static.go/module.go
scripts/                build-platforms.cmd、build-platforms.ps1、install-linux.sh
tests/                  跨模块集成测试与测试装配
docs/                   本文档集
```

## 依赖规则

- 调用链：Controller → Resource → Repository（有数据库时）→ 数据库；无数据库时 Resource 调用适配器（Manager/Client/Processor）。
- app 是组合根；platform 不导入业务模块；分组根只组合子模块；子模块不反向导入父分组包。
- monitoring/shared 只在 monitoring 分组内使用；跨分组共享走 platform。
- 通知域（notifier）不依赖监控域实体：告警事件在 monitoring/alert/convert.go 转换为通知域 Event 视图。
- 所有公开业务方法遵循 y=f(x)：Request/Search 输入，Response/PageResponse 输出。
- 事务由 Repository 拥有：监控删除级联、通知渠道解绑、定时任务删除、角色/用户更新均为单事务方法。
- Fx Invoke 按注册顺序执行：bootstrap 的迁移是全应用最先执行的初始化。

## 启动顺序

配置校验 → 日志 → 迁移（bootstrap Migrate）→ 幂等初始化（内置角色与管理员）→ 后台组件（调度器、工作池、采样、告警队列、定时任务、清理）→ HTTP 可服务 → 就绪。

健康端点：GET /health/live（存活）、GET /health/ready（数据库与组件就绪）、GET /health（兼容详细视图）、GET /version。
