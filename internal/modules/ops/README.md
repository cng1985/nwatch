# ops 分组

运维操作分组。均为非数据库适配模块：Controller → Resource → Manager（外部命令适配）。

| 子模块 | 职责 | 数据所有权 |
| --- | --- | --- |
| shell | 在线终端会话、一次性连接票据、命令审计查询 | shell_commands（审计） |
| service | systemd 服务列表/启停/日志（systemctl、journalctl） | 无持久化 |
| docker | 容器/镜像列表、启停、日志（docker 客户端） | 无持久化 |
| kube | Kubernetes 资源查看、重启、伸缩、删 Pod、日志（kubectl） | 无持久化 |

## 允许依赖

- platform/*。无跨分组依赖；被 monitoring/maintenance 引用清理命令审计。

## 安全边界

- 外部命令不经过 shell，名称白名单校验；能力可通过配置关闭。
- 终端票据一次性且 30 秒有效，跨站来源拒绝；命令审计记录登录用户、会话与来源。
