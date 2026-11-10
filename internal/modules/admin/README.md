# admin 分组

系统管理分组。

| 子模块 | 职责 | 数据所有权 |
| --- | --- | --- |
| access | 登录会话、修改密码、用户/角色 CRUD、权限目录 | users、user_roles、roles（实体在 platform/auth） |
| settings | 系统设置查询与修改、邮件通道设置与测试 | settings（KV，实体在 platform/settings） |
| backup | SQLite 备份、配置导入导出 | 只读全库导出；导入写各业务表 |
| logview | 运行日志查询 | app_logs（实体在 platform/logging） |

## 允许依赖

- platform/*（auth、settings、logging、config）。
- jobs/cronjob（时区变化与导入后重算调度）、notify/mailer（测试邮件）。
- 禁止：导入导出绕过业务校验直写其他分组表的批量入口（transfer.Import 按各域 Request 校验后落库）。

## 跨模块原子操作归属

配置导入的写入在 backup 单事务内完成；导入后调度重算在事务提交后执行。
