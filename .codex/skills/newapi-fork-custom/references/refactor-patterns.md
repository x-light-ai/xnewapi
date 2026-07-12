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

把 SQLite 建表、补列和索引逻辑移出 `model/main.go`，放入 fork 自有 migration 文件。`model/main.go` 只保留：

- 模型列表末尾的注册项；
- 一次 fork migration 调用；
- fast migration 必需的对应注册。

数据库逻辑必须同时兼容 SQLite、MySQL 和 PostgreSQL。优先 GORM；原始 SQL 必须按项目数据库规则分支。

把监控 API 路由放入 `router/fork_routes.go`，在 `api-router.go` 的稳定位置调用一次 `registerForkRoutes(...)`。认证中间件必须与原路由保持一致。

## 5. 前端 shell

把 fork 页面、路由描述、菜单项、默认配置扩展和渠道设置 helper 放在 `web/src/extensions/xnewapi/` 或职责明确的现有自有目录。

- `App.jsx`：保留一次 fork routes 组件或 route 列表接入。
- `SiderBar.jsx`：在上游菜单块末尾展开 fork 菜单项，不复制菜单生成逻辑。
- `useSidebar.js`：保留上游 `mergeAdminConfig`，只追加 fork 默认 key；不得为两个 key 删除并内联重写上游 merge。
- `EditChannelModal.jsx`：把 `upstream_protocol` 的默认值、解析、序列化和字段 UI 抽到 helper/component，Modal 只保留少量调用。
- fork 专属文案仍使用项目 i18n 流程，不写死可见文案。

不要手工修改 `bun.lock`。依赖变化后用 Bun 生成并验证。

## 6. 发布覆盖与通用修复

fork 发布 workflow 使用独立新增文件，避免改写上游 workflow。保留项目规则保护的名称、归属、模块路径和许可证。

以下类型优先贡献上游或确认上游是否已经修复：

- 请求 DTO 为保留显式 `0`/`false` 而进行的指针化；
- 通用 JSON editor 能力；
- Vite/Semi 跨平台路径兼容；
- Sidebar 配置深合并 bug。

在上游接受前，将其标为临时兼容修正并保留定向测试；接受后删除 fork 补丁，不长期双重维护。

