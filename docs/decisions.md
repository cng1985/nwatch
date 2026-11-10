# 记录的默认值覆盖与实施决策

本规范（v3.4）允许项目在记录原因、影响与验证结果后覆盖默认值。以下是与《Go 项目结构与开发标准规范》默认值不同的项目决策。未列出的条目按规范默认执行。

## D1：响应外壳迁移与前端同步

- **决策**：所有普通业务 JSON API 改为统一外壳 `{"code":200,"msg":"success","data":...}`，分页原样输出 `{"data":[...],"total","page","page_size"}`；应用错误返回 HTTP 200 + code/msg。
- **原因**：规范第 7、8 节将响应外壳列为稳定基线。
- **影响**：前端 `web/src/api.js` 集中适配外壳与直接分页解析（规范 15 节要求的前端职责）；视图读取 `items/total` 的方式保持不变。
- **验证**：tests/ 集成测试覆盖登录、监控 CRUD、分页、错误码、权限拦截与终端流。

## D2：路由风格保持 REST 兼容

- **决策**：保留既有 REST 风格路由（GET/PUT/DELETE + 路径参数），未改为规范的 POST 动作风格默认值。
- **原因**：规范 3.2/23 节要求存量协议变更有兼容或迁移安排；前端 20+ 视图按路径调用，保持路由可避免一次协议破坏。
- **影响**：Controller 从 URL 提取参数填充 Request/Search 对象；分页参数沿用 `page/pageSize` 查询名。
- **后续**：协议迁移需要版本计划时，按规范 23 节执行。

## D3：监控分组 shared 实体保留 JSON 标签

- **决策**：`modules/monitoring/shared` 的实体（Monitor、AlertEvent 等）同时携带 GORM 映射与 JSON 标签，作为分组内已发布的响应契约；不另建逐字段复制的 Response/Simple 镜像类型。
- **原因**：实体被引擎（scheduler/state/metric/alert/dashboard/maintenance/transfer）直接序列化输出；50+ 字段的三重镜像约 2000 行机械映射，回归风险大于收益。
- **影响**：各 CRUD 模块（monitor/group/notifier/cronjob/access 等）的对外方法仍用类型化 Request/Search/Simple/Response 外壳（别名或包装结构），y=f(x) 与统一 Response/PageResponse 不受影响；shared 仅限 monitoring 分组内使用。
- **验证**：tests/ 集成测试断言响应字段与旧形状兼容。

## D4：SQLite 连接池与 WAL

- **决策**：SQLite 连接池默认 8/4（而非规范默认 1/1），journal_mode 默认 WAL（而非 DELETE）。
- **原因**：既有部署依赖 WAL 与多连接（检测工作池、聚合、采样并发读写）；回退默认会显著降低吞吐。
- **验证**：go test 全量通过；WAL+busy_timeout=5s 在测试与生产镜像中运行。

## D5：全局请求体上限 16MB

- **决策**：HTTP 引擎全局请求体上限为 16MB（规范普通 JSON 默认 2MB）。
- **原因**：监控表格导入（xlsx）单文件上限 8MB，且为 multipart 上传；导入端点另有逐文件 8MB 校验。
- **影响**：普通 JSON 实际上限较规范宽松；后续可按路由拆分限额收紧。

## D6：迁移方式为受控 AutoMigrate

- **决策**：采用 GORM AutoMigrate 执行受控迁移，模型清单集中在 `internal/modules/migrations`，由 `app/bootstrap` 在启动时最先执行。
- **原因**：单实例小工具，无独立迁移文件版本管理的既有需求；规范允许"生产不只依赖自动迁移"作为目标，当前阶段记录为已知差距。
- **影响**：多实例并发启动需要互斥（未实现，单实例部署）；migrations/ 目录暂无 SQL 版本文件。
- **后续**：引入版本化迁移时，在 migrations/ 增加版本记录并保留执行历史。

## D7：认证错误也返回 HTTP 200 + code

- **决策**：401/403 等应用错误一律 HTTP 200 + code（含认证失败）。
- **原因**：规范 7.3 节"成功/应用错误均返回 HTTP 200"，应用码与 HTTP 状态码分开。
- **影响**：前端 api.js 按 payload.code===401 处理登录过期；网络层错误仍可能是真实非 200。
- **验证**：TestRolePermissions、Shell 测试覆盖 401/403 形状。

## D8：兼容环境变量别名

- **决策**：保留 NMONITOR_* 环境变量作为已登记的兼容别名，优先级低于 APP_*。
- **原因**：既有部署（env 文件、systemd 单元）依赖旧名称；规范允许登记弃用别名。
- **影响**：load.go 的 envBindings 显式登记主名与别名；新增配置只允许 APP_*。

## D9：四库支持声明

- **决策**：sqlite 已验证；mysql、postgres、sqlserver 的连接能力已实现（driver + DSN + 连接池），兼容性未验证。
- **原因**：规范要求未经验证只标记计划支持。
- **后续**：按规范 22 节对声明数据库补迁移、约束与关键并发用例后更新支持矩阵。
