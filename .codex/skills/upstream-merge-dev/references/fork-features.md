# newapi Fork 功能点清单

记录 newapi(`x-light-ai/xnewapi`)相对官方上游 `QuantumNous/new-api` 的所有 fork 自定义功能,用于从上游 merge 时判断**语义冲突**、保护 fork 功能不被覆盖。合并前必读,合并后据此逐项验证。

> **本清单仅作导航,不作权威依据。** 合并前**必须**以真实 diff 为准核对:
> ```bash
> git fetch upstream
> BASE=$(git merge-base dev <目标tag>)
> git diff $BASE..dev --name-status | sort   # A=fork新增(不冲突) M=fork改上游(会冲突)
> ```
> 判断某文件是否 fork 自定义的黄金标准:出现在 `git diff $BASE..dev` 里才算;文件"存在"不代表是 fork 改的(可能是上游功能)。核实归属用 `git log --diff-filter=A -- <file>` 看引入者,官方作者(如 `i@caion.me`、`seefs001`)引入且 dev 无改动的即上游功能。
>
> 本清单需随 fork 演进维护:新增 fork 功能时补充,重构时更新路径。上次全量核对:2026-07-12(BASE=`f2f3410`;已执行 fork 缩面并补齐 `FORK-CUSTOM` 标记)。

## 风险等级说明

- **高**:核心自研业务逻辑,上游修改相关文件时极易语义冲突,必须逐文件审查并保护。
- **中**:与上游有集成点(渠道选择、relay 主流程、渠道设置 DTO),需审查接口兼容性。
- **低**:独立新增文件或配置项,冲突概率低,但仍需确认未被上游覆盖。

## 文件分类总则

- **A 类(fork 新增文件)**:上游没有的文件,merge 时 git 原样保留,**不会冲突**。确认未被误删即可。
- **B 类(fork 修改的上游文件)**:双方都可能改,**冲突真正发生处**,合并时必须逐一审查。这是清单的重点。

## 功能概览(真正的 fork 自定义,共 8 项)

