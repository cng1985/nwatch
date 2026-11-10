# jobs 分组

任务业务分组。

| 子模块 | 职责 | 数据所有权 |
| --- | --- | --- |
| cronjob | 定时任务 CRUD、cron 表达式、脚本/HTTP 执行、执行记录查询、调度 | cron_jobs、cron_runs |

## 允许依赖

- platform/*（settings、httpreq、config）。
- monitoring/checker（脚本执行引擎）、monitoring/shared（无：仅 checker）。
- 被依赖方：admin/settings 与 admin/backup 调用 Service.RescheduleAll；monitoring/maintenance 清理执行记录。

## 跨模块原子操作归属

任务删除级联（主表+执行记录）归属 cronjob.Repository 单事务。
