# 数据库说明

## 驱动与连接

- 默认 SQLite（纯 Go 驱动，免 CGO），相对路径相对 data-dir 解析。
- 同时支持 mysql、postgres、sqlserver：通过 `database.dsn`/APP_DATABASE_DSN 提供原生连接串。声明矩阵：sqlite 已验证，其余三库连接能力已实现、兼容性待按规范 22 节验证（见 docs/decisions.md D9）。
- `database.enabled=false` 时不初始化数据库；未知驱动或参数缺失启动失败，不静默回退。

## 迁移

- 受控 AutoMigrate：模型清单集中在 `internal/modules/migrations/migrations.go`，按数据归属登记。
- `app/bootstrap.Migrate` 是全应用最先执行的 Invoke；失败即启动失败。
- 初始化幂等：内置角色（admin/operator/viewer）与管理员账号只在缺失时创建，更新不重置已有数据。

## 数据约定

- 表列 snake_case，固定表名（实体 TableName 显式声明）。
- ID 为正 int64（uint），JSON 数值；创建不接受客户端主键。
- 时间点内部 time.Time，API 带时区 RFC 3339；业务日期按系统时区的自然日。
- 审计列（created_at/updated_at）由服务端维护。
- 删除策略：监控删除级联清理检测记录与指标（同事务）；分组删除解除引用；单据类数据（事件、检测记录、通知日志）保留并按保留天数清理。

## 保留与清理

maintenance 按设置中的保留天数清理：检测记录、事件、指标、小时指标、终端命令审计、运行日志、定时任务执行记录。
