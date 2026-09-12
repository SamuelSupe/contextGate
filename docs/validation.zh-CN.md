# 验收与复现 / Validation

[English](validation.md)

日期：2026-09-11、2026-09-12。后端与数据库测试运行于 OrbStack Linux arm64；管理界面通过本机 Chrome 实际操作验证，没有引入浏览器自动化框架。

## v0.3.0 共享业务本体 — 2026-09-12

- OrbStack 最终矩阵全部通过：**18 个产品／20 个版本组合**，132 个原生查询/错误用例、89 个拒绝操作用例，以及七类模板原生结果等价与映射发现检查；目标数据未改变。[矩阵记录](verification/matrix.json)。
- 同一 Customer/Order 本体通过 PostgreSQL 表与 MongoDB 集合验证关联/聚合、精确 Decimal、空结果、参数拒绝、可见子集、固定版本和显式采用、并发编辑冲突、引用删除保护、归档、加密重启恢复。执行层还验证请求开始时的本体上下文及发布后旧游标拒绝。[业务与生命周期记录](verification/ontology.json)。
- OrbStack 全部 Go race 检查、TypeScript/Vite 构建及已有无损参数测试通过。定义校验覆盖继承环、属性冲突、身份属性、关系端点、矛盾基数/范围及声明与发现状态。
- 本机 Chrome 实际完成英文界面多语言定义编辑、错误基数修复、两源映射导入、试跑/发布、Agent 授权拒绝与可见定义、原生结果和本体引用。发布本体 v3 后两源保持 v2；PostgreSQL 显式查看差异并采用 v3，MongoDB 仍为 v2，模板执行版本保持 1，无需重新试跑。
- 已下载并检查本体及语义 v2 JSON，完成草稿丢弃、导入错误、方向键标签切换、Escape、390×844 导航与映射表单。修复新增标签造成的横向溢出后，页面宽度为 390 像素；最终浏览器警告和错误日志为空。[UI 记录](verification/ontology-ui.json)。
- 双架构 CI、发行包独立解包和 GitHub 下载回验记录随 [0.3.0 发行版](https://github.com/SamuelSupe/mcpdbhub/releases/tag/v0.3.0) 提供。本机 amd64 发行包使用 OrbStack 模拟，外部数据库矩阵在 Linux arm64 运行。

## v0.2.0 语义目录与模板验证（历史草稿）

- 在 OrbStack 重跑了全部 **18 个产品／20 个版本组合**。原生适配器/MCP 矩阵包含 132 个查询/错误用例和 89 个拒绝操作用例；各产品还比较了真实试跑、发布后的模板与原生查询结果，覆盖无损值、空结果、错误及支持的分页，目标数据未改变。[当前矩阵](verification/matrix.json) 和各产品记录保留实际实现摘要。
- OrbStack `go test -race ./...` 通过，覆盖加密持久化与删除、编辑冲突、草稿隔离、试跑/发布门槛、连接/凭证/版本失效、说明性发布、任务取消、游标隔离、参数注入及禁止位置、OAuth 仅模板限制。
- TypeScript/Vite 构建与两项 Node 无损请求测试通过。发行输入包含七类查询家族示例和中英文语义文档。
- 本机 Chrome 已完成初始化、创建仅模板 SQLite 数据源、多语言概述和字段编辑、结构导入、参数化模板编辑、未试跑拒绝发布、试跑/发布及保留 `9007199254740993` 的预览。[桌面截图](screenshots/semantics.png) 来自该隔离实例。
- 使用 Chrome 创建的 Agent Token 通过真实 HTTP/API 验证了语义发现、服务重启后模板执行、原生查询拒绝、JSON 导入导出往返及丢弃草稿。这些是 API 验证，未替代剩余浏览器操作。[语义验证记录](verification/semantics.json)。
- Mac 在验证中锁屏，Agent 预览、JSON 往返、键盘/窄屏及最终控制台检查仍待完成，没有记为通过。该 0.2.0 发行版保留为草稿。上面的 0.3.0 Chrome 检查验证当前实现，不追溯标记旧构建通过。

以下历史章节对应原版本；当前矩阵文件已更新为 0.2.0，0.1.0 证据保留在[原始标签](https://github.com/SamuelSupe/mcpdbhub/blob/v0.1.0/docs/verification/matrix.json)。

## 本轮真实验收

18 个产品、20 个版本组合全部通过，共 132 个查询/错误场景与 89 个危险操作拒绝场景。网络数据库逐项通过原生适配器和 MCP HTTP 两层检查；SQLite/DuckDB 通过真实文件引擎及 MCP 检查。[机器可读汇总](verification/matrix.json)记录实现摘要和逐产品报告。

本轮复现并修复：Search 忽略 size 及漏报截断、Cypher 数字变字符串、CQL Decimal/浮点/集合参数绑定失败、MongoDB distinct 数组与缺失字段语义错误。分页现在要求每页取完后的内容与完整基线一致；不能只有返回数量断言。

## 已执行的检查

| 范围 | 实际行为与结果 |
|---|---|
| 全部 Go 测试 | `go test ./...` 通过，包含真实 SQLite/DuckDB、官方 MCP HTTP 客户端、实际 stdio 子进程桥接 |
| 竞态 | server、engine、oauth、adapter 的 `go test -race` 通过 |
| 数据库矩阵 | 18 产品、20 个独立版本组合通过；[逐项版本记录](verification/)与[能力限制](support-matrix.zh-CN.md) |
| 只读 | DDL/DML、多语句、写入 CTE、危险函数/脚本/命令被拒绝；执行前后目标数据一致 |
| 每产品 MCP | 每个网络产品通过官方 SDK 的 HTTP 调用完成数据源配置、查询用例、结构发现、拒绝写入、Agent 隔离、审计与撤销；SQLite/DuckDB 另有真实文件 MCP 验收 |
| 身份与授权 | Agent 数据源隔离、元数据拒绝、Token 撤销、授权变更取消运行中和排队任务 |
| OAuth | 授权码 + PKCE S256、错误 verifier、错误 resource、回调不匹配、同意/拒绝、空选择重试、刷新轮换、重放撤销、过期与撤销 |
| 请求和凭证边界 | 未声明连接字段拒绝、CSRF、凭证响应脱敏和加密存储、CIMD 私网/回调限制、HTTPS 信任证书验证、文档 ID 路径注入拒绝、Redis 超大 RESP 长度在驱动分配内存前拒绝 |
| 分页与结果 | MongoDB/CQL/Search 逐页取完与基线比较、Redis 扫描、跨身份/查询/篡改游标拒绝；搜索 size:0 / 小页 / 无排序截断；Neo4j 小数及嵌套整数；CQL Decimal / float / double / frozen list / map；MongoDB distinct 数组与缺失字段 |
| 超时与恢复 | 真实 PostgreSQL 长聚合 1 秒超时、取消后复用连接、停止/重启数据库恢复、停用/重新启用数据源；[记录](verification/recovery.json) |
| 配置持久化 | 重新打开配置数据库后凭证/授权/审计恢复；发布镜像启动、初始化与重启恢复通过，[镜像记录](verification/package.json) |
| 前端构建 | TypeScript + Vite 正式构建通过；Go 内嵌资源，运行镜像无 Node.js |
| Chrome | 初始化页面、登录、空列表、添加/保存/测试数据源、结构预览、参数化查询、写入失败与旧结果清理、搜索空状态、Agent Token 一次显示/撤销、审计、支持清单、设置、OAuth 预注册/同意/拒绝/撤销/过期页 |
| 视觉与键盘 | 对照紧凑版概念图检查侧栏、表格和右侧配置抽屉；390×844 窄屏导航/抽屉；Escape 关闭、焦点恢复、Shift+Tab 循环；Chrome 控制台未发现应用错误 |

权限 evidence 与功能验收分开记录。本轮 Elasticsearch/OpenSearch 开启 HTTPS 和安全插件，专用读取角色直接写入返回 403，未信任证书连接被拒绝；Neo4j 使用 Community。测试证明上述隔离实例的行为，不推断生产账号的全部权限或集群配置。没有进行长期压力、所有类型/操作符组合、集群故障切换、amd64/Windows/macOS 原生发行包或公网 OAuth 客户端互操作验收。

## 搭建 OrbStack 开发容器

在仓库根目录执行（名称均为本项目专用资源）：

```sh
docker network create mcpdbhub-test
docker run -d --name mcpdbhub-dev --network mcpdbhub-test \
  --cpus 4 --memory 6g \
  -v "$PWD:/work" \
  -v mcpdbhub-gomod:/go/pkg/mod \
  -v mcpdbhub-gocache:/root/.cache/go-build \
  -w /work golang:1.26-bookworm sleep infinity
# 若环境代理使用自定义 CA，先在这个开发容器配置受信任 CA。
npm --prefix web ci
npm --prefix web run build
docker exec mcpdbhub-dev go mod download
docker exec mcpdbhub-dev go test ./...
docker exec mcpdbhub-dev go test -race ./internal/server ./internal/engine ./internal/oauth ./internal/adapter
```

`go:embed` 需要先构建前端。Go 镜像提供 CGO 工具链。开发容器中还需要 curl，供测试脚本连接隔离的 HTTP 数据库接口；Bookworm 完整 Go 镜像已包含它。

## 复现所有数据库

```sh
python3 scripts/matrix.py
# 只验收某些产品
python3 scripts/matrix.py postgres mariadb redis
# 默认包含 SQLite/DuckDB；也可仅运行真实文件引擎及 MCP 流程
python3 scripts/matrix.py sqlite duckdb
python3 scripts/publish-verification.py
```

脚本创建带 `com.mcpdbhub.fixture=true` 标签的独立实例，无宿主机数据库端口，逐个初始化、测试并清理自己创建的容器。不清理同名既存容器或其他工作负载。需预留镜像下载空间；第一次依赖和数据库镜像下载可能较慢。

诊断时可设置 `MCPDBHUB_KEEP_FIXTURES=1` 保留实例。下一次运行前自行删除对应测试容器，禁止清理无关环境。失败日志和含专用测试凭证的 manifest 留在被 Git 忽略的 `artifacts/matrix`；**发布脚本只复制无凭证的通过记录**。选择产品重测时先移除旧记录，失败不复用之前的成功证据。

发布前还会校验实现摘要与 MCP 验收标记，代码变更后的旧成功记录不能直接发布。报告中的 checked_at/verified_at 为 UTC。ScyllaDB 版本来自 system.versions，TimescaleDB 同时记录扩展版本与 PostgreSQL 版本，Valkey 不以兼容 Redis 的版本号冒充产品版本。

## 真实服务超时/恢复检查

先启动 Hub，并在隔离环境保留 PostgreSQL fixture：

```sh
MCPDBHUB_KEEP_FIXTURES=1 python3 scripts/matrix.py postgres
MCPDBHUB_TEST_URL=http://127.0.0.1:19840 \
MCPDBHUB_ADMIN_PASSWORD='<当前测试服务的管理员密码>' \
python3 scripts/check-recovery.py
```

脚本先核对 fixture 标签，仅停止/重启 `mcpdbhub-it-postgres`，创建并删除一条专用数据源配置，不操作其他数据源。测试不用于生产数据库。

## Docker 交付

重建镜像后可保留 PostgreSQL fixture，运行 `python3 scripts/check-package.py`，验证最终镜像的非 root、只读根目录、无 Node、初始化、MCP 查询、重启恢复与 Token 撤销。该脚本自行创建并清理专用容器与配置卷。构建依赖通过 BuildKit 缓存复用；仅下载当前目标平台需要的模块。环境使用自定义代理 CA 时通过 `--secret id=build_ca,src=/path/to/trusted-ca.pem` 传入构建信任链。


`Dockerfile` 分前端构建、Go 构建、Debian slim 运行三个阶段；运行 UID/GID 10001。`compose.yaml` 默认回环监听、数据库目录只读挂载、只读根文件系统、去除 capabilities 和 no-new-privileges。配置卷仍可写，以保存管理员配置和审计。

生产部署需要自己的公开 HTTPS 地址、数据库最小权限账号和 CA 配置。具体命令见中英文 README。此次镜像仅在 Linux arm64 实际构建和运行；其他平台未标记为已验收。

## 产品流程修复与英文管理界面回归（2026-09-11）

本轮针对授权生命周期、连接状态、凭证切换、无损预览、审计诊断和英文配置流程进行了增量验证；不是再次执行全部 20 个产品/版本案例。

- OrbStack `go test ./...`、`go test -race ./...` 通过。新增行为回归覆盖旧版本撤销数据迁移、暂停/恢复、Token 轮换与旧身份失效、永久撤销、认证方式切换与清除、连接失败/恢复、请求 ID 审计关联、Agent 预览隔离及错误脱敏。
- `node --test web/test/query.test.mjs` 通过，验证首次请求和游标续页都保留整数/Decimal 参数的原始 JSON 数值文本；英文前端 TypeScript/Vite 构建通过。
- 实际数据库与 MCP 回归：PostgreSQL 17.11、MongoDB 7.0.39、Redis 7.4.6、Elasticsearch 8.19.17、OpenSearch 3.2.0、SQLite 3.53.4、DuckDB 1.5.5 通过。搜索数据库重新验证 TLS 信任边界及数据库账号写入拒绝，断言使用结构化错误码 `database_permission` / `HTTP 403`。
- 本机 Chrome：错误密码保存后保留配置页和明确诊断；改正密码后恢复且不重复创建；已成功的数据源停机后检查转为失败，重启后恢复；`9007199254740993` 和 `0.1234567890123456789012345` 经 UI 提交后原样返回；结构导航、表格结果、MongoDB 原生翻页、取消及后续查询通过。
- Chrome 创建的 Token 经实际 MCP 调用成功；暂停后 401、恢复后成功；轮换后旧 Token 401、新 Token 成功；永久撤销后 401 且编辑授权不能重新启用；Connect 页可再次打开并展示实际客户端调用状态。无授权 Agent 的结构预览被拒绝。
- 审计按请求 ID 筛选并展示原生错误码、诊断建议，空状态正常。390×844 配置页无横向溢出；InfluxDB 3 默认端口 8181；Escape 关闭并恢复入口焦点；本轮浏览器日志未发现应用错误。
- 最终 Docker 镜像由 `scripts/check-package.py` 验证非 root、只读根目录、无 Node 运行时、初始化、MCP 查询、配置/会话重启恢复和撤销；结果见 [package.json](verification/package.json)。正在运行的本地服务已替换为该镜像中的最终二进制。

测试数据源、临时数据库容器及测试 Token 文件已清理；撤销后的测试 Agent 与审计记录保留。没有修改原有数据源内容。

## 第二轮产品缺陷修复回归（2026-09-11）

本轮修复六项问题：Agent 过期编辑覆盖新授权、删除数据源后残留授权、仅改名称中断查询、InfluxDB 版本示例与错误说明不匹配、OAuth 客户端缺少生命周期管理，以及管理员忘记密码无法本地恢复。

- OrbStack `go test ./...`、`go test -race ./...` 通过。行为回归覆盖授权版本冲突、删除及历史数据迁移、改名期间的运行查询和游标、按版本发现 InfluxDB 查询能力、OAuth 回调修改/停用/密钥轮换/删除后的授权码及访问和刷新令牌失效、旧 OAuth Agent 关联、不同客户端隔离，以及密码恢复保留数据源与 Agent 凭证。
- PostgreSQL 17.11、InfluxDB 1.8.10 / 2.7.12 / 3.11.2、SQLite 3.53.4、DuckDB 1.5.5 通过真实引擎和 MCP 回归，共 28 个查询/错误场景、27 个危险操作拒绝场景；执行前后目标数据一致。[本轮独立记录](verification/lifecycle.json)保留实现摘要与原始通过证据。本轮未重新执行全部 20 个版本组合。
- 本机 Chrome 双标签页复现授权竞争：第二页删除授权后，第一页的旧编辑被拒绝，重新加载后显示当前授权。历史缺失数据源的授权已迁移清理，Agent 编辑可以正常保存；原有撤销状态保留。
- Chrome 验证 OAuth 客户端注册、一次显示凭证、错误回调保留输入并显示诊断、修改回调、停用、轮换、重新启用、搜索空状态和删除。390×844 下操作按钮可见、无重叠；桌面和窄屏均经过截图检查。
- Chrome 添加真实 InfluxDB 2 数据源后，默认示例使用配置的 bucket 和 Flux，直接返回 6 条时序记录。提交 InfluxQL 时提示 `InfluxDB 2.x requires language flux`，不再误导为数据库权限问题。设置页提供英文恢复说明；浏览器日志未发现应用错误。
- 前端 TypeScript/Vite 构建及现有无损参数测试通过。最终 Linux arm64 镜像通过 `scripts/check-package.py`，额外验证停止服务后通过 stdin 恢复密码、旧密码和管理会话失效、新密码登录、原有数据库凭证和 Agent Token 继续查询；[镜像验证记录](verification/package.json)包含镜像和源码摘要。

当前本地服务运行已验证镜像中的二进制。仅清理本轮临时 OAuth 客户端、数据源和数据库容器；保留原有配置、撤销状态及审计记录。密码恢复使用隔离配置卷验收，没有更改本地服务的管理员密码。[中文恢复说明](../README.zh-CN.md)与[英文恢复说明](../README.md)已更新。

## 第三轮查询与授权缺陷修复回归（2026-09-11）

本轮修复五项问题：Agent 原样保存意外延后到期时间、单个 Agent 的排队请求占满全局并发、正常 SQL 函数被关键字误拒绝、SQL 结构发现静默丢失第 1001 项，以及空授权值导致 Agent 页面崩溃。

- OrbStack `go test ./...`、`go test -race ./...` 通过。回归覆盖 Agent/数据源/全局三个并发上限、排队隔离及取消后容量释放、跨操作游标拒绝、1001 张表完整发现、2 KiB 响应限制下的续页、管理 API 分页，以及历史 `null` 授权的读取和编辑。
- PostgreSQL、MySQL、MariaDB、TiDB、CockroachDB、TimescaleDB、ClickHouse、SQLite、DuckDB、InfluxDB 3 共 10 个实际产品/版本组合通过：60 个查询/错误场景、59 个危险操作拒绝场景，前后数据一致。九个 SQL 数据源均逐页比较命名空间、表和字段列表与完整基线。此次为受影响范围的增量回归，未重新执行完整 20 版本矩阵；[独立记录](verification/query-access.json)保留每项版本与实现摘要。
- MariaDB 首次运行暴露测试初始化竞争：socket 查询连到了即将关闭的临时服务。脚本现通过 TCP 等待正式服务，修复后重新建库、适配器和 MCP 验收通过。
- 最终镜像的隔离 HTTP 服务中，Agent A 提交 32 个受控慢请求时，Agent B 的独立 SQLite 查询耗时 2 ms；1001 张真实 SQLite 表通过 MCP 分为 1000 + 1 两页，最后一项为 `t_1000`。慢请求使用专用 HTTP 测试服务；该检查验证排队隔离，不作为 Elasticsearch 性能指标。
- 本机 Chrome 原样保存到期时间 `2026-09-12T09:15:00.123456Z` 后值完全相同，键盘将本地时间改为 `17:16:28` 后保存为 `09:16:28Z`。省略或显式 `null` 授权的 Agent 列表与编辑正常；390 像素窄屏、Escape 关闭与入口焦点恢复通过。
- Chrome 实际点击结构 **Next page** 找到第 1001 张表，并成功发现其字段。`replace()` / `format()` 预览返回 `xbc` / `ok`；`DROP TABLE` 被拒绝，随后确认 1001 张表未变。页面日志未发现应用错误。
- TypeScript/Vite 构建和现有无损参数测试通过。最终 Linux arm64 镜像通过初始化、HTTP MCP、重启恢复、Token 撤销及密码恢复验收，[镜像记录](verification/package.json)与运行二进制摘要均已记录。本地 19840 服务已更新，原管理会话与配置仍可使用。

本轮使用独立配置卷及数据库文件进行修改验证，没有更改原服务的管理员密码、数据源或 Agent 授权。临时容器、配置卷和 Token 文件已清理。新增操作绑定会使升级前签发的短期游标失效，客户端重新执行首个请求即可。SQL 结构分页不承诺数据库结构变动时的跨页快照一致性。

## v0.1.0 发行前检查（2026-09-11）

- 统一版本标识与公开 Go 模块路径后，在 OrbStack 重跑 18 产品、20 个版本组合：132 个查询/错误场景、89 个危险操作拒绝场景全部通过。源码摘要及逐产品原始证据已更新至 [matrix.json](verification/matrix.json)。
- Go 全量测试与竞态检查通过；包含真实 SQLite/DuckDB、HTTP MCP、stdio 桥接、OAuth、授权撤销和分页回归。PostgreSQL/TimescaleDB 的测试初始化也改为等待正式 TCP 服务，避免临时启动服务尚未安装扩展时抢先建表。
- README 截图来自本机 Chrome 实际运行页面。使用独立配置卷、真实 PostgreSQL / SQLite / DuckDB 和示例数据，验证 3 个 Agent 的 MCP 查询以及授权下的收入聚合；截图未包含数据库或 Agent 凭证。
- Linux 发行包构建方式、运行库要求、独立解包验收及发布步骤见 [发行流程](releasing.md)。每个最终发行包的下载摘要和验收记录随 GitHub Release 提供，历史修复记录保留其各自源码摘要。

## OTLP 审计上报 — 2026-09-11（v0.1.0 之后）

OrbStack 中 `go test -race ./...` 通过，覆盖实际 HTTP/TLS/gRPC 接收端、SQLite 进度恢复、认证与脱敏、拒收与重试、发送取消和管理接口权限。独立 Hub 与官方 Collector 0.160.0 完成 HTTP/protobuf、gRPC 联调，真实查询成功及写入拒绝均生成审计日志。停 Collector 后查询仍约 4 ms 完成；积压 3 条日志跨 Hub 重启保留，恢复后补发，最终接收连续 ID 1–7、待发送为 0、实例 ID 不变。这是小型隔离用例的隔离性验证，不是性能基准。

本机 Chrome 验证英文配置页的保存、测试、错误提示、凭证清除、刷新状态、键盘操作和 390×844 窄屏，无横向溢出及应用控制台错误。TypeScript/Vite 构建通过。本轮未修改数据库适配器，未重跑全部 20 个版本的兼容矩阵，也未发布新发行包。详见[英文验证记录](validation.md)及[发送语义](audit-export.zh-CN.md)。

## v0.1.1 发行范围

v0.1.1 增加 OTLP 审计上报，发行专属的原生 CI 及实际解压包验证见 [VALIDATION.json](https://github.com/SamuelSupe/mcpdbhub/releases/download/v0.1.1/VALIDATION.json)。本轮重新验证 PostgreSQL 适配器和 MCP，并验证 Collector 上报、故障恢复与英文界面。全部 18 产品/20 版本的矩阵仍是 v0.1.0 的历史证据，本次未重跑全矩阵。
