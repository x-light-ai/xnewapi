# Fork 缩面模式

## 目录

1. 通用判断
2. Translator 接入
3. 渠道选择与 retry
4. 生命周期与迁移
5. 前端 shell
6. 发布覆盖与通用修复

## 1. 通用判断

按下列问题依次判断：

1. 上游是否已有等价能力？有则采用上游并删除重复 fork 实现。
2. 能否只新增所属层的自有文件？优先新增文件。
3. 能否从 route、main、factory、policy 或 registry 等稳定边界显式组装？只保留一个接入点。
4. 是否正在删除或内联重写上游代码，只为增加少量 fork 配置？恢复上游实现，改用末尾追加或 wrapper。
5. 通用正确性修复是否适合贡献上游？适合时记录为临时兼容补丁，并在上游吸收后删除。

关注语义差异块数量，不只关注文件数量。理想的 B 类文件只包含一个 import 和一个相邻调用或注册块。

## 2. Translator 接入

把 CPA 衍生实现保留在独立 translator 文件或中立 translator 包中。不要让 Ali、Gemini、Ollama 等 adaptor 直接依赖 OpenAI channel 的内部实现。

优先通过原有 `service/convert.go` 门面或明确的 registry 保持上游调用点不变。注册必须满足：

- 使用显式 `RegisterClaudeTranslator(...)` 或 bootstrap 调用，不依赖隐藏的跨包 `init()`。
- registry 内部状态不导出为可随意改写的全局函数变量，并防止重复注册。
- bootstrap 位于能够同时依赖 service 和具体 translator、且不会形成 import cycle 的组装层。
- 测试显式安装 fork translator，不假设 main package 已加载。
- 请求、流响应和非流响应分别核对签名；fork translator 返回的错误不得被吞掉或静默回退。
- `OriginalRequestRawJSON`、tool name map 和 stream state 优先收敛为一个明确的 translator state，不持续向共享 `RelayInfo` 增加松散字段。

如果上游函数签名无法表达 fork 的错误语义，允许在少数响应边界保留带标记 hook，不为追求零 diff 降低正确性。

## 3. 渠道选择与 retry

把 SuccessRateSelector、熔断、探索和候选耗尽规则放在 `service` 的 fork 自有策略文件中。

推荐形态：

```go
// FORK-CUSTOM: success-rate selector controls candidate exhaustion.
for service.ShouldContinueRelayRetry(retryParam); retryParam.IncreaseRetry() {
    // Keep the upstream loop body intact.
}
```

结果观察调用可以作为成功和失败路径上的追加 hook，但必须靠近最终结果确认点。不要在 controller 中展开 selector 配置、熔断阈值或候选遍历算法。

保持指定渠道不重试、默认 `RetryTimes` 行为以及 selector 启用后的候选耗尽语义。用定向测试覆盖启用和禁用两种模式。

## 4. 生命周期与迁移

把后台任务启动收敛为一次调用，例如 `service.StartForkServices()`。自有实现仍留在其所属 service/model 文件中。

### 4.1 fork 建表不进上游模型列表

**fork 模型不得写入上游 `DB.AutoMigrate(...)` 的模型列表。** 该列表是上游追加模型的高频改动区，在其中插入 fork 项等于长期占用冲突位。

`model/main.go` 只允许一处 fork 痕迹：`migrateDB()` 与 `migrateDBFast()` 各插入一行 `migrateForkTables()` 调用。

```go
// FORK-CUSTOM: Migrate fork-owned tables outside the upstream AutoMigrate list.
if err := migrateForkTables(); err != nil {
    return err
}
```

分层落点：

- `model/fork_migration.go` —— `migrateForkTables()` 统一入口，只按功能依次调用，不含建表细节。
- 每个功能一个自有 migration 文件持有自己的迁移函数（如 `migrateChannelMonitorTables()`、`migrateUpstreamProviderTables()`），同一功能的多张表在函数内部一起注册，包括其 SQLite 与 server 分支差异。

**验收标准是纯新增。** `model/main.go` 相对上游必须只有插入行、没有修改行：

```bash
diff <(git show <上游ref>:model/main.go) model/main.go
```

输出只应出现 `a` 型（追加）差异块。出现 `c` 型（修改）说明还在改上游代码行。

反面案例：曾把 fork 模型并入上游已有调用，写成 `DB.AutoMigrate(&ChannelCircuitEvent{}, &SubscriptionPlan{})`。这把一处纯新增变成了修改行，上游一旦调整 `SubscriptionPlan` 即语义冲突。正确做法是还原该行，fork 表在自有函数里单独注册。

