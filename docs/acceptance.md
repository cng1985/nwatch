# 验收检查表

对照规范第 24 节（A01—A15）的落地情况与证据：

| 条目 | 状态 | 证据 |
| --- | --- | --- |
| A01 分模块结构、按需文件、y=f(x) | 达成 | internal/modules/{monitoring,notify,jobs,ops,admin}；各叶子模块 domain/entity/convert/repository/resource/controller/module |
| A02 Resource 编排、Repository 管事务 | 达成 | monitor/group/notifier/cronjob 的 Repository 事务方法；无数据库模块走 Manager 适配 |
| A03 公开方法统一外壳 | 达成 | response.Response/PageResponse；go vet + 集成测试断言外壳 |
| A04 事务失败回滚 | 达成 | 监控删除级联、通知解绑、任务删除、角色/用户更新均单事务 |
| A05 跨模块原子操作 | 不适用/按需 | 当前无跨模块强原子需求；规范 21 节为按需能力 |
| A06 Scope 可信传递 | 达成 | platform/domain Scope 由认证中间件写入标准 context；改密/用户管理读取 ActorID |
| A07 受保护字段、零值、批量、ID、时间 | 达成 | 修改先读后应用字段；创建拒绝客户端主键；表格导入限 8MB；批量大小可配置 |
| A08 查询白名单与上限 | 达成 | 分页上限 200、偏移保护；状态筛选白名单（service/docker）；导出上限 10000 |
| A09 三份配置与字段字典 | 达成 | configs/ 三文件；load.go 显式登记 APP_* 与别名；未知键校验 |
| A10 默认 SQLite、四库声明 | 达成（部分） | sqlite 已验证；其余三库连接实现、兼容性未验证（decisions D9） |
| A11 Gin 安全、错误输出、健康、启停 | 达成 | Recovery/RequestID/MaxBody/TrustedProxies；health live/ready；监听器同步报错；超时排空 |
| A12 scripts 三入口闭环 | 达成 | build-platforms.cmd/.ps1、install-linux.sh；release 布局完整 |
| A13 备份恢复与升级失败恢复 | 部分 | 手动/下载备份已验证；恢复演练与回切未验证（docs/deployment.md 已注明） |
| A14 可选能力按需 | 达成 | 未引入队列/分布式缓存/通用事务框架；shell/service/docker/kube 可配置关闭 |
| A15 样板与文档、生成边界 | 达成 | 本文档集；无代码生成器（项目未引入生成） |

## 测试与验证命令

```bash
gofmt -l internal tests web cmd        # 应无输出
go vet ./...
go test ./...
cd web && npm run build                 # 前端构建
```

集成测试（tests/）覆盖：登录与监控检测流、分页与错误外壳、权限拦截（只读角色）、通知打码与测试发送、连续失败邮件节奏、主机与脚本记录、进程监控创建、定时任务 CRUD 与执行记录查询、终端票据与 WebSocket、服务/Docker/Kubernetes 适配与错误映射。
