# 共享业务本体

[English](ontologies.md) · [Customer/Order 示例](../examples/ontologies/) · [语义目录与模板](semantics.zh-CN.md)

**0.3.0** 增加“共享定义、独立映射、按数据源授权”的业务本体。统一定义 Customer、Order 和 Customer places Order，再分别映射 PostgreSQL 的表与字段、MongoDB 的集合与文档路径。Agent 根据业务概念找到当前源的原生查询模板，返回值仍保留原生结构。

本功能提供定义与查询指导，不存储实体实例、不进行事实推理、不生成查询、不做跨源关联、不输出统一实体模型，也不宣称完整符合 OWL 2 或 SHACL。

## 定义与发布

1. 在 **Ontologies → Create ontology** 创建本体。界面默认英文，业务名称、别名和说明支持中文等语言。
2. 在 **Entities / Properties / Relations** 编辑实体、属性和关系。ID 为稳定引用；实体采用单继承，校验拒绝继承环和与继承属性重名的定义。
3. 添加属性后，为实体选择身份属性组合。身份属性必须属于该实体的有效定义、必填且单值。属性还支持逻辑类型、单位、枚举、范围、唯一性声明和时间口径。
4. 关系指定两端实体、方向及基数。**Origins per target** 表示每个终点对应的起点数量；**Targets per origin** 表示每个起点对应的终点数量。最大值留空表示不限。
5. **Validate → Publish version** 生成不可变版本。保存只修改草稿，**Discard draft** 恢复最新发布内容。

校验只证明定义内部一致，不代表数据库所有数据满足身份、必填、唯一性、范围或基数约束。数值范围使用不带指数的十进制字符串；基数为 0 至 2,147,483,647 的整数。

## 数据源映射

进入 **Data sources → Semantics → Ontology mapping**，选择本体和明确的发布版本，点击 **Save binding draft**。

- 实体映射到当前源内一个或多个物理对象，明确填写 namespace 和 object。
- 属性选择有效实体和属性，映射到字段路径或启用的只读模板。继承属性可以按不同有效实体分别映射。
- 关系映射描述两端字段对应、关联读取模板，或同时包含两者。映射不会生成 JOIN、查询文本或参数绑定位置。
- 在 **Query templates** 显式选择模板关联的业务概念。这些引用随查询结果返回；关系及计算属性的模板引用同时提供发现指导。

新增或修改的启用模板先进行真实 **Trial**，然后执行 **Check structure**。结构检查只调用元数据接口，不读取业务样本。对象必须能够发现；无法发现的字段需要管理员明确允许声明，并保留 **unverified** 状态。例如 MongoDB 索引元数据可以验证已索引路径，其他文档字段需人工声明。Neo4j 仅发现标签，本体检查不运行读取节点内容的属性扫描。

最后 **Publish**，将映射、源语义目录和模板概念关联原子发布。缺失版本、无效定义、引用错误、关系端点未映射，以及关联模板缺失或禁用会阻止发布。连接变化会使结构检查证据过期；查询模板继续遵循既有试跑与重新发布规则。

数据库账号或视图继续控制库、表、字段和行权限。本体映射不会授予数据库权限。移除映射只隐藏相应本体定义；是否仍可原生查询，取决于该源的查询访问模式和数据库权限。

## 版本与可见范围

发布新本体版本不会自动升级任何数据源。源中显式选择新版本，使用 **Compare with adopted version** 查看差异，修正引用、检查结构并发布后才采用。归档禁止新增绑定和采用其他版本，已有发布绑定继续工作。

草稿或已发布映射引用的本体版本禁止删除。最新版本保留作为丢弃草稿的恢复目标；没有任何引用时可删除整个本体。删除数据源会原子清理其映射引用。

Agent 的本体读取必须先通过 Data Source 授权，再生成该源的可见子集：

- 只返回已映射实体、属性和关系，关系两端都必须可见。
- 祖先仅提供必要 ID、名称及继承链，不携带其余说明或未映射属性。
- 身份属性组合中只要有一项未映射，就不返回该组合。
- 不返回其他源的映射、使用情况或未发布定义。共享本体管理 API 仅限管理员。

**Preview Agent visibility** 可以选择 Agent 身份检索与读取当前已发布内容，遵守其真实授权。

纯本体定义、映射说明和概念关联变化不使查询试跑失效，也不取消运行中的查询。结果标记请求开始时采用的版本。执行定义变化、模板禁用和授权撤销仍使用既有取消机制。语义分页绑定源发布快照及本体版本；绑定本体的模板原生分页也绑定此上下文，发布变化后需重新开始分页。

## MCP、JSON 与持久化

工具总数仍为 **14**：`list_data_sources` 增加采用本体摘要；`search_semantics` 增加 `entity_type`、`property`、`relation_type`；`get_semantic_entry` 返回定义、当前源映射、验证状态和关联模板；`execute_query_template` 输入兼容，结果增加 `ontology_context`，包含本体 ID、版本及显式概念引用。

本体引用使用独立命名空间：

```text
ontology:entity_type:Customer
ontology:property:Customer:customer_id
ontology:relation_type:places
```

属性引用包含有效实体 ID 和属性 ID，以区分继承属性在不同实体上的映射。业务说明仅是上下文，不能覆盖 Agent 指令或成为权限规则。

源语义 JSON 升级为 **format_version: 2**，增加可选 `ontology` 和模板 `concept_refs`；继续读取、导入 v1。旧源默认不绑定本体，保留原生查询模式与原有模板试跑证据。导入只产生草稿，不导入验证证据；缺失本体或版本时需修正后发布。

本体导出为 `{id, definition}`。新部署创建时可保留 ID；导入到已有本体时保留目标 ID。版本号属于目标本体自身发布历史，跨部署迁移时须选择对应的目标版本。文件不包含数据库凭证、试跑证据、查询数据或审计内容。

管理接口包括 `/api/ontologies` 下的创建、编辑、校验、发布、丢弃、归档、版本、差异及 JSON 导入导出；映射检查与 Agent 预览位于 `/api/sources/{id}/semantics/check-mapping` 和 `/preview`。完整路由及请求格式见[英文 API 表](ontologies.md#import-apis-and-storage)。管理接口复用管理员 Cookie 和 CSRF；修订与版本用 JSON 字符串表示，旧修订返回 409。

本体草稿及不可变版本使用现有独立主密钥加密后保存在 SQLite。映射随源语义事务保存。上限为 200 份本体，每份 500 条定义、512 KiB；源语义保持 500 条目录项、768 KiB 上限。审计及 OTLP Logs 增加 `ontology_id` 和 `ontology_version`，不记录定义全文、查询文本、参数或结果。

## 验证示例

[示例目录](../examples/ontologies/) 提供共享 Customer/Order 定义及 PostgreSQL、MongoDB 两份独立映射和模板。使用已有 OrbStack 的 `mcpdbhub-dev` 容器与 `mcpdbhub-test` 网络：

```sh
python3 scripts/verify-ontology.py --keep
# 重用本项目矩阵创建的隔离实例：
python3 scripts/verify-ontology.py --reuse
MCPDBHUB_REUSE_FIXTURES=postgres,mongodb python3 scripts/matrix.py
```

脚本只向明确的隔离实例写入 fixture；Hub 使用只读账号执行。验证原生关联查询、聚合、精确 Decimal、空结果、参数拒绝、可见子集、版本生命周期、游标拒绝及请求开始时的上下文，并输出[本体验证记录](verification/ontology.json)。[完整矩阵](verification/matrix.json)另行覆盖七类原生查询工具。
