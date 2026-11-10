# monitoring 分组

监控业务分组。父目录只组合子模块，不承载业务代码。

## 子模块职责与数据所有权

| 子模块 | 职责 | 数据所有权 |
| --- | --- | --- |
| shared | 分组内共享实体与常量（Monitor、Group、MonitorCheck、MonitorMetric、AlertEvent、ScriptRun） | 分组内共享表结构的唯一定义 |
| monitor | 监控项 CRUD、启停、手动检测、检测记录/事件/指标查询、证书视图、Excel 导入导出 | monitors 及关联（经 shared） |
| group | 监控分组 CRUD | groups |
| checker | 检测执行引擎（http/tls/tcp/cpu/mem/disk/script/process） | 无持久化；按需写入 ScriptRun |
| scheduler | 到期检测调度与工作池 | 无持久化（经 shared 更新检测状态） |
| state | 连续失败/成功的状态机 | 无持久化 |
| metric | 分钟/小时/天聚合与查询 | monitor_metrics |
| alert | 告警决策、异步通知队列、故障期间定时提醒 | 写 notification_logs（实体在 notify 分组） |
| dashboard | 总览与趋势查询 | 只读 |
| host | 主机资源采样、历史与进程列表 | host_samples |
| maintenance | 数据保留清理与备份 | 跨表清理（只删不改） |
| script | 手动脚本执行与记录查询 | script_runs |

## 允许依赖

- 分组内：shared、兄弟模块（无环：monitor→scheduler→checker→shared 等）。
- 分组外：platform/*、notify/notifier（发送适配与实体）、notify/mailer（告警邮件）、jobs/cronjob（仅 maintenance 清理 cron 数据）。
- 禁止：反向导入本组合包；notify 分组不得反向依赖本分组实体（通知走 notifier.Event 视图）。

## 跨模块原子操作归属

当前无跨分组强原子操作。监控删除级联（主表+明细+绑定）归属 monitor.Repository 单事务；出库审核式的跨模块事务协作（规范 21 节）出现真实需求时，按该节显式设计并在本 README 登记。