| # | 功能 | A类新增文件 | B类改上游文件 | 风险 |
|---|------|-----------|-------------|------|
| 1 | OpenAI↔Claude 转换器(cpaopenai) | `relay/channel/openai/claude_*.go`、`service/fork_claude_translator.go`、`forkcustom/translator.go` | 各 request/response 边界的一行 service facade hook、`relay/common/relay_info.go`、`dto/openai_request.go` | 高 |
| 2 | Codex↔Claude 转换器(codex2claude) | `relay/channel/codex/claude_*.go` | `relay/channel/codex/adaptor.go` | 高 |
| 3 | 第三方 Codex 供应商支持 | `relay/fork_claude_upstream.go`、`relay/channel/codex/fork_upstream_policy.go`、classic/default 的 `features/xnewapi` 渠道扩展 | `dto/channel_settings.go`、`relay/claude_handler.go`、两套前端渠道表单的单点 hook | 中 |
| 4 | 渠道成功率选择器(SuccessRateSelector) | `service/channel_success_rate*.go`、`service/fork_retry_policy.go`、classic/default 的 SuccessRateSelector 设置组件 | `service/channel_select.go`、`controller/relay.go` 的单点 policy/observe hook、default 模型设置 section registry | 中 |
| 5 | 渠道监控系统 | 原有监控文件 + `forkcustom/bootstrap.go`、`model/channel_circuit_event_migration.go`、`router/fork_routes.go`、classic/default 的 `features/xnewapi/` | `main.go`、`model/main.go`、`router/api-router.go`、两套前端 shell 的单点扩展 | 中 |
| 6 | 渠道成功率高级配置 | `setting/operation_setting/channel_success_rate_setting.go` | — | 低 |
| 7 | 渠道权重管理 | `service/channel_score_override.go` | (复用 #5 的 controller/router) | 低 |
| 8 | 渠道测试与自动禁用 | — | `controller/channel-test.go` | 低 |

---

## 高风险功能(必须逐文件审查保护)

### 1. OpenAI↔Claude 转换器(cpaopenai)
Claude 请求→OpenAI、OpenAI 响应→Claude SSE 的协议转换(Claude 客户端对接 OpenAI Chat 上游)。
- **A类核心**:`relay/channel/openai/claude_request_translator.go`、`claude_response_translator.go`、`claude_translator_utils.go`、`claude_sse_bytes.go`;`service/fork_claude_translator.go` 提供显式 registry;`forkcustom/translator.go` 在应用组装边界注册 CPA 实现。
- **B类接入点**(合并重点审查):请求和响应边界统一调用 `service.TranslateClaudeRequest` / `OpenAIResponseToClaude` / `OpenAIStreamResponseToClaude`;`relay/common/relay_info.go` 只增加一个 `ForkTranslator` 状态对象;`dto/openai_request.go` 保留 StreamOptions 指针化(Rule 6)。不得恢复跨 channel 直接调用或隐藏 `init()` 注册。
- **上游来源特殊**:此转换器同步自参考仓库 **CLIProxyAPI**(非官方 new-api),用 [[cliproxyapi-translator-sync]] 技能维护,不随官方 merge 更新。官方 merge 时只需保证这些文件不被误删/误改。

### 2. Codex↔Claude 转换器(codex2claude)
Claude 请求→Codex Responses、Codex 响应→Claude SSE 的协议转换,含 web_search、defer function call、pending resolve 等。
- **A类核心**:`relay/channel/codex/claude_request_translator.go`、`claude_response_translator.go`、`claude_response_web_search.go`、`claude_helper.go`、`claude_translator_utils.go`。
- **B类接入点**:`relay/channel/codex/adaptor.go` 只保留 URL/header policy hook;实现位于 `fork_upstream_policy.go`。
- **上游来源同 #1**:同步自 CLIProxyAPI,用 cliproxyapi-translator-sync 技能维护。

---

## 中风险功能(审查接口兼容性)

### 3. 第三方 Codex 供应商支持
Anthropic Claude 渠道新增 `upstream_protocol` 字段(`anthropic`/`codex`),按 key 形态自动分流 URL 与鉴权,使主渠道可指向第三方兼容 Codex 上游。
- **A类实现**:`relay/fork_claude_upstream.go`、`relay/channel/codex/fork_upstream_policy.go`、`web/classic/src/extensions/xnewapi/channelSettings.jsx`、`web/default/src/features/xnewapi/channel-upstream-protocol-field.tsx`。
- **B类接入点**:`dto/channel_settings.go`、`relay/claude_handler.go` 的单点 dispatch hook、classic `EditChannelModal.jsx` 的 extension 调用，以及 default `channel-form.ts` / `channel-mutate-drawer.tsx` 的窄字段 hook。
- 合并注意:上游改 `claude_handler.go` 分支或渠道设置 DTO 时验证分流逻辑。

### 4. 渠道成功率选择器(SuccessRateSelector)
基于真实请求结果的运行时渠道择优:Laplace 平滑、半衰期衰减、连续失败惩罚、探索机制、临时熔断/半开、跨优先级切换。
- **A类核心**:`service/channel_success_rate.go`(+ `channel_success_rate_*_test.go` 多个)、classic `SettingsSuccessRateSelector.jsx`、default `features/xnewapi/success-rate-settings-section.tsx`。
- **B类接入点**:`service/channel_select.go` 的两处 selection hook;`controller/relay.go` 通过 `service/fork_retry_policy.go` 的单一循环条件和结果 observe hook 接入，不在 controller 展开 fork 策略。
- 合并注意:上游重构 `channel_select.go` 或重试主流程时逐文件审查。

### 5. 渠道监控系统
渠道健康度监控:成功率/失败数/延迟汇总、可用性趋势、延迟/稳定性排名、临时熔断事件记录,前端独立页面。
- **A类核心**:`controller/channel_monitor.go`(+`_test.go`)、`model/channel_monitor.go`、`model/channel_monitor_db.go`(+`_test.go`)、`model/channel_circuit_event.go`、`web/classic/src/pages/ChannelMonitor/`、`web/default/src/features/xnewapi/channel-monitor.tsx`。
- **A类组装**:`forkcustom/bootstrap.go`、`model/channel_circuit_event_migration.go`、`router/fork_routes.go`、classic `extensions/xnewapi/`、default `features/xnewapi/` 与 `routes/_authenticated/channel-monitor/`。
- **B类接入点**:`main.go` 仅调用 `forkcustom.Start()`;`model/main.go` 仅保留模型注册和迁移调用;`router/api-router.go` 仅调用 `registerForkRoutes`;classic App/Sidebar 与 default sidebar/section registry 只展开 fork extension。
- 合并注意:DB 聚合查询需保持三库兼容(Rule 2);上游改路由注册或 `model/main.go` 迁移时确认监控注册未被覆盖。

---

## 低风险功能(独立新增,确认未被覆盖即可)

| # | 功能 | 说明与关键文件 |
|---|------|--------------|
| 6 | 渠道成功率高级配置 | SuccessRateSelector 细粒度参数(半衰期、探索率、连续失败阈值、熔断/恢复)。`setting/operation_setting/channel_success_rate_setting.go` |
| 7 | 渠道权重管理 | 后台手动设渠道初始权重/优先级,影响择优。`service/channel_score_override.go`,复用 #5 的 controller/router |
| 8 | 渠道测试与自动禁用 | 手动/批量渠道测试 + 失败触发临时熔断。`controller/channel-test.go` + SuccessRateSelector 熔断逻辑 |

---

## ⚠️ 易误认为 fork、实为上游的功能(dev 未改动,合并随上游自动合并即可)

以下功能的文件由**官方作者引入**(存在于 upstream 多个分支),dev **零改动**,**不是 fork 自定义**。合并时**不要**对它们做保护性手动审查,直接跟随上游即可。曾被旧清单误标为 fork 功能,特此记录避免重蹈覆辙。

| 功能 | 关键文件 | 引入者 |
|------|---------|--------|
| 分层计费系统(表达式计费) | `pkg/billingexpr/`、`service/tiered_settle.go`、`setting/billing_setting/tiered_billing.go`、`web/.../TieredPricingEditor.jsx` | CaIon(官方) |
| 工具调用与文本计费 | `service/tool_billing.go`、`service/text_quota.go` | 官方 |
| 渠道亲和性(Channel Affinity) | `service/channel_affinity.go`、`setting/operation_setting/channel_affinity_setting.go` | seefs001(官方) |
| 渠道监控设置(运行参数) | `setting/operation_setting/monitor_setting.go` | CaIon(官方) |
| Codex 凭证刷新任务 | `service/codex_credential_refresh.go`、`codex_credential_refresh_task.go` | seefs001(官方) |
| 性能监控 | `controller/performance.go`、`common/system_monitor{,_unix,_windows}.go` | CaIon(官方) |

> 注:项目 CLAUDE.md **Rule 7**(改分层计费前读 `pkg/billingexpr/expr.md`)描述的是**上游功能**的规范 —— 读文档仍然正确,但它不代表 billingexpr 是 fork 自研。

## 通用修复反哺状态

- StreamOptions 指针化:上游补丁见 `docs/upstream-pr-stream-options-pointer.patch`,目标基线 `upstream/main@4e570389`,含显式 `false` 测试。
- JSONEditor key description:上游补丁见 `docs/upstream-pr-jsoneditor-key-descriptions.patch`,已适配当前上游 `web/classic/` 路径。
- Semi Windows path:当前上游已迁移到 Rsbuild并显式解析 Semi 路径,无需提交旧 Vite 补丁;结论见 `docs/upstream-review-semi-windows-path.md`。fork 合并 Rsbuild 前继续保留本地 wrapper。

---

## 合并策略速查

- **高风险 #1-#2(转换器)**:逐文件审查(workflow.md 第 9 节)。上游是 CLIProxyAPI,官方 merge 时只保证 A 类文件不被误删/误改;其功能更新走 cliproxyapi-translator-sync 技能。**重点看 B 类接入点**(各渠道 adaptor、`relay_info.go`、`compatible_handler.go`)是否被上游 relay 主流程改动波及。
- **中风险 #3-#5**:Git 自动合并后必做语义冲突审查(workflow.md 第 6.5 节),重点 B 类接入点:`channel_select.go`/`controller/relay.go`(#4)、`claude_handler.go`/`channel_settings.go`(#3)、`router/api-router.go`/`main.go`/`model/main.go`(#5)。
- **低风险 #6-#8**:确认独立文件未被覆盖,通常 Git 自动合并即可。
- **上游功能区(⚠️ 节)**:不做任何保护,直接跟随上游。

## 清单核对方式

```bash
git fetch upstream
BASE=$(git merge-base dev <目标tag>)
# dev 相对共同祖先的自定义改动, A=新增 M=改上游, 对照上表核实完整性
git diff $BASE..dev --name-status | sort
# 双方都改的文件(最易冲突, 重点审查)
comm -12 <(git diff $BASE..dev --name-only | sort -u) <(git diff $BASE..<目标tag> --name-only | sort -u)
# 核实某文件归属(fork 还是上游): 看引入者 + 是否在 upstream 分支
git log --oneline --diff-filter=A -- <file> | tail -1
git branch -r --contains <引入commit> | grep upstream
```