**顺序约束**：`migrateForkTables()` 必须在上游 `AutoMigrate` 之前执行，因为存量 DDL 修复必须先于驱动的任何 `ColumnTypes` 调用（见 4.2）。

`migrateDBFast()` 当前无调用方（死代码），但仍需与 `migrateDB()` 保持同步，避免它被重新启用时行为分叉。

### 4.2 SQLite 手写 DDL 的硬约束

数据库逻辑必须同时兼容 SQLite、MySQL 和 PostgreSQL。优先 GORM；原始 SQL 必须按项目数据库规则分支。

**手写 SQLite DDL 必须与 GORM 生成形态一致**，否则 `glebarez/sqlite` v1.9.0 无法回读，`AutoMigrate` 直接失败。该驱动用正则解析 `sqlite_master` 里存的原始 DDL，两条限制没有容错：

| 约束 | 违反后果 |
|------|---------|
| 索引 DDL 的 `CREATE [UNIQUE] INDEX <name> ON` 头部只能用单个空格分隔 | 名字与 `ON` 之间有换行即 `invalid DDL`，该表的 `AutoMigrate` 整体失败 |
| 建表列名必须反引号包裹 | 未加引号时 `AlterColumn` 的字段查找正则匹配不到，报 `failed to look up field <列名>` |

配套注意点：

- `CREATE INDEX IF NOT EXISTS` 会被 SQLite 在入库前剥掉，因此**它既不会引起问题、也无法修复已损坏的索引**；不要把它当作幂等保护。
- `gorm:"bigint"` 是无效 tag（正确写法 `type:bigint`）。被忽略后模型期望 `integer` 而存量库是 `BIGINT`，凭空触发 `AlterColumn`，进而暴露上表第二条约束。为 `int64` 字段标类型时务必带 `type:`。
- 存量库修复统一复用 `model/fork_sqlite_ddl.go`，不要在各功能文件里重复实现：
  - `normalizeSQLiteIndexHeaders(table)` 只重写索引头部，保持 `WHERE` 谓词字节不变；
  - `repairLegacySQLiteTableDDL(model)` 对未加引号的存量表重建为规范形态并搬运数据。
- 重建表搬运数据时，新 schema 中 `NOT NULL` 且无默认值、而旧表没有的列必须按类型补零值，否则插入撞约束。空表不会暴露这个问题，必须用带数据的用例覆盖。

改迁移后的最小验证：对真实库副本连续跑两次迁移，确认第二次是 no-op、行数不变、无残留备份表。

把监控 API 路由放入 `router/fork_routes.go`，在 `api-router.go` 的稳定位置调用一次 `registerForkRoutes(...)`。认证中间件必须与原路由保持一致。

## 5. 前端 shell

把 fork 页面、路由描述、菜单项、默认配置扩展和渠道设置 helper 放在 `web/src/extensions/xnewapi/` 或职责明确的现有自有目录。

- `App.jsx`：保留一次 fork routes 组件或 route 列表接入。
- `SiderBar.jsx`：在上游菜单块末尾展开 fork 菜单项，不复制菜单生成逻辑。
- `useSidebar.js`：保留上游 `mergeAdminConfig`，只追加 fork 默认 key；不得为两个 key 删除并内联重写上游 merge。
- `EditChannelModal.jsx`：把 `upstream_protocol` 的默认值、解析、序列化和字段 UI 抽到 helper/component，Modal 只保留少量调用。
- 完全位于 `web/src/features/xnewapi/` 的 fork 自有 UI 可直接写中文可见文案，无需 i18n；不要把 fork 专属 key 写入上游 locale。共享或上游组件中的文案仍使用完整 i18n 流程。

不要手工修改 `bun.lock`。依赖变化后用 Bun 生成并验证。

## 6. 发布覆盖与通用修复

fork 发布 workflow 使用独立新增文件，避免改写上游 workflow。保留项目规则保护的名称、归属、模块路径和许可证。

以下类型优先贡献上游或确认上游是否已经修复：

- 请求 DTO 为保留显式 `0`/`false` 而进行的指针化；
- 通用 JSON editor 能力；
- Vite/Semi 跨平台路径兼容；
- Sidebar 配置深合并 bug。

在上游接受前，将其标为临时兼容修正并保留定向测试；接受后删除 fork 补丁，不长期双重维护。
