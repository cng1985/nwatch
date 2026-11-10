# notify 分组

通知业务分组。

| 子模块 | 职责 | 数据所有权 |
| --- | --- | --- |
| notifier | 通知渠道 CRUD、测试发送、发送适配（钉钉/企业微信/Webhook）、通知日志查询 | notifiers、notification_logs |
| mailer | 邮件通道（SMTP 发送与失败重试、测试邮件） | 经 notifier 写 notification_logs，无独立表 |

## 允许依赖

- platform/*（settings、config 等）。
- 依赖监控分组仅限：mailer 引用 monitoring/shared 的 AlertEvent 构造邮件内容（单向，无环）；notifier 不依赖监控域（接收 notifier.Event 视图）。
- 被依赖方：monitoring/alert 调用本分组发送能力。

## 跨模块原子操作归属

通知发送为异步尽力而为（队列重试），不参与业务事务；可靠通知需要 Outbox 时按规范 6.4/19 节显式设计。
